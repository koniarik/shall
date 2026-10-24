// Package rules checks the rules of format 1 for one set, holding its
// mirrors and its references to other sets against the sets it depends on.
package rules

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/set"
	"shall/internal/tree"
)

// Dep is a set the manifest depends on, as the checker could read it.
type Dep struct {
	// Set is the set, or nil when it cannot be read.
	Set *model.Set
	// Why says why the set cannot be read.
	Why string
}

type checker struct {
	out      []finding.Finding
	set      *model.Set
	manifest string
	// depends holds the names the manifest depends on, in its order, and
	// deps the sets they name.
	depends  []string
	depLines map[string]int
	deps     map[string]Dep
}

var (
	shallWord = regexp.MustCompile(`\bSHALL\b`)
	otherWord = regexp.MustCompile(`\b(MUST|REQUIRED|SHOULD|RECOMMENDED|MAY|OPTIONAL)\b`)
	ears      = regexp.MustCompile(`^(The |When |While |Where |If .*, then )`)
	childNo   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// refKinds gives, for each kind of item and field, the kinds of item the
// field can name. A field missing here names only items of other sets.
var refKinds = map[string]string{
	"ranking":        "O",
	"N stakeholders": "S",
	"N serves":       "O",
	"R serves":       "NR",
}

// local are the fields that name items of their own set only.
var local = map[string]bool{"ranking": true, "N stakeholders": true, "N serves": true}

// itemKinds are the letters of the kinds of item.
const itemKinds = "OSNRT"

// Check returns the findings of the rules for the set s, whose manifest is at
// manifestPath, given the sets it depends on by the names the manifest gives
// them.
func Check(manifestPath string, manifest *tree.Node, s *model.Set, deps map[string]Dep) []finding.Finding {
	c := &checker{set: s, manifest: manifestPath, depLines: map[string]int{}, deps: deps}
	c.checkDepends(manifestPath, manifest)
	c.checkMissionFile(manifestPath)
	c.checkRetired()
	c.checkIDs()
	for _, r := range s.Ranking() {
		c.ref(r, "ranking")
	}
	for _, r := range s.MissionCites() {
		c.ref(r, "mission")
	}
	for _, it := range s.Items {
		c.checkItem(it)
	}
	c.checkMirrors()
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
		c.depends = append(c.depends, p.Key)
		c.depLines[p.Key] = p.KeyLine
		if d := c.deps[p.Key]; d.Set == nil {
			c.add(finding.Error, manifestPath, p.KeyLine, "", "depends",
				"set %q cannot be read, and the mirror rules need it: %s", p.Key, cmp.Or(d.Why, "nothing was read for it"))
		}
		if g := p.Value.Get("git"); g != nil {
			if address, ok := g.Str(); ok && set.Credentials(address) {
				c.add(finding.Warning, manifestPath, g.Line, "", "depends",
					"the address of %q carries credentials, which anyone who reads the repository can use; leave them to git, such as through a credential helper", p.Key)
			}
		}
	}
}

func (c *checker) checkMissionFile(manifestPath string) {
	var missions []string
	for _, f := range c.set.Files {
		if f.Mission {
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

func (c *checker) checkRetired() {
	for _, e := range c.set.Retired {
		if first := c.set.RetiredEntry(e.ID); first != e {
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s is already retired at %s:%d%s", e.ID, first.File.Path, first.Line, as(first.ID, e.ID))
			continue
		}
		switch {
		case e.Owner == nil && strings.Contains(e.ID, "."):
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s cannot be retired here; this list holds top-level ids", e.ID)
		case e.Owner != nil && !isChildOf(model.Path(e.ID), e.Owner.Path()):
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s cannot be retired under %s, which only retires its own children", e.ID, e.Owner.ID)
		}
	}
}

// as names the id a path was taken under, where it is not id itself.
func as(taken, id string) string {
	if taken == id {
		return ""
	}
	return ", as " + taken
}

func (c *checker) checkIDs() {
	for _, it := range c.set.Items {
		line := it.Node.Get("id").Line
		if it.Parent == nil && strings.Contains(it.ID, ".") {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "a top-level id has no \".\"")
		}
		if it.Parent != nil && !isChildOf(it.Path(), it.Parent.Path()) {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "a child of %s has the id <letter>%s.<number>", it.Parent.ID, it.Parent.Path())
		}
		if it.Parent != nil && !nests(it.Parent.Kind, it.Kind) {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "%s cannot sit under %s; %s", it.ID, it.Parent.ID, holds(it.Parent.Kind))
		}
		if e := c.set.RetiredEntry(it.ID); e != nil {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "%s cannot return: its path is retired at %s:%d%s", it.ID, e.File.Path, e.Line, as(e.ID, it.ID))
			continue
		}
		if first := c.set.AtPath(it.ID); first != it {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "%s is already used at %s:%d%s; a path names one item, whatever its letter", it.ID, first.File.Path, first.Node.Line, as(first.ID, it.ID))
		}
	}
}

