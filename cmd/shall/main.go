// Command shall checks a product's mission, needs and requirements.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"shall/internal/check"
	"shall/internal/finding"
)

const usage = `usage: shall check [-json] [-warnings-as-errors] [directory]`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print the findings as one JSON document")
	strict := flags.Bool("warnings-as-errors", false, "fail on warnings too")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	findings, err := check.Run(root)
	if err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	nErrors, nWarnings := 0, 0
	for _, f := range findings {
		if f.Severity == finding.Error {
			nErrors++
		} else {
			nWarnings++
		}
	}
	if *asJSON {
		writeJSON(stdout, findings, nErrors, nWarnings)
	} else {
		writeText(stdout, findings, nErrors, nWarnings)
	}
	if nErrors > 0 || (*strict && nWarnings > 0) {
		return 1
	}
	return 0
}

func writeJSON(w io.Writer, findings []finding.Finding, nErrors, nWarnings int) {
	if findings == nil {
		findings = []finding.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(struct {
		Findings []finding.Finding `json:"findings"`
		Errors   int               `json:"errors"`
		Warnings int               `json:"warnings"`
	}{findings, nErrors, nWarnings})
}

func writeText(w io.Writer, findings []finding.Finding, nErrors, nWarnings int) {
	for _, f := range findings {
		where := f.File
		if f.Line > 0 {
			where = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		subject := strings.TrimSpace(f.ID + " " + f.Field)
		if subject != "" {
			fmt.Fprintf(w, "%s: %s: %s: %s\n", where, f.Severity, subject, f.Message)
		} else {
			fmt.Fprintf(w, "%s: %s: %s\n", where, f.Severity, f.Message)
		}
	}
	fmt.Fprintf(w, "%d errors, %d warnings\n", nErrors, nWarnings)
}
