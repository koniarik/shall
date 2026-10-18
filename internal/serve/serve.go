// Package serve shows the sets under a directory in a browser and offers
// them to scripts and agents, following their files as they change. The
// server answers with JSON, every decision about a set made; the page is one
// static file whose script builds the document from those answers.
package serve

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"shall/internal/check"
	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/tree"
)

// DefaultAddr is where the server listens when told nowhere else: on this
// machine only.
const DefaultAddr = "127.0.0.1:7180"

// pollEvery is how often the server looks for changed files.
const pollEvery = time.Second

var (
	//go:embed web/index.html
	indexPage []byte
	//go:embed web/static
	staticFiles embed.FS
)

// Server serves the sets under one directory.
type Server struct {
	root  string
	local bool
	// boot tells this run of the server from others, so that a page left open
	// across a restart sees a new version.
	boot    string
	changes int

	snap   atomic.Pointer[snapshot]
	stamps map[string]stamp

	mu   sync.Mutex
	subs map[chan string]struct{}
}

// New reads the sets under root and returns a server for them. A local
// server answers only requests that address this machine by a loopback name.
func New(root string, local bool) *Server {
	s := &Server{
		root: root, local: local,
		boot: strconv.FormatInt(time.Now().UnixNano(), 36),
		subs: map[chan string]struct{}{},
	}
	s.snap.Store(load(root, s.version()))
	s.scan(s.dirs())
	return s
}

// Run serves the sets under root on ln until ctx is done. The server is
// local when ln listens on a loopback address.
func Run(ctx context.Context, root string, ln net.Listener) error {
	tcp, ok := ln.Addr().(*net.TCPAddr)
	return New(root, ok && tcp.IP.IsLoopback()).Serve(ctx, ln)
}

// Serve serves on ln, following the files, until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go s.watch(ctx)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		srv.Close()
		<-errc
		return nil
	case err := <-errc:
		return err
	}
}

// Handler returns the server's routes: the page at every address a reader
// can open, its static files, and the API.
func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /{set}/{$}", s.page)
	mux.HandleFunc("GET /{set}/{id}", s.page)
	mux.Handle("GET /-/static/", http.StripPrefix("/-/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /api/sets", s.apiSets)
	mux.HandleFunc("GET /api/sets/{set}", s.apiSet)
	mux.HandleFunc("GET /api/sets/{set}/files/{path...}", s.apiFile)
	mux.HandleFunc("GET /api/sets/{set}/items/{id}", s.apiItem)
	mux.HandleFunc("GET /api/sets/{set}/resolve", s.apiResolve)
	mux.HandleFunc("GET /api/events", s.events)
	return s.guard(mux)
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.local && !loopbackHost(r.Host) {
			http.Error(w, "this server answers only to localhost", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// loopbackHost tells whether a Host header names this machine by localhost
// or a loopback address, with or without a port.
func loopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) current() *snapshot { return s.snap.Load() }

// version returns the version of the files as of the next load.
func (s *Server) version() string {
	s.changes++
	return s.boot + "." + strconv.Itoa(s.changes)
}

// page answers every address a reader can open with the one page, whose
// script reads the address.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexPage)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// problem answers that a request names nothing the server has.
func problem(w http.ResponseWriter, a errorAnswer) {
	writeJSON(w, http.StatusNotFound, a)
}

// set returns the set a request names, or answers that there is none.
func (s *Server) set(w http.ResponseWriter, r *http.Request) (*snapshot, *setView) {
	snap := s.current()
	sv := snap.byName[r.PathValue("set")]
	if sv == nil {
		problem(w, errorAnswer{Error: fmt.Sprintf("No set named %s is served here.", r.PathValue("set"))})
	}
	return snap, sv
}

func (s *Server) apiSets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.current().list())
}

func (s *Server) apiSet(w http.ResponseWriter, r *http.Request) {
	snap, sv := s.set(w, r)
	if sv == nil {
		return
	}
	writeJSON(w, http.StatusOK, builder{snap, sv}.set())
}

