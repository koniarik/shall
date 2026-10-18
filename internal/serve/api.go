package serve

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"

	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/tree"
)

// The answers of the API. Fields keep the names the format gives them; what
// the server works out is named in camelCase. Every answer carries the
// version of the files it was built from.

type errorAnswer struct {
	Error   string       `json:"error"`
	Retired *retiredJSON `json:"retired,omitempty"`
}

type setsAnswer struct {
	Version string       `json:"version"`
	Sets    []setSummary `json:"sets"`
	Clashes []clashJSON  `json:"clashes"`
	// Findings are those that belong to no set served under its name.
	Findings []finding.Finding `json:"findings"`
}

type setSummary struct {
	Name     string `json:"name"`
	Manifest string `json:"manifest"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

type clashJSON struct {
	Name      string   `json:"name"`
	Manifests []string `json:"manifests"`
}

type setAnswer struct {
	Version  string `json:"version"`
	Name     string `json:"name"`
	Manifest string `json:"manifest"`
	// Findings are those that concern no file of the set.
	Findings []finding.Finding `json:"findings"`
	// Files are the mission file, with its items, then the chapters, without
	// theirs.
	Files []*fileJSON `json:"files"`
}

type fileJSON struct {
	Path string `json:"path"`
	// Kind is mission or chapter.
	Kind   string `json:"kind"`
	Broken bool   `json:"broken,omitempty"`
	Title  string `json:"title,omitempty"`
	Counts counts `json:"counts"`
	// Findings are those that concern the file and none of its items.
	Findings     []finding.Finding `json:"findings"`
	Mission      string            `json:"mission,omitempty"`
	Ranking      []refJSON         `json:"ranking,omitempty"`
	Objectives   []*itemJSON       `json:"objectives,omitempty"`
	Stakeholders []*itemJSON       `json:"stakeholders,omitempty"`
	Needs        []*itemJSON       `json:"needs,omitempty"`
	Requirements []*itemJSON       `json:"requirements,omitempty"`
	Retired      []retiredJSON     `json:"retired,omitempty"`
}

// counts are the items a file holds and the findings shown in it.
type counts struct {
	Items    int `json:"items"`
	Findings int `json:"findings"`
}

type fileAnswer struct {
	Version string `json:"version"`
	*fileJSON
}

type itemAnswer struct {
	Version string `json:"version"`
	*itemJSON
}

type resolveAnswer struct {
	Version string  `json:"version"`
	Ref     refJSON `json:"ref"`
	// Message says why the id names no item.
	Message string `json:"message,omitempty"`
}

type searchAnswer struct {
	Version string `json:"version"`
	Query   string `json:"query"`
	// Results are the items found, those matching by id first, then by
	// title, then by text, each group in reading order.
	Results []searchResult `json:"results"`
	// More counts the items found beyond those listed.
	More int `json:"more,omitempty"`
	// Message says why nothing was found.
	Message string `json:"message,omitempty"`
}

type searchResult struct {
	// Match is id, title or text: where the words were found.
	Match string `json:"match"`
	refJSON
}

type itemJSON struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Text  string `json:"text"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	// Status is the status as written; EffectiveStatus is a requirement's
	// lowest status counting its children.
	Status          string            `json:"status,omitempty"`
	EffectiveStatus string            `json:"effectiveStatus,omitempty"`
	Measure         string            `json:"measure,omitempty"`
	Stakeholder     *refJSON          `json:"stakeholder,omitempty"`
	Serves          []refJSON         `json:"serves,omitempty"`
	Rationale       string            `json:"rationale,omitempty"`
	Verify          string            `json:"verify,omitempty"`
	Notes           []string          `json:"notes,omitempty"`
	Links           []linkJSON        `json:"links,omitempty"`
	Mirrors         *refJSON          `json:"mirrors,omitempty"`
	NamedBy         []namedByJSON     `json:"namedBy,omitempty"`
	Findings        []finding.Finding `json:"findings,omitempty"`
	Retired         []retiredJSON     `json:"retired,omitempty"`
	Children        []*itemJSON       `json:"children,omitempty"`
}

// refJSON is what an id written in a field names.
type refJSON struct {
	// ID is the id as written, name/id for an item of another set.
	ID string `json:"id"`
	// State is item, retired, unknown, or mission for the word in a need's
	// serves.
	State string `json:"state"`
	// Set is the set the id resolves in, and ItemID the item's id there
	// where it differs from ID.
	Set    string `json:"set,omitempty"`
	ItemID string `json:"itemId,omitempty"`
	// Title is the item's, or the title a retired id was retired under.
	Title string `json:"title,omitempty"`
	// File is the file that holds the item.
	File string `json:"file,omitempty"`
	// RetiredWith is the retired ancestor that took the id with it.
	RetiredWith string `json:"retiredWith,omitempty"`
}

type namedByJSON struct {
	Field string `json:"field"`
	refJSON
}

