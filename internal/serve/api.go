package serve

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"

	"shall/internal/dot"
	"shall/internal/finding"
	"shall/internal/markdown"
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
	// Requirements counts the set's requirements, children included, by
	// effective status.
	Requirements statusCounts `json:"requirements"`
}

type statusCounts struct {
	Total       int `json:"total"`
	Draft       int `json:"draft"`
	Approved    int `json:"approved"`
	Implemented int `json:"implemented"`
	Verified    int `json:"verified"`
	Waived      int `json:"waived"`
	// Other counts requirements with no status, or one the format does not
	// define.
	Other int `json:"other"`
}

func (c *statusCounts) addAll(o statusCounts) {
	c.Total += o.Total
	c.Draft += o.Draft
	c.Approved += o.Approved
	c.Implemented += o.Implemented
	c.Verified += o.Verified
	c.Waived += o.Waived
	c.Other += o.Other
}

func (c *statusCounts) add(status string) {
	c.Total++
	switch status {
	case "draft":
		c.Draft++
	case "approved":
		c.Approved++
	case "implemented":
		c.Implemented++
	case "verified":
		c.Verified++
	case "waived":
		c.Waived++
	default:
		c.Other++
	}
}

type clashJSON struct {
	Name      string   `json:"name"`
	Manifests []string `json:"manifests"`
}

type setAnswer struct {
	// Version is left out of a dump, which is no answer of a running
	// server.
	Version  string `json:"version,omitempty"`
	Name     string `json:"name"`
	Manifest string `json:"manifest"`
	// Requirements counts the set's requirements by effective status.
	Requirements statusCounts `json:"requirements"`
	// Findings are those that concern no file of the set.
	Findings []finding.Finding `json:"findings"`
	// Files are the mission file, with its items, then the chapters, without
	// theirs.
	Files []*fileJSON `json:"files"`
	// Depends gives, for each name the manifest gives a set it depends on,
	// the name of the served set it names.
	Depends map[string]string `json:"depends,omitempty"`
}

type fileJSON struct {
	Path string `json:"path"`
	// Kind is mission or chapter.
	Kind   string `json:"kind"`
	Broken bool   `json:"broken,omitempty"`
	Title  string `json:"title,omitempty"`
	Counts counts `json:"counts"`
	// Findings are those that concern the file and none of its items.
	Findings []finding.Finding `json:"findings"`
	Mission  string            `json:"mission,omitempty"`
	Ranking  []refJSON         `json:"ranking,omitempty"`
	Items    []*itemJSON       `json:"items,omitempty"`
	Retired  []retiredJSON     `json:"retired,omitempty"`
	// Cites holds the mission statement when it cites an id.
	Cites *citesJSON `json:"cites,omitempty"`
}

// counts are the items a file holds, the findings shown in it, and its
// requirements by effective status.
type counts struct {
	Items        int          `json:"items"`
	Findings     int          `json:"findings"`
	Requirements statusCounts `json:"requirements"`
}

type fileAnswer struct {
	Version string `json:"version"`
	*fileJSON
}

type itemAnswer struct {
	Version string `json:"version"`
	*itemJSON
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
	Status          string `json:"status,omitempty"`
	EffectiveStatus string `json:"effectiveStatus,omitempty"`
	Measure         string `json:"measure,omitempty"`
	// Rank is an objective's place in the mission's ranking, from 1, where
	// the ranking lists it.
	Rank         int               `json:"rank,omitempty"`
	Stakeholders []refJSON         `json:"stakeholders,omitempty"`
	Serves       []refJSON         `json:"serves,omitempty"`
	Rationale    string            `json:"rationale,omitempty"`
	Verify       string            `json:"verify,omitempty"`
	Notes        []string          `json:"notes,omitempty"`
	Links        []linkJSON        `json:"links,omitempty"`
	Mirrors      *refJSON          `json:"mirrors,omitempty"`
	NamedBy      []namedByJSON     `json:"namedBy,omitempty"`
	Findings     []finding.Finding `json:"findings,omitempty"`
	Retired      []retiredJSON     `json:"retired,omitempty"`
	Children     []*itemJSON       `json:"children,omitempty"`
	// Requirements counts a topic's requirements, all below it, by effective
	// status.
	Requirements *statusCounts `json:"requirements,omitempty"`
	// Blocks are a topic's text read as Markdown, each id it cites a ref
	// run.
	Blocks []markdown.Block `json:"blocks,omitempty"`
	// Cites are the item's prose fields that cite, as runs of text and refs.
	Cites *citesJSON `json:"cites,omitempty"`
}

// citesJSON holds each prose field that cites an id, as runs of text and
// refs; a field that cites none is left out.
type citesJSON struct {
	Text      []markdown.Inline   `json:"text,omitempty"`
	Measure   []markdown.Inline   `json:"measure,omitempty"`
	Rationale []markdown.Inline   `json:"rationale,omitempty"`
	Notes     [][]markdown.Inline `json:"notes,omitempty"`
	Mission   []markdown.Inline   `json:"mission,omitempty"`
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

var kinds = map[byte]string{'O': "objective", 'S': "stakeholder", 'N': "need", 'R': "requirement", 'T': "topic"}

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
		if sv.Set != nil {
			for _, it := range sv.Set.Items {
				if it.Kind == 'R' && sv.Set.Item(it.ID) == it {
					s.Requirements.add(it.EffectiveStatus())
				}
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
			v := b.file(f, f.Mission)
			a.Requirements.addAll(v.Counts.Requirements)
			a.Files = append(a.Files, v)
		}
	}
	for name, dep := range b.sv.deps {
		if a.Depends == nil {
			a.Depends = map[string]string{}
		}
		a.Depends[name] = dep.Name
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
		if runs := b.runs(v.Mission); runs != nil {
			v.Cites = &citesJSON{Mission: runs}
		}
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
		v.Items = append(v.Items, b.item(it))
	}
	return v
}

