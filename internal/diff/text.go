package diff

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"shall/internal/finding"
)

// WriteText writes a diff for people: each set, its set-wide fields, then
// its files with their items, its findings gained and lost, and the totals
// last. It fails only when git cannot mark the changed words.
func WriteText(w io.Writer, r *Result) error {
	if !r.Changed() {
		fmt.Fprintln(w, "nothing changed")
		return nil
	}
	var t draft
	for _, s := range r.Sets {
		t.line("%s: set %s%s", s.Manifest, s.Name, unless(s.Change, "changed"))
		for _, f := range s.Fields {
			t.change(f.Old, f.New, "  %s: ", f.Field)
		}
		files := map[string]*FileChange{}
		items := map[string][]ItemChange{}
		var paths []string
		for i, f := range s.Files {
			files[f.Path] = &s.Files[i]
			paths = append(paths, f.Path)
		}
		for _, it := range s.Items {
			if items[it.File] == nil && files[it.File] == nil {
				paths = append(paths, it.File)
			}
			items[it.File] = append(items[it.File], it)
		}
		slices.Sort(paths)
		paths = slices.Compact(paths)
		for _, p := range paths {
			switch f := files[p]; {
			case f == nil:
				t.line("  %s", p)
			case f.Change == "retitled":
				t.change(f.Title.Old, f.Title.New, "  %s: chapter retitled ", p)
			case f.Mission:
				t.line("  %s: mission file %s", p, f.Change)
			default:
				t.line("  %s: chapter %s", p, f.Change)
			}
			for _, it := range items[p] {
				note := it.Change
				if it.MovedFrom != "" {
					note += ", moved from " + it.MovedFrom
				}
				t.line("    %s %s: %s", it.ID, it.Title, note)
				for _, f := range it.Fields {
					t.change(f.Old, f.New, "      %s: ", f.Field)
				}
				if e := it.EffectiveStatus; e != nil {
					t.change(e.Old, e.New, "      effective status: ")
				}
			}
		}
		if fs := s.Findings; fs != nil {
			t.line("  findings: %d new, %d gone", len(fs.New), len(fs.Gone))
			for _, f := range fs.New {
				t.line("    new: %s", line(f))
			}
			for _, f := range fs.Gone {
				t.line("    gone: %s", line(f))
			}
		}
	}
	tot := r.Totals
	items := "items"
	if tot.Changed == 1 {
		items = "item"
	}
	t.line("%d %s changed, %d added, %d removed, %d retired, %d moved", tot.Changed, items, tot.Added, tot.Removed, tot.Retired, tot.Moved)
	return t.write(w)
}

// draft is a diff's lines, held until git has marked the changes they end
// with, all at once.
type draft struct {
	lines   []string
	ending  []int // the change each line ends with, or -1
	changes []change
}

func (t *draft) line(format string, args ...any) {
	t.lines = append(t.lines, fmt.Sprintf(format, args...))
	t.ending = append(t.ending, -1)
}

// change adds a line that starts as format says and ends with a change.
func (t *draft) change(old, cur any, format string, args ...any) {
	t.lines = append(t.lines, fmt.Sprintf(format, args...))
	t.ending = append(t.ending, len(t.changes))
	t.changes = append(t.changes, change{old, cur})
}

func (t *draft) write(w io.Writer) error {
	marked, err := mark(t.changes)
	if err != nil {
		return err
	}
	for i, l := range t.lines {
		if c := t.ending[i]; c >= 0 {
			l += marked[c]
		}
		fmt.Fprintln(w, l)
	}
	return nil
}

func unless(change, usual string) string {
	if change == usual {
		return ""
	}
	return ": " + change
}

func line(f finding.Finding) string {
	where := f.File
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	if subject := strings.TrimSpace(f.ID + " " + f.Field); subject != "" {
		return fmt.Sprintf("%s: %s: %s: %s", where, f.Severity, subject, f.Message)
	}
	return fmt.Sprintf("%s: %s: %s", where, f.Severity, f.Message)
}

// render writes a value as written for a line of text: a list in brackets,
// a map in braces with its keys in order.
func render(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case null:
		return "null"
	case string:
		return v
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = render(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + render(v[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(v)
}
