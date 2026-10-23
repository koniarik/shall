// Package model reads the parsed files of one set into items that know their
// file, their parent and children, the ids they name and the items that name
// them.
package model

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"shall/internal/set"
	"shall/internal/tree"
)

// File is one file of a set.
type File struct {
	// Path names the file as findings name it.
	Path string
	// Tree is the file's content, or nil when it could not be read or parsed.
	Tree *tree.Node
	// Mission tells the mission file from a chapter. New sets it from the
	// file's name.
	Mission bool
	// Items are the file's top-level items, in written order.
	Items []*Item
	// Retired are the entries of the file's own retired list.
	Retired []*Retired
}

// Title returns the chapter's title, or "" when it has none.
func (f *File) Title() string {
	s, _ := f.Tree.Get("title").Str()
	return s
}

// Item is one objective, stakeholder, need, requirement or topic.
type Item struct {
	// Kind is the letter its id starts with: 'O', 'S', 'N', 'R' or 'T'.
	Kind     byte
	ID       string
	File     *File
	Node     *tree.Node
	Parent   *Item
	Children []*Item
	// Retired are the entries of the item's own retired list.
	Retired []*Retired
}

// Path returns the path of the item's id, which names the item whatever its
// letter.
func (it *Item) Path() string { return Path(it.ID) }

// Path returns the path of an id: the id without its letter.
func Path(id string) string {
	if id == "" {
		return ""
	}
	return id[1:]
}

// Title returns the item's title.
func (it *Item) Title() string { return it.str("title") }

// Text returns the item's text.
func (it *Item) Text() string { return it.str("text") }

// Status returns the item's written status, or "" when it has none.
func (it *Item) Status() string { return it.str("status") }

func (it *Item) str(field string) string {
	s, _ := it.Node.Get(field).Str()
	return s
}

// Waived tells whether the item or one of its ancestors is waived.
func (it *Item) Waived() bool {
	for ; it != nil; it = it.Parent {
		if it.Status() == "waived" {
			return true
		}
	}
	return false
}

var statuses = []string{"draft", "approved", "implemented", "verified"}

// EffectiveStatus returns the lowest of a requirement's own status and the
// effective statuses of the requirements nearest below it, through topics,
// that are not waived. A waived requirement, and every requirement below
// one, is effectively waived. A status outside the order is returned as
// written.
func (it *Item) EffectiveStatus() string {
	if it.Waived() {
		return "waived"
	}
	low := slices.Index(statuses, it.Status())
	if low < 0 {
		return it.Status()
	}
	for _, c := range it.Below(it.Kind) {
		if c.Waived() {
			continue
		}
		if r := slices.Index(statuses, c.EffectiveStatus()); r >= 0 && r < low {
			low = r
		}
	}
	return statuses[low]
}

// Below returns the items of a kind nearest below it: its children of that
// kind, and those below the topics among its children, through topics.
func (it *Item) Below(kind byte) []*Item {
	var out []*Item
	for _, c := range it.Children {
		switch c.Kind {
		case kind:
			out = append(out, c)
		case 'T':
			out = append(out, c.Below(kind)...)
		}
	}
	return out
}

// Above returns the nearest item above it of its own kind, through topics,
// or nil. It is the item a child inherits from.
func (it *Item) Above() *Item {
	for p := it.Parent; p != nil; p = p.Parent {
		switch {
		case p.Kind == it.Kind:
			return p
		case p.Kind != 'T':
			return nil
		}
	}
	return nil
}

// Inherited returns a field as the item sets it, or else as the items above
// it of its own kind set it, nearest first; nil when none does. The caller
// names only fields the item's kind inherits.
func (it *Item) Inherited(field string) *tree.Node {
	for x := it; x != nil; x = x.Above() {
		if n := x.Node.Get(field); n != nil {
			return n
		}
	}
	return nil
}

// Class returns the stakeholder class a need sits under: the nearest one
// above it, through needs and topics, or nil.
func (it *Item) Class() *Item {
	for p := it.Parent; p != nil; p = p.Parent {
		switch p.Kind {
		case 'S':
			return p
		case 'N', 'T':
		default:
			return nil
		}
	}
	return nil
}

// Refs returns the ids the item writes, field by field in the order the
// format defines them. The word mission in a need's serves names no item.
func (it *Item) Refs() []Ref {
	var out []Ref
	add := func(field string, n *tree.Node) {
		if id, ok := n.Str(); ok {
			out = append(out, Ref{From: it, File: it.File, Field: field, ID: id, Node: n})
		}
	}
	list := func(field string) {
		if l := it.Node.Get(field); l != nil && l.Kind == tree.Seq {
			for _, n := range l.Items {
				if s, _ := n.Str(); it.Kind == 'N' && field == "serves" && s == "mission" {
					continue
				}
				add(field, n)
			}
		}
	}
	if it.Kind == 'N' {
		list("stakeholders")
	}
	if it.Kind == 'N' || it.Kind == 'R' {
		list("serves")
	}
	if it.Kind != 'T' {
		add("mirrors", it.Node.Get("mirrors"))
	}
	return out
}

