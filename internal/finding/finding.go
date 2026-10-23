// Package finding holds what a check reports.
package finding

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Severity tells an error from a warning.
type Severity string

// The severities.
const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Finding is one problem in a set's files.
type Finding struct {
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	ID       string   `json:"id,omitempty"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message"`
}

// String returns the finding as one line of text: where it is, its
// severity, the item and field it concerns, and its message.
func (f Finding) String() string {
	where := f.File
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	if subject := strings.TrimSpace(f.ID + " " + f.Field); subject != "" {
		return fmt.Sprintf("%s: %s: %s: %s", where, f.Severity, subject, f.Message)
	}
	return fmt.Sprintf("%s: %s: %s", where, f.Severity, f.Message)
}

// Sort orders findings by file, then line.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
}
