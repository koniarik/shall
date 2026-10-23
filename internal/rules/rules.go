// Package rules checks the rules of format 1 that hold within one set.
package rules

import (
	"fmt"
	"regexp"
	"strings"

	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/set"
	"shall/internal/tree"
)

type checker struct {
	out     []finding.Finding
	set     *model.Set
	depends map[string]bool
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

// kinds are the letters of the kinds of item.
const kinds = "OSNRT"

// Check returns the findings of the within-set rules for the set s, whose
// manifest is at manifestPath.
func Check(manifestPath string, manifest *tree.Node, s *model.Set) []finding.Finding {
	c := &checker{set: s, depends: map[string]bool{}}
	c.checkDepends(manifestPath, manifest)
	c.checkMissionFile(manifestPath)
	c.checkRetired()
	c.checkIDs()
	for _, r := range s.Ranking() {
		c.ref(r, "ranking")
	}
	for _, it := range s.Items {
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
	if !strings.ContainsRune(kinds, rune(parent)) || !strings.ContainsRune(kinds, rune(child)) {
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
// that writes it.
func (c *checker) ref(r model.Ref, key string) {
	from := ""
	if r.From != nil {
		from = r.From.ID
	}
	file, line, id := r.File.Path, r.Node.Line, r.ID
	if strings.Contains(id, "/") {
		name, _, _ := strings.Cut(id, "/")
		switch {
		case local[key]:
			c.add(finding.Error, file, line, from, r.Field, "%s names another set; %s names items of this set only", id, r.Field)
		case !c.depends[name]:
			c.add(finding.Error, file, line, from, r.Field, "%s names set %q, which the manifest does not depend on", id, name)
		}
		return
	}
	if e := c.set.RetiredWith(id); e != nil {
		if model.Path(e.ID) == model.Path(id) && e.ID != id {
			c.add(finding.Error, file, line, from, r.Field, "%s names the path of %s, retired at %s:%d; the letter does not match", id, e.ID, e.File.Path, e.Line)
			return
		}
		c.add(finding.Warning, file, line, from, r.Field, "%s is retired, at %s:%d", id, e.File.Path, e.Line)
		return
	}
	target := c.set.Item(id)
	if target == nil {
		if other := c.set.AtPath(id); other != nil {
			c.add(finding.Error, file, line, from, r.Field, "%s names the path of %s, at %s:%d; the letter does not match", id, other.ID, other.File.Path, other.Node.Line)
			return
		}
		c.add(finding.Error, file, line, from, r.Field, "%s is no item of this set", id)
		return
	}
	if !strings.ContainsRune(refKinds[key], rune(target.Kind)) {
		c.add(finding.Error, file, line, from, r.Field, "%s cannot be named in %s", id, r.Field)
	}
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
	if it.Above() == nil && it.Node.Get("serves") == nil {
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