// Retired is one entry of a retired list.
type Retired struct {
	ID    string
	Title string
	File  *File
	Line  int
	// Owner is the item whose list holds the entry, or nil for a file's list.
	Owner *Item
}

// Ref is one id named in a field.
type Ref struct {
	// From is the item that names the id, or nil for the ranking.
	From  *Item
	File  *File
	Field string
	ID    string
	Node  *tree.Node
}

// Set is one set's files and the items they hold.
type Set struct {
	// Files are the mission file first, then the chapters in path order.
	Files []*File
	// Items are every item with an id, in reading order, duplicates included.
	Items []*Item
	// Retired are every retired entry with an id, in reading order,
	// duplicates included.
	Retired []*Retired

	byPath  map[string]*Item
	retired map[string]*Retired
	named   map[string][]Ref
}

// New reads files into a set and takes them over: it marks each mission file
// and fills each file's items and retired entries, so a file belongs to one
// set only. Items and entries without an id are left out, an item's children
// with it.
func New(files []*File) *Set {
	s := &Set{byPath: map[string]*Item{}, retired: map[string]*Retired{}, named: map[string][]Ref{}}
	s.Files = slices.Clone(files)
	for _, f := range s.Files {
		f.Mission = path.Base(f.Path) == set.MissionName
	}
	slices.SortStableFunc(s.Files, func(a, b *File) int {
		if a.Mission != b.Mission {
			if a.Mission {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Path, b.Path)
	})
	for _, f := range s.Files {
		if f.Tree == nil {
			continue
		}
		if list := f.Tree.Get("items"); list != nil && list.Kind == tree.Seq {
			for _, n := range list.Items {
				if it := s.collectItem(f, n, nil); it != nil {
					f.Items = append(f.Items, it)
				}
			}
		}
		f.Retired = s.collectRetired(f, f.Tree.Get("retired"), nil)
	}
	for _, it := range s.Items {
		if _, gone := s.retired[it.Path()]; gone {
			continue
		}
		if _, dup := s.byPath[it.Path()]; !dup {
			s.byPath[it.Path()] = it
		}
	}
	for _, it := range s.Items {
		for _, r := range it.Refs() {
			s.named[r.ID] = append(s.named[r.ID], r)
		}
	}
	return s
}

func (s *Set) collectItem(f *File, n *tree.Node, parent *Item) *Item {
	id, ok := n.Get("id").Str()
	if !ok || id == "" {
		return nil
	}
	it := &Item{Kind: id[0], ID: id, File: f, Node: n, Parent: parent}
	s.Items = append(s.Items, it)
	if children := n.Get("children"); children != nil && children.Kind == tree.Seq {
		for _, ch := range children.Items {
			if c := s.collectItem(f, ch, it); c != nil {
				it.Children = append(it.Children, c)
			}
		}
	}
	it.Retired = s.collectRetired(f, n.Get("retired"), it)
	return it
}

func (s *Set) collectRetired(f *File, list *tree.Node, owner *Item) []*Retired {
	if list == nil || list.Kind != tree.Seq {
		return nil
	}
	var out []*Retired
	for _, n := range list.Items {
		id, ok := n.Get("id").Str()
		if !ok || id == "" {
			continue
		}
		title, _ := n.Get("title").Str()
		e := &Retired{ID: id, Title: title, File: f, Line: n.Line, Owner: owner}
		out = append(out, e)
		s.Retired = append(s.Retired, e)
		if _, dup := s.retired[Path(id)]; !dup {
			s.retired[Path(id)] = e
		}
	}
	return out
}

// Item returns the item with id, or nil. Of two items with one path, the
// first read counts; a path that is retired names no item.
func (s *Set) Item(id string) *Item {
	if it := s.byPath[Path(id)]; it != nil && it.ID == id {
		return it
	}
	return nil
}

// AtPath returns the item with the path of id, whatever its letter, or nil.
func (s *Set) AtPath(id string) *Item { return s.byPath[Path(id)] }

// RetiredEntry returns the first entry that retires the path of id itself,
// under any letter, or nil.
func (s *Set) RetiredEntry(id string) *Retired { return s.retired[Path(id)] }

// RetiredWith returns the entry that retires the path of id, itself or with
// an ancestor, or nil.
func (s *Set) RetiredWith(id string) *Retired {
	p := Path(id)
	for {
		if e, ok := s.retired[p]; ok {
			return e
		}
		i := strings.LastIndex(p, ".")
		if i < 0 {
			return nil
		}
		p = p[:i]
	}
}

// NamedBy returns every reference to id from an item, in reading order.
func (s *Set) NamedBy(id string) []Ref { return s.named[id] }

// Ranking returns the objective ids the mission file ranks, highest first.
func (s *Set) Ranking() []Ref {
	var out []Ref
	for _, f := range s.Files {
		if !f.Mission || f.Tree == nil {
			continue
		}
		list := f.Tree.Get("ranking")
		if list == nil || list.Kind != tree.Seq {
			continue
		}
		for _, n := range list.Items {
			if id, ok := n.Str(); ok {
				out = append(out, Ref{File: f, Field: "ranking", ID: id, Node: n})
			}
		}
	}
	return out
}
