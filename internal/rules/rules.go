// Package rules checks the rules of format 1 that hold within one set.
package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"shall/internal/finding"
	"shall/internal/set"
	"shall/internal/tree"
)

// File is one parsed file of a set, its path as findings name it.
type File struct {
	Path string
	Tree *tree.Node
}

type item struct {
	kind   byte
	id     string
	file   string
	node   *tree.Node
	parent *item
}

type retiredEntry struct {
	id    string
	file  string
	line  int
	owner *item
	kinds string
}

type checker struct {
	out         []finding.Finding
	items       []*item
	byID        map[string]*item
	retired     map[string]*retiredEntry
	retiredList []*retiredEntry
	depends     map[string]bool
}

var (
	shallWord = regexp.MustCompile(`\bSHALL\b`)
	otherWord = regexp.MustCompile(`\b(MUST|REQUIRED|SHOULD|RECOMMENDED|MAY|OPTIONAL)\b`)
	ears      = regexp.MustCompile(`^(The |When |While |Where |If .*, then )`)
	childNo   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Check returns the findings of the within-set rules for a set whose manifest
// is at manifestPath and whose files are files.
func Check(manifestPath string, manifest *tree.Node, files []File) []finding.Finding {
	c := &checker{byID: map[string]*item{}, retired: map[string]*retiredEntry{}, depends: map[string]bool{}}
	c.checkDepends(manifestPath, manifest)
	c.checkMissionFile(manifestPath, files)
	for _, f := range files {
		if filepath.Base(f.Path) == set.MissionName {
			c.collect(f, "objectives", 'O')
			c.collect(f, "stakeholders", 'S')
			c.collect(f, "needs", 'N')
			c.collectRetired(f.Path, f.Tree.Get("retired"), nil, "OSN")
		} else {
			c.collect(f, "requirements", 'R')
			c.collectRetired(f.Path, f.Tree.Get("retired"), nil, "R")
		}
	}
	c.checkIDs()
	for _, f := range files {
		if filepath.Base(f.Path) == set.MissionName {
			c.checkRanking(f)
		}
	}
	for _, it := range c.items {
		c.checkItem(it)
	}
	return c.out
}

func (c *checker) add(sev finding.Severity, file string, line int, id, field, format string, args ...any) {
	c.out = append(c.out, finding.Finding{
		Severity: sev, File: file, Line: line, ID: id, Field: field,
		Message: fmt.Sprintf(format, args...),
	})
}

func (c *checker) checkDepends(manifestPath string, manifest *tree.Node) {
	deps := manifest.Get("depends")
	if deps == nil || deps.Kind != tree.Map {
		return
	}
	for _, p := range deps.Pairs {
		c.depends[p.Key] = true
		c.add(finding.Error, manifestPath, p.KeyLine, "", "depends",
			"set %q cannot be read, and the mirror rules need it; this checker does not yet read other sets", p.Key)
	}
}

func (c *checker) checkMissionFile(manifestPath string, files []File) {
	var missions []string
	for _, f := range files {
		if filepath.Base(f.Path) == set.MissionName {
			missions = append(missions, f.Path)
		}
	}
	if len(missions) == 0 {
		c.add(finding.Error, manifestPath, 1, "", "", "the set has no %s", set.MissionName)
	}
	for _, m := range missions[min(1, len(missions)):] {
		c.add(finding.Error, m, 1, "", "", "the set already has %s at %s; it has exactly one", set.MissionName, missions[0])
	}
}

func (c *checker) collect(f File, key string, kind byte) {
	list := f.Tree.Get(key)
	if list == nil || list.Kind != tree.Seq {
		return
	}
	for _, n := range list.Items {
		c.collectItem(f.Path, n, kind, nil)
	}
}

func (c *checker) collectItem(file string, n *tree.Node, kind byte, parent *item) {
	id, ok := n.Get("id").Str()
	if !ok || id == "" {
		return
	}
	it := &item{kind: kind, id: id, file: file, node: n, parent: parent}
	c.items = append(c.items, it)
	if children := n.Get("children"); children != nil && children.Kind == tree.Seq {
		for _, ch := range children.Items {
			c.collectItem(file, ch, kind, it)
		}
	}
	c.collectRetired(file, n.Get("retired"), it, string(kind))
}

func (c *checker) collectRetired(file string, list *tree.Node, owner *item, kinds string) {
	if list == nil || list.Kind != tree.Seq {
		return
	}
	for _, n := range list.Items {
		id, ok := n.Get("id").Str()
		if !ok || id == "" {
			continue
		}
		e := &retiredEntry{id: id, file: file, line: n.Line, owner: owner, kinds: kinds}
		if prev, dup := c.retired[id]; dup {
			c.add(finding.Error, file, n.Line, id, "retired", "%s is already retired at %s:%d", id, prev.file, prev.line)
			continue
		}
		c.retired[id] = e
		c.retiredList = append(c.retiredList, e)
	}
}

func (c *checker) checkIDs() {
	for _, it := range c.items {
		line := it.node.Get("id").Line
		if it.parent == nil && strings.Contains(it.id, ".") {
			c.add(finding.Error, it.file, line, it.id, "id", "a top-level id has no \".\"")
		}
		if it.parent != nil && !isChildOf(it.id, it.parent.id) {
			c.add(finding.Error, it.file, line, it.id, "id", "a child of %s has the id %s.<number>", it.parent.id, it.parent.id)
		}
		if e, ok := c.retired[it.id]; ok {
			c.add(finding.Error, it.file, line, it.id, "id", "%s is retired at %s:%d and cannot return", it.id, e.file, e.line)
			continue
		}
		if prev, ok := c.byID[it.id]; ok {
			c.add(finding.Error, it.file, line, it.id, "id", "%s is already used at %s:%d", it.id, prev.file, prev.node.Line)
			continue
		}
		c.byID[it.id] = it
	}
	for _, e := range c.retiredList {
		switch {
		case e.owner == nil && (strings.Contains(e.id, ".") || !strings.ContainsRune(e.kinds, rune(e.id[0]))):
			c.add(finding.Error, e.file, e.line, e.id, "retired", "%s cannot be retired here; this list holds top-level ids of kinds %s", e.id, e.kinds)
		case e.owner != nil && !isChildOf(e.id, e.owner.id):
			c.add(finding.Error, e.file, e.line, e.id, "retired", "%s cannot be retired under %s, which only retires its own children", e.id, e.owner.id)
		}
	}
}

// retiredWith returns the entry that retires id, itself or with an ancestor,
// or nil.
func (c *checker) retiredWith(id string) *retiredEntry {
	for {
		if e, ok := c.retired[id]; ok {
			return e
		}
		i := strings.LastIndex(id, ".")
		if i < 0 {
			return nil
		}
		id = id[:i]
	}
}

func isChildOf(id, parent string) bool {
	rest, ok := strings.CutPrefix(id, parent+".")
	return ok && childNo.MatchString(rest)
}

func (c *checker) ref(file string, n *tree.Node, from, field string, kinds string) {
	id, ok := n.Str()
	if !ok {
		return
	}
	if strings.Contains(id, "/") {
		name, _, _ := strings.Cut(id, "/")
		if !c.depends[name] {
			c.add(finding.Error, file, n.Line, from, field, "%s names set %q, which the manifest does not depend on", id, name)
		}
		return
	}
	if e := c.retiredWith(id); e != nil {
		c.add(finding.Warning, file, n.Line, from, field, "%s is retired, at %s:%d", id, e.file, e.line)
		return
	}
	target, ok := c.byID[id]
	if !ok {
		c.add(finding.Error, file, n.Line, from, field, "%s is no item of this set", id)
		return
	}
	if !strings.ContainsRune(kinds, rune(target.kind)) {
		c.add(finding.Error, file, n.Line, from, field, "%s cannot be named in %s", id, field)
	}
}

func (c *checker) refs(it *item, field, kinds string) {
	list := it.node.Get(field)
	if list == nil || list.Kind != tree.Seq {
		return
	}
	for _, n := range list.Items {
		if s, _ := n.Str(); it.kind == 'N' && s == "mission" {
			continue
		}
		c.ref(it.file, n, it.id, field, kinds)
	}
}

func (c *checker) checkRanking(f File) {
	list := f.Tree.Get("ranking")
	if list == nil || list.Kind != tree.Seq {
		return
	}
	for _, n := range list.Items {
		c.ref(f.Path, n, "", "ranking", "O")
	}
}

func (c *checker) checkItem(it *item) {
	if m := it.node.Get("mirrors"); m != nil {
		c.ref(it.file, m, it.id, "mirrors", "")
	}
	switch it.kind {
	case 'N':
		if s := it.node.Get("stakeholder"); s != nil {
			c.ref(it.file, s, it.id, "stakeholder", "S")
		}
		c.refs(it, "serves", "O")
	case 'R':
		c.refs(it, "serves", "NR")
		c.checkRequirement(it)
	}
}

func (c *checker) checkRequirement(it *item) {
	if it.parent == nil && it.node.Get("serves") == nil {
		c.add(finding.Warning, it.file, it.node.Line, it.id, "serves", "the requirement serves nothing")
	}
	if it.parent != nil && it.node.Get("status") == nil && !waived(it.parent) {
		c.add(finding.Error, it.file, it.node.Line, it.id, "status", "field \"status\" is missing")
	}
	textNode := it.node.Get("text")
	text, ok := textNode.Str()
	if !ok {
		return
	}
	switch n := len(shallWord.FindAllString(text, -1)); {
	case n == 0:
		c.add(finding.Error, it.file, textNode.Line, it.id, "text", "the text states no SHALL; a requirement makes one promise")
	case n > 1:
		c.add(finding.Error, it.file, textNode.Line, it.id, "text", "the text says SHALL %d times; a requirement makes one promise", n)
	}
	if m := otherWord.FindString(text); m != "" {
		c.add(finding.Error, it.file, textNode.Line, it.id, "text", "the text uses %s; a requirement uses only SHALL and SHALL NOT", m)
	}
	if !ears.MatchString(text) {
		c.add(finding.Warning, it.file, textNode.Line, it.id, "text",
			"the text is in none of the sentence forms: The …, When …, While …, Where …, If …, then …")
	}
}

func waived(it *item) bool {
	for ; it != nil; it = it.parent {
		if s, _ := it.node.Get("status").Str(); s == "waived" {
			return true
		}
	}
	return false
}