type linkJSON struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type retiredJSON struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	File  string `json:"file"`
	Line  int    `json:"line"`
}

var kinds = map[byte]string{'O': "objective", 'S': "stakeholder", 'N': "need", 'R': "requirement"}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (snap *snapshot) list() setsAnswer {
	a := setsAnswer{
		Version: snap.version, Sets: []setSummary{}, Clashes: []clashJSON{},
		Findings: orEmpty(snap.outside),
	}
	for _, sv := range snap.sets {
		if !sv.served {
			continue
		}
		s := setSummary{Name: sv.Name, Manifest: sv.Manifest}
		for _, f := range sv.Findings {
			if f.Severity == finding.Error {
				s.Errors++
			} else {
				s.Warnings++
			}
		}
		a.Sets = append(a.Sets, s)
	}
	for _, c := range snap.clashes {
		a.Clashes = append(a.Clashes, clashJSON{Name: c.name, Manifests: c.manifests})
	}
	return a
}

// builder builds the answers about one set.
type builder struct {
	snap *snapshot
	sv   *setView
}

func (b builder) set() setAnswer {
	a := setAnswer{
		Version: b.snap.version, Name: b.sv.Name, Manifest: b.sv.Manifest,
		Findings: orEmpty(b.sv.top), Files: []*fileJSON{},
	}
	if b.sv.Set != nil {
		for _, f := range b.sv.Set.Files {
			a.Files = append(a.Files, b.file(f, f.Mission))
		}
	}
	return a
}

// file returns a file's answer, with its items when withItems is true.
func (b builder) file(f *model.File, withItems bool) *fileJSON {
	v := &fileJSON{
		Path: f.Path, Kind: "chapter", Broken: f.Tree == nil,
		Counts: b.count(f), Findings: orEmpty(b.sv.onFile[f]),
	}
	if f.Mission {
		v.Kind = "mission"
	}
	if v.Broken {
		return v
	}
	v.Title = f.Title()
	v.Retired = retiredList(f.Retired)
	if f.Mission {
		v.Mission, _ = f.Tree.Get("mission").Str()
		for _, r := range b.sv.Set.Ranking() {
			if r.File == f {
				v.Ranking = append(v.Ranking, b.ref(r.ID))
			}
		}
	}
	if !withItems {
		return v
	}
	for _, it := range f.Items {
		j := b.item(it)
		switch it.Kind {
		case 'O':
			v.Objectives = append(v.Objectives, j)
		case 'S':
			v.Stakeholders = append(v.Stakeholders, j)
		case 'N':
			v.Needs = append(v.Needs, j)
		case 'R':
			v.Requirements = append(v.Requirements, j)
		}
	}
	return v
}

// count returns the number of items in a file and of findings shown in it.
func (b builder) count(f *model.File) counts {
	c := counts{Findings: len(b.sv.onFile[f])}
	var walk func([]*model.Item)
	walk = func(items []*model.Item) {
		for _, it := range items {
			c.Items++
			c.Findings += len(b.sv.onItem[it])
			walk(it.Children)
		}
	}
	walk(f.Items)
	return c
}

func (b builder) item(it *model.Item) *itemJSON {
	n := it.Node
	v := &itemJSON{
		ID: it.ID, Kind: kinds[it.Kind], Title: it.Title(), Text: it.Text(),
		File: it.File.Path, Line: n.Line, Findings: b.sv.onItem[it],
	}
	switch it.Kind {
	case 'O':
		v.Measure, _ = n.Get("measure").Str()
	case 'N':
		v.Status = it.Status()
		if s, ok := n.Get("stakeholder").Str(); ok {
			r := b.ref(s)
			v.Stakeholder = &r
		}
	case 'R':
		v.Status, v.EffectiveStatus = it.Status(), it.EffectiveStatus()
		v.Rationale, _ = n.Get("rationale").Str()
		v.Verify, _ = n.Get("verify").Str()
	}
	if list := n.Get("serves"); (it.Kind == 'N' || it.Kind == 'R') && list != nil && list.Kind == tree.Seq {
		v.Serves = []refJSON{}
		for _, x := range list.Items {
			id, ok := x.Str()
			switch {
			case !ok:
			case it.Kind == 'N' && id == "mission":
				v.Serves = append(v.Serves, refJSON{ID: id, State: "mission"})
			default:
				v.Serves = append(v.Serves, b.ref(id))
			}
		}
	}
	if m, ok := n.Get("mirrors").Str(); ok {
		r := b.ref(m)
		v.Mirrors = &r
	}
	if list := n.Get("notes"); list != nil {
		for _, x := range list.Items {
			if s, ok := x.Str(); ok {
				v.Notes = append(v.Notes, s)
			}
		}
	}
	if list := n.Get("links"); list != nil {
		for _, x := range list.Items {
			u, _ := x.Get("url").Str()
			t, _ := x.Get("title").Str()
			v.Links = append(v.Links, linkJSON{URL: u, Title: t})
		}
	}
	for _, r := range b.sv.Set.NamedBy(it.ID) {
		v.NamedBy = append(v.NamedBy, namedByJSON{Field: r.Field, refJSON: b.refIn(b.sv, r.From.ID, r.From.ID)})
	}
	v.Retired = retiredList(it.Retired)
	for _, c := range it.Children {
		v.Children = append(v.Children, b.item(c))
	}
	return v
}

