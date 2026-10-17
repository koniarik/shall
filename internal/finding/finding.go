// Package finding holds what a check reports.
package finding

import (
	"cmp"
	"slices"
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

// Sort orders findings by file, then line.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
}
