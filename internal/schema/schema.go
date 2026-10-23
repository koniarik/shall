// Package schema checks a YAML tree against the subset of JSON Schema that the
// format's schema files use.
package schema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"shall/internal/finding"
	"shall/internal/tree"
)

// Validator holds the schema files, by file name.
type Validator struct {
	files    map[string]map[string]any
	patterns map[string]*regexp.Regexp
}

var types = []string{"object", "array", "string", "integer", "boolean"}

// Load reads every *.schema.json file at the root of fsys. It refuses a schema
// that uses a keyword, or a form of a keyword, this validator does not
// implement, and one whose references do not resolve.
func Load(fsys fs.FS) (*Validator, error) {
	names, err := fs.Glob(fsys, "*.schema.json")
	if err != nil {
		return nil, err
	}
	v := &Validator{files: map[string]map[string]any{}, patterns: map[string]*regexp.Regexp{}}
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		v.files[name] = s
	}
	for _, name := range slices.Sorted(maps.Keys(v.files)) {
		if err := v.checkSchema(v.files[name], name, name); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (v *Validator) checkSchema(s map[string]any, file, where string) error {
	for _, k := range slices.Sorted(maps.Keys(s)) {
		val := s[k]
		bad := func() error { return fmt.Errorf("%s: %s has a form this validator does not implement", where, k) }
		switch k {
		case "$schema", "$id", "title":
		case "$ref":
			ref, ok := val.(string)
			if !ok {
				return bad()
			}
			if target, _ := v.resolve(ref, file); target == nil {
				return fmt.Errorf("%s: reference %s does not resolve", where, ref)
			}
		case "type":
			if t, ok := val.(string); !ok || !slices.Contains(types, t) {
				return bad()
			}
		case "properties", "$defs":
			m, ok := val.(map[string]any)
			if !ok {
				return bad()
			}
			for _, name := range slices.Sorted(maps.Keys(m)) {
				sub, ok := m[name].(map[string]any)
				if !ok {
					return bad()
				}
				if err := v.checkSchema(sub, file, where+"/"+k+"/"+name); err != nil {
					return err
				}
			}
		case "items", "propertyNames", "if", "then", "else":
			sub, ok := val.(map[string]any)
			if !ok {
				return bad()
			}
			if err := v.checkSchema(sub, file, where+"/"+k); err != nil {
				return err
			}
		case "additionalProperties", "unevaluatedProperties":
			switch sub := val.(type) {
			case bool:
			case map[string]any:
				if err := v.checkSchema(sub, file, where+"/"+k); err != nil {
					return err
				}
			default:
				return bad()
			}
		case "anyOf":
			list, ok := val.([]any)
			if !ok {
				return bad()
			}
			for i, b := range list {
				sub, ok := b.(map[string]any)
				if !ok {
					return bad()
				}
				if err := v.checkSchema(sub, file, fmt.Sprintf("%s/anyOf/%d", where, i)); err != nil {
					return err
				}
			}
		case "required":
			if !stringList(val) {
				return bad()
			}
		case "dependentRequired":
			m, ok := val.(map[string]any)
			if !ok {
				return bad()
			}
			for _, need := range m {
				if !stringList(need) {
					return bad()
				}
			}
		case "enum":
			list, ok := val.([]any)
			if !ok || !slices.ContainsFunc(list, isScalar) || slices.ContainsFunc(list, func(e any) bool { return !isScalar(e) }) {
				return bad()
			}
		case "const":
			if !isScalar(val) {
				return bad()
			}
		case "pattern":
			p, ok := val.(string)
			if !ok {
				return bad()
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return fmt.Errorf("%s: pattern: %w", where, err)
			}
			v.patterns[p] = re
		case "minLength", "minItems":
			if _, ok := val.(float64); !ok {
				return bad()
			}
		case "format":
			if val != "uri" {
				return bad()
			}
		default:
			return fmt.Errorf("%s: schema keyword %q is not supported", where, k)
		}
	}
	return nil
}

func stringList(val any) bool {
	list, ok := val.([]any)
	return ok && !slices.ContainsFunc(list, func(e any) bool { _, isStr := e.(string); return !isStr })
}

func isScalar(val any) bool {
	switch val.(type) {
	case string, float64, bool:
		return true
	}
	return false
}

// Check validates n, read from path, against the schema file named schema. A
// schema missing from the validator is a programming error.
func (v *Validator) Check(schema, path string, n *tree.Node) []finding.Finding {
	s, ok := v.files[schema]
	if !ok {
		panic(fmt.Sprintf("schema %s is not loaded", schema))
	}
	var out []finding.Finding
	v.validate(s, schema, n, scope{file: path, line: n.Line}, &out)
	return out
}

type scope struct {
	file  string
	line  int
	id    string
	field string
}

func (c scope) report(line int, format string, args ...any) finding.Finding {
	if line == 0 {
		line = c.line
	}
	return finding.Finding{
		Severity: finding.Error, File: c.file, Line: line, ID: c.id, Field: c.field,
		Message: fmt.Sprintf(format, args...),
	}
}

func (c scope) subject() string {
	if c.field == "" {
		return "the file"
	}
	return fmt.Sprintf("field %q", c.field)
}

func (v *Validator) resolve(ref, base string) (map[string]any, string) {
	file, pointer, _ := strings.Cut(ref, "#")
	if file == "" {
		file = base
	} else {
		file = path.Join(path.Dir(base), file)
	}
	var cur any = v.files[file]
	for _, seg := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if seg == "" {
			continue
		}
		m, _ := cur.(map[string]any)
		cur = m[seg]
	}
	s, _ := cur.(map[string]any)
	return s, file
}

