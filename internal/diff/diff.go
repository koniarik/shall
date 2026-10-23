// Package diff compares the sets under a directory as they stand at two git
// refs, item by item.
package diff

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"shall/internal/check"
	"shall/internal/finding"
	"shall/internal/markdown"
	"shall/internal/model"
	"shall/internal/set"
	"shall/internal/tree"
)

// Result is what differs between two refs.
type Result struct {
	// Base and Head are the refs compared; an empty Head is the working
	// tree.
	Base   string     `json:"base"`
	Head   string     `json:"head"`
	Sets   []*SetDiff `json:"sets"`
	Totals Totals     `json:"totals"`
}

// Changed tells whether anything differs.
func (r *Result) Changed() bool { return len(r.Sets) > 0 }

// SetDiff is what differs in one set, matched by the path of its manifest.
type SetDiff struct {
	Manifest string `json:"manifest"`
	Name     string `json:"name"`
	// Change is added, removed or changed.
	Change string        `json:"change"`
	Fields []FieldChange `json:"fields,omitempty"`
	Files  []FileChange  `json:"files,omitempty"`
	Items  []ItemChange  `json:"items,omitempty"`
	// Findings is nil when no finding is new or gone.
	Findings *Findings `json:"findings,omitempty"`
}

// FieldChange is a field whose value differs, each value as written. A
// field one side lacks has only the other's value; one written as null, or
// with nothing after its colon, has the value null.
type FieldChange struct {
	Field string `json:"field"`
	Old   any    `json:"old,omitempty"`
	New   any    `json:"new,omitempty"`
}

// Values are the old and new value of something worked out, not written.
type Values struct {
	Old any `json:"old,omitempty"`
	New any `json:"new,omitempty"`
}

// FileChange is a chapter added, removed or retitled, the mission file
// among them.
type FileChange struct {
	Path string `json:"path"`
	// Mission tells the mission file from a chapter.
	Mission bool `json:"mission,omitempty"`
	// Change is added, removed or retitled.
	Change string  `json:"change"`
	Title  *Values `json:"title,omitempty"`
}

// ItemChange is an item added, removed, retired or changed.
type ItemChange struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// File holds the item at the head, or at the base for an item no
	// longer there.
	File string `json:"file"`
	// Change is added, removed, retired or changed.
	Change string `json:"change"`
	// MovedFrom is the file that held the item at the base, when it is
	// another.
	MovedFrom       string        `json:"movedFrom,omitempty"`
	Fields          []FieldChange `json:"fields,omitempty"`
	EffectiveStatus *Values       `json:"effectiveStatus,omitempty"`
}

// Findings are the checker's findings new at the head, and those gone.
type Findings struct {
	New  []finding.Finding `json:"new,omitempty"`
	Gone []finding.Finding `json:"gone,omitempty"`
}

