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
	"N stakeholder": "S",
	"N serves":      "O",
	"R serves":      "NR",
}

// Check returns the findings of the within-set rules for the set s, whose
// manifest is at manifestPath.
func Check(manifestPath string, manifest *tree.Node, s *model.Set) []finding.Finding {
	c := &checker{set: s, depends: map[string]bool{}}
	c.checkDepends(manifestPath, manifest)
	c.checkMissionFile(manifestPath)
	c.checkRetired()
	c.checkIDs()
	for _, r := range s.Ranking() {
		c.ref(r, "O")
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
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s is already retired at %s:%d", e.ID, first.File.Path, first.Line)
			continue
		}
		switch {
		case e.Owner == nil && (strings.Contains(e.ID, ".") || !strings.ContainsRune(e.Holds, rune(e.ID[0]))):
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s cannot be retired here; this list holds top-level ids of kinds %s", e.ID, e.Holds)
		case e.Owner != nil && !isChildOf(e.ID, e.Owner.ID):
			c.add(finding.Error, e.File.Path, e.Line, e.ID, "retired", "%s cannot be retired under %s, which only retires its own children", e.ID, e.Owner.ID)
		}
	}
}

func (c *checker) checkIDs() {
	for _, it := range c.set.Items {
		line := it.Node.Get("id").Line
		if it.Parent == nil && strings.Contains(it.ID, ".") {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "a top-level id has no \".\"")
		}
		if it.Parent != nil && !isChildOf(it.ID, it.Parent.ID) {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "a child of %s has the id %s.<number>", it.Parent.ID, it.Parent.ID)
		}
		if e := c.set.RetiredEntry(it.ID); e != nil {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "%s is retired at %s:%d and cannot return", it.ID, e.File.Path, e.Line)
			continue
		}
		if first := c.set.Item(it.ID); first != it {
			c.add(finding.Error, it.File.Path, line, it.ID, "id", "%s is already used at %s:%d", it.ID, first.File.Path, first.Node.Line)
		}
	}
}

func isChildOf(id, parent string) bool {
	rest, ok := strings.CutPrefix(id, parent+".")
	return ok && childNo.MatchString(rest)
}

func (c *checker) ref(r model.Ref, kinds string) {
	from := ""
	if r.From != nil {
		from = r.From.ID
	}
	file, line, id := r.File.Path, r.Node.Line, r.ID
	if strings.Contains(id, "/") {
		name, _, _ := strings.Cut(id, "/")
		if !c.depends[name] {
			c.add(finding.Error, file, line, from, r.Field, "%s names set %q, which the manifest does not depend on", id, name)
		}
		return
	}
	if e := c.set.RetiredWith(id); e != nil {
		c.add(finding.Warning, file, line, from, r.Field, "%s is retired, at %s:%d", id, e.File.Path, e.Line)
		return
	}
	target := c.set.Item(id)
	if target == nil {
		c.add(finding.Error, file, line, from, r.Field, "%s is no item of this set", id)
		return
	}
	if !strings.ContainsRune(kinds, rune(target.Kind)) {
		c.add(finding.Error, file, line, from, r.Field, "%s cannot be named in %s", id, r.Field)
	}
}

func (c *checker) checkItem(it *model.Item) {
	for _, r := range it.Refs() {
		c.ref(r, refKinds[string(it.Kind)+" "+r.Field])
	}
	if it.Kind == 'R' {
		c.checkRequirement(it)
	}
}

func (c *checker) checkRequirement(it *model.Item) {
	file := it.File.Path
	if it.Parent == nil && it.Node.Get("serves") == nil {
		c.add(finding.Warning, file, it.Node.Line, it.ID, "serves", "the requirement serves nothing")
	}
	if it.Parent != nil && it.Node.Get("status") == nil && !it.Parent.Waived() {
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
