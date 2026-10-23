package serve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// safe matches an item's id that can name a file of a static copy, as
	// every id the checker accepts does.
	safe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// basePath matches a path a copy can be served under.
	basePath = regexp.MustCompile(`^/([A-Za-z0-9._~-]+/)*$`)
)

// Export writes into out a static copy of what a server would serve for the
// sets under root: the page, marked as reading a copy served under base,
// its static files, and every answer the page reads, each a JSON file
// named by its address with .json added. Every address a reader can open
// has a copy of the page, and 404.html is one for any other. out must be
// new or empty: Export never deletes. A served set whose name cannot name a
// folder of the copy fails the export.
func Export(root, out, base string) error {
	if !basePath.MatchString(base) {
		return fmt.Errorf("the base %q is not a path that begins and ends with /, such as /shall/", base)
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty", out)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	w := writer{out: out}
	page := copyPage(base)
	w.file("index.html", page)
	w.file("404.html", page)
	static, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		return err
	}
	if err := fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(static, p)
		w.err = errors.Join(w.err, err)
		w.file("-/static/"+p, data)
		return nil
	}); err != nil {
		return err
	}
	snap := load(root, "copy")
	w.json("api/sets.json", snap.list())
	for _, sv := range snap.sets {
		if !sv.served {
			continue
		}
		if !folder(sv.Name) {
			return fmt.Errorf("the set name %q cannot name a folder of the copy", sv.Name)
		}
		b := builder{snap, sv}
		api := "api/sets/" + sv.Name
		w.file(sv.Name+"/index.html", page)
		w.json(api+".json", b.set())
		if sv.Set == nil {
			continue
		}
		for p, f := range sv.files {
			w.json(api+"/files/"+p+".json", fileAnswer{Version: snap.version, fileJSON: b.file(f, true)})
		}
		for _, it := range sv.Set.Items {
			if sv.item(it.ID) != it || !safe.MatchString(it.ID) {
				continue
			}
			w.json(api+"/items/"+it.ID+".json", itemAnswer{Version: snap.version, itemJSON: b.item(it)})
			w.file(sv.Name+"/"+it.ID+".html", page)
		}
		for _, e := range sv.Set.Retired {
			if sv.Set.RetiredEntry(e.ID) != e || !safe.MatchString(e.ID) {
				continue
			}
			w.json(api+"/items/"+e.ID+".json", errorAnswer{
				Error:   fmt.Sprintf("%s was retired at %s:%d.", e.ID, e.File.Path, e.Line),
				Retired: &retiredJSON{ID: e.ID, Title: e.Title, File: e.File.Path, Line: e.Line},
			})
			w.file(sv.Name+"/"+e.ID+".html", page)
		}
	}
	return w.err
}

// folder tells whether a set's name can name a folder of a copy, as the
// page addresses it: one part of a path, beside the copy's own files. A host
// decodes the address the page encodes the name in.
func folder(name string) bool {
	switch name {
	case "", ".", "..", "index.html", "404.html":
		return false
	}
	return !strings.ContainsAny(name, "/\\\x00")
}

// copyPage returns the page as a static copy serves it: under base, reading
// files, and with its policy in a meta tag, as a static host sends none.
func copyPage(base string) []byte {
	meta := fmt.Sprintf("<meta name=\"shall-base\" content=\"%s\">\n<meta name=\"shall-static\" content=\"\">\n"+
		"<meta http-equiv=\"Content-Security-Policy\" content=\"%s\">\n", base, pagePolicy)
	page := bytes.Replace(indexPage, []byte("<meta charset=\"utf-8\">\n"), []byte("<meta charset=\"utf-8\">\n"+meta), 1)
	page = bytes.Replace(page, []byte(`class="home" href="/"`), []byte(`class="home" href="`+base+`"`), 1)
	return bytes.ReplaceAll(page, []byte(`"/-/static/`), []byte(`"`+base+`-/static/`))
}

// writer writes the files of a copy, keeping the first error.
type writer struct {
	out string
	err error
}

func (w *writer) file(name string, data []byte) {
	if w.err != nil {
		return
	}
	clean := path.Clean(name)
	if clean != name || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		w.err = fmt.Errorf("%s would leave %s", name, w.out)
		return
	}
	p := filepath.Join(w.out, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		w.err = err
		return
	}
	w.err = os.WriteFile(p, data, 0o644)
}

func (w *writer) json(name string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		w.err = errors.Join(w.err, err)
		return
	}
	w.file(name, append(data, '\n'))
}
