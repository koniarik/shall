// Command shall checks a product's mission, needs and requirements, and
// serves them as documents.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"shall/internal/check"
	"shall/internal/diff"
	"shall/internal/finding"
	"shall/internal/serve"
)

const usage = `usage: shall check [-json] [-warnings-as-errors] [directory]
       shall diff [-json] [-base ref] [-head ref] [directory]
       shall serve [-addr host:port] [directory]`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "check":
			return runCheck(args[1:], stdout, stderr)
		case "diff":
			return runDiff(args[1:], stdout, stderr)
		case "serve":
			return runServe(ctx, args[1:], stdout, stderr)
		}
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

// directory returns the one directory argument, the current one by default.
func directory(flags *flag.FlagSet, stderr io.Writer) (string, bool) {
	switch flags.NArg() {
	case 0:
		return ".", true
	case 1:
		return flags.Arg(0), true
	}
	fmt.Fprintln(stderr, usage)
	return "", false
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print the diff as one JSON document")
	base := flags.String("base", "HEAD", "the ref to compare from")
	head := flags.String("head", "", "the ref to compare to; the working tree when empty")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := directory(flags, stderr)
	if !ok {
		return 2
	}
	r, err := diff.Between(root, *base, *head)
	if err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	} else if err := diff.WriteText(stdout, r); err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	if r.Changed() {
		return 1
	}
	return 0
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", serve.DefaultAddr, "the address and port to listen on")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := directory(flags, stderr)
	if !ok {
		return 2
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 1
	}
	fmt.Fprintf(stdout, "shall serves %s at http://%s/\n", root, openable(ln.Addr()))
	if err := serve.Run(ctx, root, ln); err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 1
	}
	return 0
}

// openable returns an address a browser on this machine can open: a server
// listening on every interface is reached through localhost.
func openable(a net.Addr) string {
	tcp, ok := a.(*net.TCPAddr)
	if !ok || !tcp.IP.IsUnspecified() {
		return a.String()
	}
	return net.JoinHostPort("localhost", strconv.Itoa(tcp.Port))
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print the findings as one JSON document")
	strict := flags.Bool("warnings-as-errors", false, "fail on warnings too")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := directory(flags, stderr)
	if !ok {
		return 2
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
		fmt.Fprintln(w, f)
	}
	fmt.Fprintf(w, "%d errors, %d warnings\n", nErrors, nWarnings)
}