// Totals count the items by what happened to them; a moved item counts as
// moved besides.
type Totals struct {
	Changed int `json:"changed"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Retired int `json:"retired"`
	Moved   int `json:"moved"`
}

// Between reads the sets under dir at base and at head and compares them. It
// fails when neither ref holds a set, since there is then nothing to
// compare.
func Between(dir, base, head string) (*Result, error) {
	before, err := Read(dir, base)
	if err != nil {
		return nil, err
	}
	after, err := Read(dir, head)
	if err != nil {
		return nil, err
	}
	if len(before) == 0 && len(after) == 0 {
		return nil, fmt.Errorf("no %s under %s at either ref", set.ManifestName, dir)
	}
	return Compare(base, head, before, after), nil
}

// Compare compares the sets read at base with those read at head.
func Compare(baseRef, headRef string, base, head []*check.Result) *Result {
	r := &Result{Base: baseRef, Head: headRef, Sets: []*SetDiff{}}
	byPath := func(results []*check.Result) map[string]*check.Result {
		m := map[string]*check.Result{}
		for _, res := range results {
			m[res.Manifest] = res
		}
		return m
	}
	bases, heads := byPath(base), byPath(head)
	var paths []string
	for p := range bases {
		paths = append(paths, p)
	}
	for p := range heads {
		if bases[p] == nil {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	for _, p := range paths {
		if d := compareSet(bases[p], heads[p]); d != nil {
			r.Sets = append(r.Sets, d)
			for _, it := range d.Items {
				switch it.Change {
				case "added":
					r.Totals.Added++
				case "removed":
					r.Totals.Removed++
				case "retired":
					r.Totals.Retired++
				case "changed":
					r.Totals.Changed++
				}
				if it.MovedFrom != "" {
					r.Totals.Moved++
				}
			}
		}
	}
	return r
}

// side is one set as read at one ref; its fields are empty when the set is
// not there.
type side struct {
	res   *check.Result
	set   *model.Set
	items map[string]*model.Item
	order []*model.Item
	files map[string]*model.File
}

// sideOf returns a set's side. Of two items with one path, the first read
// is compared, as everywhere else; the checker reports the second.
func sideOf(res *check.Result) side {
	s := side{res: res, items: map[string]*model.Item{}, files: map[string]*model.File{}}
	if res == nil || res.Set == nil {
		return s
	}
	s.set = res.Set
	for _, it := range res.Set.Items {
		if res.Set.Item(it.ID) == it {
			s.items[it.ID] = it
			s.order = append(s.order, it)
		}
	}
	for _, f := range res.Set.Files {
		s.files[f.Path] = f
	}
	return s
}

func (s side) manifest(key string) any {
	if s.res == nil {
		return nil
	}
	return plain(s.res.Tree.Get(key))
}

func (s side) mission(key string) any {
	if s.set == nil {
		return nil
	}
	for _, f := range s.set.Files {
		if f.Mission && f.Tree != nil {
			return plain(f.Tree.Get(key))
		}
	}
	return nil
}

// compareSet returns what differs in one set, or nil when nothing does.
func compareSet(baseRes, headRes *check.Result) *SetDiff {
	b, h := sideOf(baseRes), sideOf(headRes)
	d := &SetDiff{Change: "changed"}
	switch {
	case baseRes == nil:
		d.Change, d.Manifest, d.Name = "added", headRes.Manifest, headRes.Name
	case headRes == nil:
		d.Change, d.Manifest, d.Name = "removed", baseRes.Manifest, baseRes.Name
	default:
		d.Manifest, d.Name = headRes.Manifest, headRes.Name
	}
	for _, key := range []string{"name", "path", "depends"} {
		d.Fields = appendChange(d.Fields, key, b.manifest(key), h.manifest(key))
	}
	for _, key := range []string{"mission", "ranking"} {
		d.Fields = appendChange(d.Fields, key, b.mission(key), h.mission(key))
	}
	d.Files = compareFiles(b, h)
	d.Items = compareItems(b, h)
	if f := compareFindings(baseRes, headRes); len(f.New)+len(f.Gone) > 0 {
		d.Findings = &f
	}
	if d.Change == "changed" && len(d.Fields) == 0 && len(d.Files) == 0 && len(d.Items) == 0 && d.Findings == nil {
		return nil
	}
	return d
}

func compareFiles(b, h side) []FileChange {
	var paths []string
	for p := range b.files {
		paths = append(paths, p)
	}
	for p := range h.files {
		if b.files[p] == nil {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	var out []FileChange
	for _, p := range paths {
		bf, hf := b.files[p], h.files[p]
		switch {
		case bf == nil:
			out = append(out, FileChange{Path: p, Mission: hf.Mission, Change: "added", Title: &Values{New: nonEmpty(hf.Title())}})
		case hf == nil:
			out = append(out, FileChange{Path: p, Mission: bf.Mission, Change: "removed", Title: &Values{Old: nonEmpty(bf.Title())}})
		case fold(bf.Title()) != fold(hf.Title()):
			out = append(out, FileChange{Path: p, Mission: hf.Mission, Change: "retitled", Title: &Values{Old: nonEmpty(bf.Title()), New: nonEmpty(hf.Title())}})
		}
	}
	return out
}

func nonEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// compareItems returns the items that differ: those at the head in reading
// order, then those gone, in the base's.
func compareItems(b, h side) []ItemChange {
	var out []ItemChange
	for _, it := range h.order {
		old := b.items[it.ID]
		if old == nil {
			out = append(out, ItemChange{ID: it.ID, Title: it.Title(), File: it.File.Path, Change: "added"})
			continue
		}
		c := ItemChange{ID: it.ID, Title: it.Title(), File: it.File.Path, Change: "changed"}
		if old.File.Path != it.File.Path {
			c.MovedFrom = old.File.Path
		}
		c.Fields = compareFields(it.Kind, old.Node, it.Node)
		// The effective status is reported where it says more than the
		// written one, as when a child lowers its parent.
		oldEffective, newEffective := old.EffectiveStatus(), it.EffectiveStatus()
		if oldEffective != newEffective && (oldEffective != old.Status() || newEffective != it.Status()) {
			c.EffectiveStatus = &Values{Old: nonEmpty(oldEffective), New: nonEmpty(newEffective)}
		}
		if c.MovedFrom != "" || len(c.Fields) > 0 || c.EffectiveStatus != nil {
			out = append(out, c)
		}
	}
	for _, it := range b.order {
		if h.items[it.ID] != nil {
			continue
		}
		change := "removed"
		if h.set != nil && h.set.RetiredWith(it.ID) != nil {
			change = "retired"
		}
		out = append(out, ItemChange{ID: it.ID, Title: it.Title(), File: it.File.Path, Change: change})
	}
	return out
}

// compareFields returns the fields an item of a kind writes whose values
// differ, in the order the head writes them, then those only the base
// writes. Children and retired entries are items of their own. A topic's
// text differs where its Markdown reads as other blocks.
func compareFields(kind byte, old, cur *tree.Node) []FieldChange {
	var keys []string
	for _, n := range []*tree.Node{cur, old} {
		for _, p := range n.Pairs {
			if !slices.Contains(keys, p.Key) {
				keys = append(keys, p.Key)
			}
		}
	}
	var out []FieldChange
	for _, k := range keys {
		switch k {
		case "id", "children", "retired":
			continue
		}
		o, c := plain(old.Get(k)), plain(cur.Get(k))
		a, aText := o.(string)
		b, bText := c.(string)
		if kind == 'T' && k == "text" && aText && bText {
			if !reflect.DeepEqual(markdown.Blocks(a), markdown.Blocks(b)) {
				out = append(out, FieldChange{Field: k, Old: o, New: c})
			}
			continue
		}
		out = appendChange(out, k, o, c)
	}
	return out
}

// appendChange adds a field's change to out, unless its values differ in
// nothing but whitespace.
func appendChange(out []FieldChange, field string, old, cur any) []FieldChange {
	if reflect.DeepEqual(fold(old), fold(cur)) {
		return out
	}
	return append(out, FieldChange{Field: field, Old: old, New: cur})
}

// fold returns a plain value with every run of whitespace in its texts made
// one space, and none at their ends.
func fold(v any) any {
	switch v := v.(type) {
	case string:
		return strings.Join(strings.Fields(v), " ")
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = fold(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			out[k] = fold(x)
		}
		return out
	}
	return v
}

// null is a value written as null, or as nothing after its colon. It is
// not the nil of a value not written at all, and JSON writes it null.
type null struct{}

func (null) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// plain returns a node's value as written, as Go values: a scalar's value,
// a list of values, a map of them or null; nil when there is no node.
func plain(n *tree.Node) any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case tree.Null:
		return null{}
	case tree.Scalar:
		return n.Value
	case tree.Seq:
		out := []any{}
		for _, it := range n.Items {
			out = append(out, plain(it))
		}
		return out
	case tree.Map:
		out := map[string]any{}
		for _, p := range n.Pairs {
			out[p.Key] = plain(p.Value)
		}
		return out
	}
	return nil
}

// position is a place in a set's files as a message names it, file:line.
var position = regexp.MustCompile(regexp.QuoteMeta(set.ManifestName) + `:\d+`)

// compareFindings returns the findings new at the head and those gone. A
// finding is the same on both sides when all but its line is, and the lines
// its message names; lines move with every edit above them.
func compareFindings(baseRes, headRes *check.Result) Findings {
	key := func(f finding.Finding) string {
		message := position.ReplaceAllString(f.Message, set.ManifestName)
		return strings.Join([]string{string(f.Severity), f.File, f.ID, f.Field, message}, "\x00")
	}
	count := map[string]int{}
	if baseRes != nil {
		for _, f := range baseRes.Findings {
			count[key(f)]++
		}
	}
	var out Findings
	if headRes != nil {
		for _, f := range headRes.Findings {
			if count[key(f)] > 0 {
				count[key(f)]--
			} else {
				out.New = append(out.New, f)
			}
		}
	}
	if baseRes != nil {
		for _, f := range baseRes.Findings {
			if count[key(f)] > 0 {
				count[key(f)]--
				out.Gone = append(out.Gone, f)
			}
		}
	}
	return out
}