// count returns the number of items in a file, of findings shown in it, and
// of its requirements at each effective status.
func (b builder) count(f *model.File) counts {
	c := counts{Findings: len(b.sv.onFile[f])}
	var walk func([]*model.Item)
	walk = func(items []*model.Item) {
		for _, it := range items {
			c.Items++
			c.Findings += len(b.sv.onItem[it])
			if it.Kind == 'R' && b.sv.Set.Item(it.ID) == it {
				c.Requirements.add(it.EffectiveStatus())
			}
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
		for i, r := range b.sv.Set.Ranking() {
			if r.ID == it.ID {
				v.Rank = i + 1
				break
			}
		}
	case 'N':
		v.Status = it.Status()
		if list := n.Get("stakeholders"); list != nil && list.Kind == tree.Seq {
			for _, x := range list.Items {
				if id, ok := x.Str(); ok {
					v.Stakeholders = append(v.Stakeholders, b.ref(id))
				}
			}
		}
	case 'R':
		v.Status, v.EffectiveStatus = it.Status(), it.EffectiveStatus()
		v.Rationale, _ = n.Get("rationale").Str()
		v.Verify, _ = n.Get("verify").Str()
	case 'T':
		if v.Text != "" {
			v.Blocks = drawn(b.cited(markdown.Blocks(v.Text)))
		}
		v.Requirements = &statusCounts{}
		var below func(*model.Item)
		below = func(x *model.Item) {
			for _, c := range x.Children {
				if c.Kind == 'R' && b.sv.Set.Item(c.ID) == c {
					v.Requirements.add(c.EffectiveStatus())
				}
				below(c)
			}
		}
		below(it)
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
	c := citesJSON{Measure: b.runs(v.Measure), Rationale: b.runs(v.Rationale)}
	if it.Kind != 'T' {
		c.Text = b.runs(v.Text)
	}
	for i, note := range v.Notes {
		if runs := b.runs(note); runs != nil {
			if c.Notes == nil {
				c.Notes = make([][]markdown.Inline, len(v.Notes))
			}
			c.Notes[i] = runs
		}
	}
	if c.Text != nil || c.Measure != nil || c.Rationale != nil || c.Notes != nil {
		v.Cites = &c
	}
	v.Retired = retiredList(it.Retired)
	for _, c := range it.Children {
		v.Children = append(v.Children, b.item(c))
	}
	return v
}

// idLike matches what a reader types as an id rather than as a word.
var idLike = regexp.MustCompile(`^[OSNRTosnrt][0-9]+(\.[0-9]+)*$`)

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
	if len(words) == 0 {
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
	var items []*model.Item
	if b.sv.Set != nil {
		items = b.sv.Set.Items
	}
	var byMatch [2][]*model.Item
	for _, it := range items {
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

// runs returns a text that cites as runs of text and refs, or nil when it
// cites nothing.
func (b builder) runs(text string) []markdown.Inline {
	cites := model.Cites(text)
	if len(cites) == 0 {
		return nil
	}
	var out []markdown.Inline
	at := 0
	for _, c := range cites {
		if c.Start > at {
			out = append(out, markdown.Inline{Type: "text", Text: text[at:c.Start]})
		}
		r := b.ref(c.ID)
		out = append(out, markdown.Inline{Type: "ref", Ref: &r})
		at = c.End
	}
	if at < len(text) {
		out = append(out, markdown.Inline{Type: "text", Text: text[at:]})
	}
	return out
}

// drawn returns blocks with each DOT diagram drawn, or the reason it cannot
// be; Mermaid's are drawn in the page.
func drawn(blocks []markdown.Block) []markdown.Block {
	for i := range blocks {
		bl := &blocks[i]
		if bl.Type == "diagram" && bl.Language == "dot" {
			svg, err := dot.SVG(bl.Text)
			if err != nil {
				bl.Error = err.Error()
			}
			bl.SVG = svg
		}
		for j := range bl.Items {
			bl.Items[j] = drawn(bl.Items[j])
		}
		bl.Blocks = drawn(bl.Blocks)
	}
	return blocks
}

// cited returns blocks with each id their text cites made a ref run.
func (b builder) cited(blocks []markdown.Block) []markdown.Block {
	var inline func([]markdown.Inline) []markdown.Inline
	inline = func(list []markdown.Inline) []markdown.Inline {
		var out []markdown.Inline
		for _, x := range list {
			switch x.Type {
			case "text":
				if runs := b.runs(x.Text); runs != nil {
					out = append(out, runs...)
					continue
				}
			case "emphasis", "strong", "link":
				x.Inline = inline(x.Inline)
			}
			out = append(out, x)
		}
		return out
	}
	for i := range blocks {
		bl := &blocks[i]
		bl.Inline = inline(bl.Inline)
		for j := range bl.Items {
			bl.Items[j] = b.cited(bl.Items[j])
		}
		bl.Blocks = b.cited(bl.Blocks)
		for j := range bl.Head {
			bl.Head[j] = inline(bl.Head[j])
		}
		for _, row := range bl.Rows {
			for j := range row {
				row[j] = inline(row[j])
			}
		}
	}
	return blocks
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
