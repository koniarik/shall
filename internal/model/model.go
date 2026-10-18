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

// Title returns a chapter's title, or "" when it has none.
func (f *File) Title() string {
	s, _ := f.Tree.Get("title").Str()
	return s
}

// Item is one objective, stakeholder, need or requirement.
type Item struct {
	// Kind is the item's prefix: 'O', 'S', 'N' or 'R'.
	Kind     byte
	ID       string
	File     *File
	Node     *tree.Node
	Parent   *Item
	Children []*Item
	// Retired are the entries of the item's own retired list.
	Retired []*Retired
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
// effective statuses of its children that are not waived. A waived
// requirement, and every child of one, is effectively waived. A status
// outside the order is returned as written.
func (it *Item) EffectiveStatus() string {
	if it.Waived() {
		return "waived"
	}
	low := slices.Index(statuses, it.Status())
	if low < 0 {
		return it.Status()
	}
	for _, c := range it.Children {
		if c.Waived() {
			continue
		}
		if r := slices.Index(statuses, c.EffectiveStatus()); r >= 0 && r < low {
			low = r
		}
	}
	return statuses[low]
}

// Refs returns the ids the item names, field by field in the order the
// format defines them. The word mission in a need's serves names no item.
func (it *Item) Refs() []Ref {
	var out []Ref
	add := func(field string, n *tree.Node) {
		if id, ok := n.Str(); ok {
			out = append(out, Ref{From: it, File: it.File, Field: field, ID: id, Node: n})
		}
	}
	if it.Kind == 'N' {
		add("stakeholder", it.Node.Get("stakeholder"))
	}
	if it.Kind == 'N' || it.Kind == 'R' {
		if list := it.Node.Get("serves"); list != nil && list.Kind == tree.Seq {
			for _, n := range list.Items {
				if s, _ := n.Str(); it.Kind == 'N' && s == "mission" {
					continue
				}
				add("serves", n)
			}
		}
	}
	add("mirrors", it.Node.Get("mirrors"))
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
	// Holds lists the kinds of item the list retires.
	Holds string
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

	byID    map[string]*Item
	retired map[string]*Retired
	named   map[string][]Ref
}

// New reads files into a set and takes them over: it marks each mission file
// and fills each file's items and retired entries, so a file belongs to one
// set only. Items and entries without an id are left out, an item's children
// with it.
func New(files []*File) *Set {
	s := &Set{byID: map[string]*Item{}, retired: map[string]*Retired{}, named: map[string][]Ref{}}
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
		if f.Mission {
			s.collect(f, "objectives", 'O')
			s.collect(f, "stakeholders", 'S')
			s.collect(f, "needs", 'N')
			f.Retired = s.collectRetired(f, f.Tree.Get("retired"), nil, "OSN")
		} else {
			s.collect(f, "requirements", 'R')
			f.Retired = s.collectRetired(f, f.Tree.Get("retired"), nil, "R")
		}
	}
	for _, it := range s.Items {
		if _, gone := s.retired[it.ID]; gone {
			continue
		}
		if _, dup := s.byID[it.ID]; !dup {
			s.byID[it.ID] = it
		}
	}
	for _, it := range s.Items {
		for _, r := range it.Refs() {
			s.named[r.ID] = append(s.named[r.ID], r)
		}
	}
	return s
}

func (s *Set) collect(f *File, key string, kind byte) {
	list := f.Tree.Get(key)
	if list == nil || list.Kind != tree.Seq {
		return
	}
	for _, n := range list.Items {
		if it := s.collectItem(f, n, kind, nil); it != nil {
			f.Items = append(f.Items, it)
		}
	}
}

func (s *Set) collectItem(f *File, n *tree.Node, kind byte, parent *Item) *Item {
	id, ok := n.Get("id").Str()
	if !ok || id == "" {
		return nil
	}
	it := &Item{Kind: kind, ID: id, File: f, Node: n, Parent: parent}
	s.Items = append(s.Items, it)
	if children := n.Get("children"); children != nil && children.Kind == tree.Seq {
		for _, ch := range children.Items {
			if c := s.collectItem(f, ch, kind, it); c != nil {
				it.Children = append(it.Children, c)
			}
		}
	}
	it.Retired = s.collectRetired(f, n.Get("retired"), it, string(kind))
	return it
}

func (s *Set) collectRetired(f *File, list *tree.Node, owner *Item, holds string) []*Retired {
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
		e := &Retired{ID: id, Title: title, File: f, Line: n.Line, Owner: owner, Holds: holds}
		out = append(out, e)
		s.Retired = append(s.Retired, e)
		if _, dup := s.retired[id]; !dup {
			s.retired[id] = e
		}
	}
	return out
}

// Item returns the item with id, or nil. Of two items with one id, the first
// read counts; an id that is retired names no item.
func (s *Set) Item(id string) *Item { return s.byID[id] }

// RetiredEntry returns the first entry that retires id itself, or nil.
func (s *Set) RetiredEntry(id string) *Retired { return s.retired[id] }

// RetiredWith returns the entry that retires id, itself or with an ancestor,
// or nil.
func (s *Set) RetiredWith(id string) *Retired {
	for {
		if e, ok := s.retired[id]; ok {
			return e
		}
		i := strings.LastIndex(id, ".")
		if i < 0 {
			return nil
		}
		id = id[:i]
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