// validate checks n against s and returns the property names s evaluated.
func (v *Validator) validate(s map[string]any, base string, n *tree.Node, c scope, out *[]finding.Finding) map[string]bool {
	evaluated := map[string]bool{}
	if ref, ok := s["$ref"].(string); ok {
		target, tb := v.resolve(ref, base)
		for k := range v.validate(target, tb, n, c, out) {
			evaluated[k] = true
		}
	}
	if t, ok := s["type"].(string); ok && !hasType(n, t) {
		*out = append(*out, c.report(n.Line, "%s should be %s", c.subject(), typeName(t)))
		return evaluated
	}
	if want, ok := s["const"]; ok && !equal(n, want) {
		*out = append(*out, c.report(n.Line, "%s must be %v, not %s", c.subject(), want, n.Text()))
	}
	if list, ok := s["enum"].([]any); ok && !slices.ContainsFunc(list, func(e any) bool { return equal(n, e) }) {
		*out = append(*out, c.report(n.Line, "%s is not one of %s", n.Text(), join(list)))
	}
	if str, ok := n.Str(); ok {
		v.checkString(s, str, n, c, out)
	}
	if n.Kind == tree.Seq {
		if limit, ok := s["minItems"].(float64); ok && len(n.Items) < int(limit) {
			*out = append(*out, c.report(n.Line, "%s needs at least %d entries", c.subject(), int(limit)))
		}
		if items, ok := s["items"].(map[string]any); ok {
			for _, it := range n.Items {
				item := c
				item.line = it.Line
				v.validate(items, base, it, item, out)
			}
		}
	}
	if n.Kind == tree.Map {
		v.checkMap(s, base, n, c, out, evaluated)
	}
	if branches, ok := s["anyOf"].([]any); ok && !v.anyOf(branches, base, n, c, evaluated) {
		*out = append(*out, c.report(n.Line, "%s is not valid in %s", n.Text(), c.subject()))
	}
	if cond, ok := s["if"].(map[string]any); ok {
		v.ifThenElse(s, cond, base, n, c, out, evaluated)
	}
	return evaluated
}

// ifThenElse checks n against then where it holds to cond, and against else
// where it does not; cond itself reports nothing.
func (v *Validator) ifThenElse(s, cond map[string]any, base string, n *tree.Node, c scope, out *[]finding.Finding, evaluated map[string]bool) {
	var trial []finding.Finding
	ev := v.validate(cond, base, n, c, &trial)
	branch := "else"
	if len(trial) == 0 {
		branch = "then"
		for k := range ev {
			evaluated[k] = true
		}
	}
	if b, ok := s[branch].(map[string]any); ok {
		for k := range v.validate(b, base, n, c, out) {
			evaluated[k] = true
		}
	}
}

func (v *Validator) anyOf(branches []any, base string, n *tree.Node, c scope, evaluated map[string]bool) bool {
	for _, b := range branches {
		var trial []finding.Finding
		ev := v.validate(b.(map[string]any), base, n, c, &trial)
		if len(trial) == 0 {
			for k := range ev {
				evaluated[k] = true
			}
			return true
		}
	}
	return false
}