func (s *Server) apiFile(w http.ResponseWriter, r *http.Request) {
	snap, sv := s.set(w, r)
	if sv == nil {
		return
	}
	f := sv.files[r.PathValue("path")]
	if f == nil {
		problem(w, errorAnswer{Error: fmt.Sprintf("No file of %s has the path %s.", sv.Name, r.PathValue("path"))})
		return
	}
	writeJSON(w, http.StatusOK, fileAnswer{Version: snap.version, fileJSON: builder{snap, sv}.file(f, true)})
}

func (s *Server) apiItem(w http.ResponseWriter, r *http.Request) {
	snap, sv := s.set(w, r)
	if sv == nil {
		return
	}
	id := r.PathValue("id")
	if it := sv.item(id); it != nil {
		writeJSON(w, http.StatusOK, itemAnswer{Version: snap.version, itemJSON: builder{snap, sv}.item(it)})
		return
	}
	if e := sv.retiredWith(id); e != nil {
		problem(w, errorAnswer{
			Error:   fmt.Sprintf("%s was retired at %s:%d.", id, e.File.Path, e.Line),
			Retired: &retiredJSON{ID: e.ID, Title: e.Title, File: e.File.Path, Line: e.Line},
		})
		return
	}
	problem(w, errorAnswer{Error: fmt.Sprintf("No item of %s has the id %s.", sv.Name, id)})
}

// apiResolve says what an id names, as a reader would enter it: an id of
// the set, or name/id of another, in any case.
func (s *Server) apiResolve(w http.ResponseWriter, r *http.Request) {
	snap, sv := s.set(w, r)
	if sv == nil {
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	a := resolveAnswer{Version: snap.version}
	target, local := sv, id
	if name, rest, ok := strings.Cut(id, "/"); ok {
		target, local = sv.deps[name], rest
		if target == nil {
			target = snap.byName[name]
		}
		if target == nil {
			a.Ref = refJSON{ID: id, State: "unknown"}
			a.Message = fmt.Sprintf("No set named %s is served here.", name)
			writeJSON(w, http.StatusOK, a)
			return
		}
	}
	if target.item(local) == nil && target.retiredWith(local) == nil {
		local = strings.ToUpper(local)
	}
	b := builder{snap, sv}
	a.Ref = b.refIn(target, local, id)
	switch a.Ref.State {
	case "retired":
		e := target.retiredWith(local)
		a.Message = fmt.Sprintf("%s was retired at %s:%d.", id, e.File.Path, e.Line)
	case "unknown":
		a.Message = fmt.Sprintf("No item has the id %s.", id)
	}
	writeJSON(w, http.StatusOK, a)
}

// events streams the version of the files: once when a stream opens, and
// again whenever the files change.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	ch := s.subscribe()
	defer s.unsubscribe(ch)
	send := func(v string) error {
		fmt.Fprintf(w, "event: version\ndata: %s\n\n", v)
		return rc.Flush()
	}
	if send(s.current().version) != nil {
		return
	}
	keep := time.NewTicker(30 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case v := <-ch:
			if send(v) != nil {
				return
			}
		case <-keep.C:
			fmt.Fprint(w, ": still here\n\n")
			if rc.Flush() != nil {
				return
			}
		}
	}
}

func (s *Server) subscribe() chan string {
	ch := make(chan string, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[ch] = struct{}{}
	return ch
}

func (s *Server) unsubscribe(ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, ch)
}

// subscribers returns how many streams are open.
func (s *Server) subscribers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs)
}

// publish tells every stream the newest version, replacing one not yet sent.
func (s *Server) publish(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case <-ch:
		default:
		}
		ch <- v
	}
}

func (s *Server) reload() {
	snap := load(s.root, s.version())
	s.snap.Store(snap)
	s.publish(snap.version)
}

// snapshot is every set under the root as read at one moment.
type snapshot struct {
	version string
	root    string
	// sets are in manifest path order.
	sets []*setView
	// byName holds the sets served under their name.
	byName  map[string]*setView
	clashes []clash
	outside []finding.Finding
}

// clash is a name that several sets share.
type clash struct {
	name      string
	manifests []string
}

// setView is one set with its findings placed where they are shown.
type setView struct {
	*check.Result
	// served tells whether the set is served under its name.
	served bool
	files  map[string]*model.File
	deps   map[string]*setView
	onItem map[*model.Item][]finding.Finding
	onFile map[*model.File][]finding.Finding
	top    []finding.Finding
}