func isChildOf(path, parent string) bool {
	rest, ok := strings.CutPrefix(path, parent+".")
	return ok && childNo.MatchString(rest)
}

// nests tells whether an item of kind child may sit right under one of kind
// parent. A kind the format does not have is reported by the schema.
func nests(parent, child byte) bool {
	if !strings.ContainsRune(itemKinds, rune(parent)) || !strings.ContainsRune(itemKinds, rune(child)) {
		return true
	}
	return parent == child || parent == 'T' || child == 'T' || parent == 'S' && child == 'N'
}

// holds says what an item of a kind may hold.
func holds(kind byte) string {
	if kind == 'S' {
		return "a stakeholder class holds classes, needs and topics"
	}
	return fmt.Sprintf("%s holds only items of its own kind and topics", map[byte]string{'O': "an objective", 'N': "a need", 'R': "a requirement"}[kind])
}

// ref checks one reference; key names the field, after the kind of item
// that writes it. A citation may name an item of any kind, here or in a set
// the manifest depends on, and what it gets wrong is a warning.
func (c *checker) ref(r model.Ref, key string) {
	from := ""
	if r.From != nil {
		from = r.From.ID
	}
	kinds, onlyHere, sev := refKinds[key], local[key], finding.Error
	if r.Citation {
		kinds, onlyHere, sev = itemKinds, false, finding.Warning
	}
	if r.Field == "mirrors" && !r.Citation {
		// A mirror stands for an item of its own kind.
		kinds = string(r.From.Kind)
	}
	file, line, id := r.File.Path, r.Node.Line, r.ID
	name, local, other := strings.Cut(id, "/")
	switch {
	case !other && r.Field == "mirrors" && !r.Citation:
		c.add(sev, file, line, from, r.Field, "%s is an item of this set; mirrors names one of a set the manifest depends on, as name/id", id)
		return
	case !other:
		c.resolve(c.set, "this set", id, r, kinds, sev)
		return
	case onlyHere:
		c.add(sev, file, line, from, r.Field, "%s names another set; %s names items of this set only", id, r.Field)
		return
	case !slices.Contains(c.depends, name):
		c.add(sev, file, line, from, r.Field, "%s names set %q, which the manifest does not depend on", id, name)
		return
	}
	if dep := c.deps[name].Set; dep != nil {
		c.resolve(dep, fmt.Sprintf("set %q", name), local, r, kinds, sev)
	}
}

// resolve checks that id names an item of s of one of kinds, for the
// reference r; in names s in a message.
func (c *checker) resolve(s *model.Set, in, id string, r model.Ref, kinds string, sev finding.Severity) {
	from := ""
	if r.From != nil {
		from = r.From.ID
	}
	file, line, written := r.File.Path, r.Node.Line, r.ID
	if e := s.RetiredWith(id); e != nil {
		if model.Path(e.ID) == model.Path(id) && e.ID != id {
			c.add(sev, file, line, from, r.Field, "%s names the path of %s, retired at %s:%d; the letter does not match", written, e.ID, e.File.Path, e.Line)
			return
		}
		c.add(finding.Warning, file, line, from, r.Field, "%s is retired, at %s:%d", written, e.File.Path, e.Line)
		return
	}
	target := s.Item(id)
	if target == nil {
		if other := s.AtPath(id); other != nil {
			c.add(sev, file, line, from, r.Field, "%s names the path of %s, at %s:%d; the letter does not match", written, other.ID, other.File.Path, other.Node.Line)
			return
		}
		c.add(sev, file, line, from, r.Field, "%s is no item of %s", written, in)
		return
	}
	if !strings.ContainsRune(kinds, rune(target.Kind)) {
		c.add(sev, file, line, from, r.Field, "%s cannot be named in %s", written, r.Field)
	}
}

