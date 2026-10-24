// Command shall checks a product's mission, needs and requirements, compares
// them between git refs, and serves them as documents, exports them as a
// static copy, or dumps them as one JSON document.
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
	"strings"
	"syscall"

	"shall/internal/check"
	"shall/internal/diff"
	"shall/internal/finding"
	"shall/internal/serve"
)

const usage = `usage: shall check [-json] [-warnings-as-errors] [directory]
       shall diff [-json] [-base ref] [-head ref] [directory]
       shall serve [-addr host:port] [directory]
       shall export -out dir [-base path] [directory]
       shall dump [directory]`

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
		case "export":
			return runExport(args[1:], stderr)
		case "dump":
			return runDump(args[1:], stdout, stderr)
		}
		fmt.Fprintf(stderr, "shall: there is no command %q\n", args[0])
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

// directory returns the one directory argument, the current one by default,
// once it is known to be a directory; otherwise it says why it is not.
func directory(flags *flag.FlagSet, stderr io.Writer) (string, bool) {
	dir := "."
	switch flags.NArg() {
	case 0:
	case 1:
		dir = flags.Arg(0)
	default:
		fmt.Fprintf(stderr, "shall: %s takes one directory, after its flags, not %s\n", flags.Name(), strings.Join(flags.Args(), " "))
		fmt.Fprintln(stderr, usage)
		return "", false
	}
	switch st, err := os.Stat(dir); {
	case err != nil:
		fmt.Fprintln(stderr, "shall:", err)
		return "", false
	case !st.IsDir():
		fmt.Fprintf(stderr, "shall: %s is not a directory\n", dir)
		return "", false
	}
	return dir, true
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
		return 2
	}
	fmt.Fprintf(stdout, "shall serves %s at http://%s/\n", root, openable(ln.Addr()))
	if err := serve.Run(ctx, root, ln); err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
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
	results, findings, err := check.Sets(root)
	if err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	if len(results) == 0 {
		// There is no set to check; the findings say why.
		for _, f := range findings {
			fmt.Fprintln(stderr, f)
		}
		return 2
	}
	for _, r := range results {
		findings = append(findings, r.Findings...)
	}
	finding.Sort(findings)
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

func runExport(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "the directory to write the copy into, new or empty")
	base := flags.String("base", "/", "the path a host serves the copy under")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := directory(flags, stderr)
	if !ok {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(stderr, "shall: export needs -out, the directory to write the copy into")
		return 2
	}
	if err := serve.Export(root, *out, *base); err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	return 0
}

func runDump(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dump", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := directory(flags, stderr)
	if !ok {
		return 2
	}
	if err := serve.Dump(root, stdout); err != nil {
		fmt.Fprintln(stderr, "shall:", err)
		return 2
	}
	return 0
}