func (sv *setView) item(id string) *model.Item {
	if sv.Set == nil {
		return nil
	}
	return sv.Set.Item(id)
}

func (sv *setView) retiredWith(id string) *model.Retired {
	if sv.Set == nil {
		return nil
	}
	return sv.Set.RetiredWith(id)
}

func load(root, version string) *snapshot {
	snap := &snapshot{version: version, root: root, byName: map[string]*setView{}}
	results, outside, err := check.Sets(root)
	if err != nil {
		snap.outside = []finding.Finding{{Severity: finding.Error, File: ".", Message: err.Error()}}
		return snap
	}
	snap.outside = outside
	named := map[string][]*setView{}
	var names []string
	for _, r := range results {
		sv := newSetView(r)
		snap.sets = append(snap.sets, sv)
		if r.Name == "" {
			snap.outside = append(snap.outside, r.Findings...)
			continue
		}
		if named[r.Name] == nil {
			names = append(names, r.Name)
		}
		named[r.Name] = append(named[r.Name], sv)
	}
	slices.Sort(names)
	for _, name := range names {
		if svs := named[name]; len(svs) > 1 {
			c := clash{name: name}
			for _, sv := range svs {
				c.manifests = append(c.manifests, sv.Manifest)
			}
			snap.clashes = append(snap.clashes, c)
			continue
		}
		sv := named[name][0]
		sv.served = true
		snap.byName[name] = sv
	}
	for _, sv := range snap.sets {
		sv.deps = snap.resolve(sv)
	}
	return snap
}

func newSetView(r *check.Result) *setView {
	sv := &setView{
		Result: r,
		files:  map[string]*model.File{},
		onItem: map[*model.Item][]finding.Finding{},
		onFile: map[*model.File][]finding.Finding{},
	}
	if r.Set != nil {
		for _, f := range r.Set.Files {
			sv.files[f.Path] = f
		}
	}
	for _, f := range r.Findings {
		file := sv.files[f.File]
		switch it := sv.itemFor(file, f); {
		case file == nil:
			sv.top = append(sv.top, f)
		case it != nil:
			sv.onItem[it] = append(sv.onItem[it], f)
		default:
			sv.onFile[file] = append(sv.onFile[file], f)
		}
	}
	return sv
}

// itemFor returns the item of file that a finding concerns, or nil: of the
// items with the finding's id, the last that starts at or before its line;
// or else the item whose own retired list holds the entry the finding names.
func (sv *setView) itemFor(file *model.File, f finding.Finding) *model.Item {
	if file == nil || f.ID == "" {
		return nil
	}
	var found *model.Item
	var walk func([]*model.Item)
	walk = func(items []*model.Item) {
		for _, it := range items {
			if it.ID == f.ID && (found == nil || it.Node.Line <= f.Line) {
				found = it
			}
			walk(it.Children)
		}
	}
	walk(file.Items)
	if found != nil {
		return found
	}
	for _, e := range sv.Set.Retired {
		if e.File == file && e.ID == f.ID && e.Line == f.Line && e.Owner != nil {
			return e.Owner
		}
	}
	return nil
}

// resolve finds the served sets that a set's dependencies name. Only a
// dependency in the same repository, named by path, can be served here.
func (snap *snapshot) resolve(sv *setView) map[string]*setView {
	deps := sv.Tree.Get("depends")
	if deps == nil || deps.Kind != tree.Map {
		return nil
	}
	out := map[string]*setView{}
	repo := repoRoot(snap.root, filepath.Dir(filepath.Join(snap.root, sv.Manifest)))
	for _, p := range deps.Pairs {
		if p.Value.Get("git") != nil {
			continue
		}
		rel, _ := p.Value.Get("path").Str()
		dir := absolute(filepath.Join(repo, filepath.FromSlash(rel)))
		for _, other := range snap.sets {
			if other.served && absolute(filepath.Dir(filepath.Join(snap.root, other.Manifest))) == dir {
				out[p.Key] = other
			}
		}
	}
	return out
}

// repoRoot returns the nearest directory at or above dir that holds .git, or
// root when there is none.
func repoRoot(root, dir string) string {
	for d := absolute(dir); ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return root
		}
		d = parent
	}
}

func absolute(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return filepath.Clean(p)
}