// idLike matches what a reader types as an id rather than as a word.
var idLike = regexp.MustCompile(`^[OSNRosnr][0-9]+(\.[0-9]+)*$`)

// searchLimit is the most results a search lists.
const searchLimit = 50

// resolve returns what an id names as a reader would type it: an id of the
// set, or name/id of a set it depends on or the server serves, matched
// exactly or else in capitals. The message says why it names no item.
func (b builder) resolve(id string) (refJSON, string) {
	target, local := b.sv, id
	if name, rest, ok := strings.Cut(id, "/"); ok {
		target, local = b.sv.deps[name], rest
		if target == nil {
			target = b.snap.byName[name]
		}
		if target == nil {
			return refJSON{ID: id, State: "unknown"}, fmt.Sprintf("No set named %s is served here.", name)
		}
	}
	if target.item(local) == nil && target.retiredWith(local) == nil {
		local = strings.ToUpper(local)
	}
	r := b.refIn(target, local, id)
	switch r.State {
	case "retired":
		e := target.retiredWith(local)
		return r, fmt.Sprintf("%s was retired at %s:%d.", id, e.File.Path, e.Line)
	case "unknown":
		return r, fmt.Sprintf("No item has the id %s.", id)
	}
	return r, ""
}

// search returns what a reader typed names: the item a single word names as
// an id, then the items of the set whose id or title holds every word, then
// those whose text does, ignoring case.
func (b builder) search(q string) searchAnswer {
	a := searchAnswer{Version: b.snap.version, Query: q, Results: []searchResult{}}
	words := strings.Fields(strings.ToLower(q))
	if len(words) == 0 || b.sv.Set == nil {
		return a
	}
	var exact refJSON
	var message string
	if len(words) == 1 {
		exact, message = b.resolve(strings.TrimSpace(q))
		if exact.State == "item" {
			a.Results = append(a.Results, searchResult{Match: "id", refJSON: exact})
		}
	}
	holds := func(s string) bool {
		s = strings.ToLower(s)
		for _, w := range words {
			if !strings.Contains(s, w) {
				return false
			}
		}
		return true
	}
	var byMatch [2][]*model.Item
	for _, it := range b.sv.Set.Items {
		switch {
		case b.sv.Set.Item(it.ID) != it, exact.State == "item" && exact.Set == b.sv.Name && it.ID == cmp.Or(exact.ItemID, exact.ID):
		case holds(it.ID + " " + it.Title()):
			byMatch[0] = append(byMatch[0], it)
		case holds(it.ID + " " + it.Title() + " " + it.Text()):
			byMatch[1] = append(byMatch[1], it)
		}
	}
	for i, match := range []string{"title", "text"} {
		for _, it := range byMatch[i] {
			if len(a.Results) == searchLimit {
				a.More++
				continue
			}
			a.Results = append(a.Results, searchResult{Match: match, refJSON: b.refIn(b.sv, it.ID, it.ID)})
		}
	}
	if len(a.Results) == 0 {
		a.Message = "No item holds " + strings.Join(words, ", ") + "."
		if exact.State == "retired" || strings.Contains(q, "/") || idLike.MatchString(q) {
			a.Message = message
		}
	}
	return a
}

func retiredList(entries []*model.Retired) []retiredJSON {
	var out []retiredJSON
	for _, e := range entries {
		out = append(out, retiredJSON{ID: e.ID, Title: e.Title, File: e.File.Path, Line: e.Line})
	}
	return out
}

// ref returns what an id written in a field of the set names: an id of the
// set, or name/id of a set it depends on.
func (b builder) ref(id string) refJSON {
	if name, rest, ok := strings.Cut(id, "/"); ok {
		target := b.sv.deps[name]
		if target == nil {
			return refJSON{ID: id, State: "unknown"}
		}
		return b.refIn(target, rest, id)
	}
	return b.refIn(b.sv, id, id)
}

// refIn returns what id names in target, written as written.
func (b builder) refIn(target *setView, id, written string) refJSON {
	r := refJSON{ID: written, State: "unknown"}
	if !target.served {
		return r
	}
	if id != written {
		r.ItemID = id
	}
	if e := target.retiredWith(id); e != nil {
		r.State, r.Set, r.Title = "retired", target.Name, e.Title
		if e.ID != id {
			r.RetiredWith = e.ID
		}
		return r
	}
	it := target.item(id)
	if it == nil {
		r.ItemID = ""
		return r
	}
	r.State, r.Set, r.Title, r.File = "item", target.Name, it.Title(), it.File.Path
	return r
}