func (v *Validator) checkString(s map[string]any, str string, n *tree.Node, c scope, out *[]finding.Finding) {
	if p, ok := s["pattern"].(string); ok && !v.patterns[p].MatchString(str) {
		*out = append(*out, c.report(n.Line, "%s is not a valid value for %s", n.Text(), c.subject()))
	}
	if limit, ok := s["minLength"].(float64); ok && utf8.RuneCountInString(str) < int(limit) {
		*out = append(*out, c.report(n.Line, "%s needs at least %d characters", c.subject(), int(limit)))
	}
	if s["format"] == "uri" {
		if u, err := url.Parse(str); err != nil || u.Scheme == "" {
			*out = append(*out, c.report(n.Line, "%s is not a URI", n.Text()))
		}
	}
}

func (v *Validator) checkMap(s map[string]any, base string, n *tree.Node, c scope, out *[]finding.Finding, evaluated map[string]bool) {
	inner := scope{file: c.file, line: n.Line, id: c.id}
	if id, ok := n.Get("id").Str(); ok {
		inner.id = id
	}
	at := func(field string, line int) scope {
		f := inner
		f.field = field
		f.line = line
		return f
	}
	if req, ok := s["required"].([]any); ok {
		for _, r := range req {
			name := r.(string)
			if n.Pair(name) == nil {
				*out = append(*out, at(name, n.Line).report(n.Line, "field %q is missing", name))
			}
		}
	}
	props, _ := s["properties"].(map[string]any)
	for _, p := range n.Pairs {
		if sub, ok := props[p.Key].(map[string]any); ok {
			evaluated[p.Key] = true
			v.validate(sub, base, p.Value, at(p.Key, p.KeyLine), out)
		}
	}
	if deps, ok := s["dependentRequired"].(map[string]any); ok {
		for _, key := range slices.Sorted(maps.Keys(deps)) {
			if n.Pair(key) == nil {
				continue
			}
			for _, r := range deps[key].([]any) {
				if name := r.(string); n.Pair(name) == nil {
					*out = append(*out, at(name, n.Line).report(n.Line, "field %q is required with %q", name, key))
				}
			}
		}
	}
	if names, ok := s["propertyNames"].(map[string]any); ok {
		for _, p := range n.Pairs {
			key := &tree.Node{Kind: tree.Scalar, Line: p.KeyLine, Value: p.Key}
			v.validate(names, base, key, at("name", p.KeyLine), out)
		}
	}
	extra := s["additionalProperties"]
	unevaluated := s["unevaluatedProperties"]
	for _, p := range n.Pairs {
		if props[p.Key] != nil {
			continue
		}
		switch sub := extra.(type) {
		case map[string]any:
			evaluated[p.Key] = true
			v.validate(sub, base, p.Value, at(p.Key, p.KeyLine), out)
		case bool:
			if !sub {
				*out = append(*out, at(p.Key, p.KeyLine).report(p.KeyLine, "field %q is not defined here", p.Key))
			}
		case nil:
			if unevaluated == false && !evaluated[p.Key] {
				*out = append(*out, at(p.Key, p.KeyLine).report(p.KeyLine, "field %q is not defined here", p.Key))
			}
		}
	}
}

func hasType(n *tree.Node, t string) bool {
	switch t {
	case "object":
		return n.Kind == tree.Map
	case "array":
		return n.Kind == tree.Seq
	case "string":
		_, ok := n.Str()
		return ok
	case "integer":
		_, ok := n.Value.(int64)
		return n.Kind == tree.Scalar && ok
	case "boolean":
		_, ok := n.Value.(bool)
		return n.Kind == tree.Scalar && ok
	}
	return false
}

func typeName(t string) string {
	switch t {
	case "object":
		return "a mapping"
	case "array":
		return "a list"
	case "string":
		return "text"
	case "integer":
		return "a whole number"
	}
	return "true or false"
}

func equal(n *tree.Node, want any) bool {
	if w, ok := want.(float64); ok {
		got, isNumber := n.Number()
		return isNumber && got == w
	}
	return n != nil && n.Kind == tree.Scalar && n.Value == want
}

func join(list []any) string {
	parts := make([]string, 0, len(list))
	for _, e := range list {
		parts = append(parts, fmt.Sprint(e))
	}
	return strings.Join(parts, ", ")
}