// checkMirrors holds the set's mirrors against the sets it depends on: every
// item of each, topics aside, has one mirror here, which keeps its title and
// sits under the mirror of the nearest item above it.
func (c *checker) checkMirrors() {
	mirrorOf := map[string]*model.Item{}
	for _, it := range c.set.Items {
		n := it.Node.Get("mirrors")
		target, ok := n.Str()
		name, local, other := strings.Cut(target, "/")
		dep := c.deps[name].Set
		if !ok || !other || dep == nil || c.set.Item(it.ID) != it || !slices.Contains(c.depends, name) {
			continue
		}
		x := dep.Item(local)
		if x == nil || x.Kind != it.Kind || dep.RetiredWith(local) != nil {
			continue // the reference itself is wrong, and ref says so
		}
		if first := mirrorOf[target]; first != nil {
			c.add(finding.Error, it.File.Path, n.Line, it.ID, "mirrors", "%s already has a mirror, %s at %s:%d; an item has one", target, first.ID, first.File.Path, first.Node.Line)
			continue
		}
		mirrorOf[target] = it
		if title := it.Title(); title != x.Title() {
			c.add(finding.Warning, it.File.Path, lineOf(it, "title"), it.ID, "title", "%s is titled %q; its mirror keeps the title", target, x.Title())
		}
		if p := aboveTopics(x); p != nil {
			q := aboveTopics(it)
			want := name + "/" + p.ID
			if got, _ := mirrors(q); got != want {
				c.add(finding.Warning, it.File.Path, n.Line, it.ID, "mirrors", "%s sits under %s, so its mirror sits under the mirror of %s", target, want, want)
			}
		}
	}
	for _, name := range c.depends {
		dep := c.deps[name].Set
		if dep == nil {
			continue
		}
	items:
		for _, x := range dep.Items {
			if x.Kind == 'T' || dep.Item(x.ID) != x || mirrorOf[name+"/"+x.ID] != nil {
				continue
			}
			for a := x.Parent; a != nil; a = a.Parent {
				if m := mirrorOf[name+"/"+a.ID]; m != nil && m.Kind == 'R' && m.Waived() {
					continue items
				}
			}
			c.add(finding.Warning, c.manifest, c.depLines[name], "", "depends", "%s/%s %s has no mirror in this set", name, x.ID, x.Title())
		}
	}
}

// aboveTopics returns the nearest item above it that is not a topic, or nil.
func aboveTopics(it *model.Item) *model.Item {
	for p := it.Parent; p != nil; p = p.Parent {
		if p.Kind != 'T' {
			return p
		}
	}
	return nil
}

// mirrors returns the name/id an item mirrors, if it mirrors one.
func mirrors(it *model.Item) (string, bool) {
	if it == nil {
		return "", false
	}
	return it.Node.Get("mirrors").Str()
}

// lineOf returns the line of an item's field, or of the item.
func lineOf(it *model.Item, field string) int {
	if n := it.Node.Get(field); n != nil {
		return n.Line
	}
	return it.Node.Line
}

func (c *checker) checkItem(it *model.Item) {
	for _, r := range it.Refs() {
		c.ref(r, string(it.Kind)+" "+r.Field)
	}
	file := it.File.Path
	switch it.Kind {
	case 'O':
		if it.Above() == nil && it.Node.Get("measure") == nil {
			c.add(finding.Error, file, it.Node.Line, it.ID, "measure", "field \"measure\" is missing")
		}
	case 'N':
		c.checkNeed(it)
	case 'R':
		c.checkRequirement(it)
	case 'T':
		if it.Node.Get("text") == nil && len(it.Children) == 0 {
			c.add(finding.Warning, file, it.Node.Line, it.ID, "", "the topic has neither text nor children, so it explains and groups nothing")
		}
	}
}

func (c *checker) checkNeed(it *model.Item) {
	file := it.File.Path
	if it.Above() == nil && it.Node.Get("serves") == nil {
		c.add(finding.Error, file, it.Node.Line, it.ID, "serves", "field \"serves\" is missing")
	}
	class := it.Class()
	if class == nil && it.Inherited("stakeholders") == nil {
		c.add(finding.Warning, file, it.Node.Line, it.ID, "stakeholders", "the need has no stakeholder class: it sits under none and names none")
	}
	if class == nil {
		return
	}
	if list := it.Node.Get("stakeholders"); list != nil && list.Kind == tree.Seq {
		for _, n := range list.Items {
			if id, _ := n.Str(); id == class.ID {
				c.add(finding.Warning, file, n.Line, it.ID, "stakeholders", "%s is the class the need sits under; naming it again says it twice", id)
			}
		}
	}
}

func (c *checker) checkRequirement(it *model.Item) {
	file := it.File.Path
	if it.Above() == nil && it.Node.Get("serves") == nil && !it.Waived() {
		c.add(finding.Warning, file, it.Node.Line, it.ID, "serves", "the requirement serves nothing")
	}
	if it.Node.Get("status") == nil && (it.Parent == nil || !it.Parent.Waived()) {
		c.add(finding.Error, file, it.Node.Line, it.ID, "status", "field \"status\" is missing")
	}
	textNode := it.Node.Get("text")
	text, ok := textNode.Str()
	if !ok {
		return
	}
	switch n := len(shallWord.FindAllString(text, -1)); {
	case n == 0:
		c.add(finding.Error, file, textNode.Line, it.ID, "text", "the text states no SHALL; a requirement makes one promise")
	case n > 1:
		c.add(finding.Error, file, textNode.Line, it.ID, "text", "the text says SHALL %d times; a requirement makes one promise", n)
	}
	if m := otherWord.FindString(text); m != "" {
		c.add(finding.Error, file, textNode.Line, it.ID, "text", "the text uses %s; a requirement uses only SHALL and SHALL NOT", m)
	}
	if !ears.MatchString(text) {
		c.add(finding.Warning, file, textNode.Line, it.ID, "text",
			"the text is in none of the sentence forms: The …, When …, While …, Where …, If …, then …")
	}
}
