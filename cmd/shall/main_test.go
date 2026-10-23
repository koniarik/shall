package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shall/internal/serve"
	"shall/internal/testfile"
)

const manifest = "name: x\nformat: 1\n"

const mission = `title: M
mission: m
items:
  - id: S9
    title: A
    text: t
    children:
      - {id: N9.1, title: A, text: t, serves: [mission], status: hypothesis}
`

func shall(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errs bytes.Buffer
	code := run(t.Context(), args, &out, &errs)
	return code, out.String() + errs.String()
}

type report struct {
	Findings []struct {
		Severity string `json:"severity"`
		File     string `json:"file"`
		Line     int    `json:"line"`
		ID       string `json:"id"`
		Field    string `json:"field"`
		Message  string `json:"message"`
	} `json:"findings"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

func jsonReport(t *testing.T, dir string) (int, report) {
	t.Helper()
	code, out := shall(t, "check", "-json", dir)
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	return code, r
}

func TestFindsEverySetUnderTheDirectory(t *testing.T) {
	t.Attr("verifies", "R1")
	dir := testfile.Write(t, map[string]string{
		"a/shall.yaml":         manifest,
		"a/mission.shall.yaml": mission,
		"b/c/shall.yaml":       manifest,
	})
	_, r := jsonReport(t, dir)
	if r.Errors != 1 || r.Findings[0].File != "b/c/shall.yaml" {
		t.Fatalf("report = %+v, want one error: b/c has no mission file", r)
	}
}

func TestNoSetIsAnError(t *testing.T) {
	t.Attr("verifies", "R1")
	code, out := shall(t, "check", testfile.Write(t, map[string]string{"x.txt": ""}))
	if code != 1 || !strings.Contains(out, "no shall.yaml") {
		t.Fatalf("exit %d, output %q; want 1 and that there is no shall.yaml", code, out)
	}
}

func TestUnknownFormatIsRefused(t *testing.T) {
	t.Attr("verifies", "R2.1")
	_, r := jsonReport(t, testfile.Write(t, map[string]string{"shall.yaml": "name: x\nformat: 7\n"}))
	if r.Errors != 1 || !strings.Contains(r.Findings[0].Message, "format version 7") {
		t.Fatalf("report = %+v, want one error naming format version 7", r)
	}
}

func TestCheckingGoesOnPastErrors(t *testing.T) {
	t.Attr("verifies", "R2.2")
	_, r := jsonReport(t, testfile.Write(t, map[string]string{
		"shall.yaml":         manifest,
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: A\nitems: [\n",
		"b.shall.yaml":       "title: B\nitems:\n  - {id: R1, title: X, text: no promise, serves: [N9.1], status: draft}\n",
	}))
	files := map[string]bool{}
	for _, f := range r.Findings {
		files[f.File] = true
	}
	if !files["a.shall.yaml"] || !files["b.shall.yaml"] {
		t.Fatalf("findings = %+v, want findings in both files", r.Findings)
	}
}

func TestFindingsLocateThemselves(t *testing.T) {
	t.Attr("verifies", "R3")
	_, r := jsonReport(t, testfile.Write(t, map[string]string{
		"shall.yaml":         manifest,
		"mission.shall.yaml": mission,
		"c.shall.yaml":       "title: C\nitems:\n  - id: R1\n    title: X\n    text: The tool SHALL work.\n    serves: [N9.1]\n    status: done\n",
	}))
	if len(r.Findings) == 0 {
		t.Fatal("no findings")
	}
	f := r.Findings[0]
	if f.File != "c.shall.yaml" || f.Line != 7 || f.ID != "R1" || f.Field != "status" {
		t.Fatalf("finding = %+v, want c.shall.yaml:7 R1 status", f)
	}
}

func TestFindingsAsText(t *testing.T) {
	t.Attr("verifies", "R3")
	_, out := shall(t, "check", testfile.Write(t, map[string]string{
		"shall.yaml":         manifest,
		"mission.shall.yaml": mission,
		"c.shall.yaml":       "title: C\nitems:\n  - {id: R1, title: X, text: The tool SHALL work., serves: [N9.1], status: done}\n",
	}))
	want := "c.shall.yaml:3: error: R1 status: \"done\" is not one of draft, approved, implemented, verified, waived\n1 errors, 0 warnings\n"
	if out != want {
		t.Fatalf("output =\n%s\nwant\n%s", out, want)
	}
}

func TestFindingsAsJSON(t *testing.T) {
	t.Attr("verifies", "R4")
	code, r := jsonReport(t, testfile.Write(t, map[string]string{"shall.yaml": manifest, "mission.shall.yaml": mission}))
	if code != 0 || r.Errors != 0 || r.Findings == nil {
		t.Fatalf("exit %d, report %+v; want a passing, empty report", code, r)
	}
}

func TestErrorsFailTheRun(t *testing.T) {
	t.Attr("verifies", "R5")
	code, _ := shall(t, "check", testfile.Write(t, map[string]string{"shall.yaml": manifest}))
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}

func TestWarningsFailOnlyWhenAsked(t *testing.T) {
	t.Attr("verifies", "R5.1")
	t.Attr("verifies", "R5.2")
	dir := testfile.Write(t, map[string]string{
		"shall.yaml":         manifest,
		"mission.shall.yaml": mission,
		"c.shall.yaml":       "title: C\nitems:\n  - {id: R1, title: X, text: Tool SHALL work., serves: [N9.1], status: draft}\n",
	})
	if code, out := shall(t, "check", dir); code != 0 || !strings.Contains(out, "0 errors, 1 warnings") {
		t.Fatalf("exit %d, output %q; want a pass with one warning", code, out)
	}
	if code, _ := shall(t, "check", "-warnings-as-errors", dir); code != 1 {
		t.Fatalf("exit %d with -warnings-as-errors, want 1", code)
	}
}

func TestShallsOwnSetHasNoErrors(t *testing.T) {
	t.Attr("verifies", "R2")
	code, r := jsonReport(t, "../..")
	for _, f := range r.Findings {
		if f.Severity == "error" {
			t.Errorf("%s:%d %s %s: %s", f.File, f.Line, f.ID, f.Field, f.Message)
		}
	}
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
}

func TestUsage(t *testing.T) {
	if code, _ := shall(t); code != 2 {
		t.Fatalf("exit %d without a command, want 2", code)
	}
}

// lockedBuffer is output that a running command and a test share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestServeSaysWhereItServes(t *testing.T) {
	t.Attr("verifies", "R25.1")
	t.Attr("verifies", "R25.3")
	dir := testfile.Write(t, map[string]string{"shall.yaml": manifest, "mission.shall.yaml": mission})
	ctx, cancel := context.WithCancel(t.Context())
	var out lockedBuffer
	done := make(chan int)
	go func() { done <- run(ctx, []string{"serve", "-addr", "127.0.0.1:0", dir}, &out, &out) }()
	var addr string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, rest, ok := strings.Cut(out.String(), " at "); ok {
			addr = strings.TrimSpace(rest)
			break
		}
	}
	if !strings.HasPrefix(addr, "http://127.0.0.1:") {
		cancel()
		t.Fatalf("output = %q, want the address it serves on", out.String())
	}
	resp, err := http.Get(addr + "api/sets")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"name": "x"`) {
		t.Errorf("sets = %s, want set x", body)
	}
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("exit %d after being stopped, want 0", code)
	}
}

func TestServeIsLocalByDefault(t *testing.T) {
	t.Attr("verifies", "R25.2")
	host, _, err := net.SplitHostPort(serve.DefaultAddr)
	if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
		t.Errorf("default address %s is not on this machine only", serve.DefaultAddr)
	}
}

func TestServeRefusesATakenAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if code, out := shall(t, "serve", "-addr", ln.Addr().String(), t.TempDir()); code != 1 || !strings.Contains(out, "shall:") {
		t.Errorf("exit %d %q, want 1 with the reason", code, out)
	}
}

func TestWrongUseIsRefused(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"nonsense"},
		{"check", "-no-such-flag"},
		{"check", dir, dir},
		{"serve", "-no-such-flag"},
		{"serve", dir, dir},
	} {
		if code, out := shall(t, args...); code != 2 || !strings.Contains(out, "usage") && !strings.Contains(out, "flag provided but not defined") {
			t.Errorf("shall %v = exit %d %q, want 2 with the usage", args, code, out)
		}
	}
}

func TestCheckingADirectoryThatIsNotThere(t *testing.T) {
	if code, out := shall(t, "check", filepath.Join(t.TempDir(), "nowhere")); code != 2 || !strings.Contains(out, "shall:") {
		t.Errorf("exit %d %q, want 2 with the reason", code, out)
	}
}

func TestAnAddressOnEveryInterfaceIsOpenedOnLocalhost(t *testing.T) {
	t.Attr("verifies", "R25.3")
	for _, c := range []struct {
		addr net.Addr
		want string
	}{
		{&net.TCPAddr{IP: net.IPv6unspecified, Port: 7180}, "localhost:7180"},
		{&net.TCPAddr{IP: net.IPv4zero, Port: 7180}, "localhost:7180"},
		{&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7180}, "127.0.0.1:7180"},
		{&net.UnixAddr{Name: "/tmp/x", Net: "unix"}, "/tmp/x"},
	} {
		if got := openable(c.addr); got != c.want {
			t.Errorf("openable(%v) = %s, want %s", c.addr, got, c.want)
		}
	}
}

func TestTheCurrentDirectoryByDefault(t *testing.T) {
	t.Attr("verifies", "R1")
	t.Chdir(testfile.Write(t, map[string]string{"shall.yaml": manifest, "mission.shall.yaml": mission}))
	if code, out := shall(t, "check"); code != 0 || !strings.Contains(out, "0 errors, 0 warnings") {
		t.Errorf("exit %d %q, want the current directory checked clean", code, out)
	}
}

func TestOneHookToAdopt(t *testing.T) {
	t.Attr("verifies", "R6")
	if testing.Short() {
		t.Skip("builds the hook with pre-commit")
	}
	preCommit, err := exec.LookPath("pre-commit")
	if err != nil {
		t.Skip("pre-commit is not installed")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on the path")
	}
	// The hook builds against this machine's module cache, with downloads
	// off, so the test needs no network; pre-commit and git keep their state
	// in the test's own directory.
	cache, err := exec.Command(goTool, "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	mods, err := exec.Command(goTool, "list", "-m", "-f", "{{if not .Main}}{{.Path}}={{.Dir}}{{end}}", "all").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range strings.Fields(string(mods)) {
		if path, dir, _ := strings.Cut(m, "="); dir == "" {
			t.Skip("the module cache lacks " + path + "; build shall once first")
		}
	}
	home := t.TempDir()
	gitConfig := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[user]\n\tname = Test\n\temail = test@example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GOMODCACHE="+strings.TrimSpace(string(cache)), "GOPROXY=off", "GOFLAGS=",
		"PRE_COMMIT_HOME="+filepath.Join(home, "pre-commit"),
		"GIT_CONFIG_GLOBAL="+gitConfig, "GIT_CONFIG_NOSYSTEM=1")
	repo := snapshot(t, git, env)
	dir := testfile.Write(t, map[string]string{"shall.yaml": manifest, "mission.shall.yaml": mission})
	in := func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := in(git, "init", "-q"); err != nil {
		t.Fatal(err, out)
	}
	if out, err := in(git, "add", "."); err != nil {
		t.Fatal(err, out)
	}
	if out, err := in(preCommit, "try-repo", repo, "shall-check", "--all-files"); err != nil {
		t.Fatalf("the hook failed on a valid set: %v\n%s", err, out)
	}
	testfile.WriteFile(t, filepath.Join(dir, "c.shall.yaml"), "title: C\nitems:\n  - {id: R1, title: A, text: no promise, serves: [N9.1], status: draft}\n")
	if out, err := in(git, "add", "."); err != nil {
		t.Fatal(err, out)
	}
	if out, err := in(preCommit, "try-repo", repo, "shall-check", "--all-files"); err == nil || !strings.Contains(out, "states no SHALL") {
		t.Errorf("the hook passed a set with an error: %v\n%s", err, out)
	}
}

func TestDiffExitStatus(t *testing.T) {
	t.Attr("verifies", "R37.1")
	t.Attr("verifies", "R37.4")
	t.Attr("verifies", "R37.5")
	t.Attr("verifies", "R37.6")
	dir := testfile.Repo(t)
	testfile.Commit(t, dir, map[string]string{"shall.yaml": manifest, "mission.shall.yaml": mission, "doc/a.shall.yaml": "title: A\nitems: []\n"})
	if code, out := shall(t, "diff", dir); code != 0 || out != "nothing changed\n" {
		t.Errorf("diff with nothing changed = exit %d %q, want 0 and that nothing changed", code, out)
	}
	testfile.WriteFile(t, filepath.Join(dir, "c.shall.yaml"), "title: C\nitems:\n  - {id: R1, title: One, text: The tool SHALL work., serves: [N9.1], status: draft}\n")
	if code, out := shall(t, "diff", dir); code != 1 || !strings.Contains(out, "R1 One: added") {
		t.Errorf("diff with a new requirement = exit %d %q, want 1 and R1 added", code, out)
	}
	code, out := shall(t, "diff", "-json", dir)
	var r struct{ Totals struct{ Added int } }
	if err := json.Unmarshal([]byte(out), &r); code != 1 || err != nil || r.Totals.Added != 1 {
		t.Errorf("diff -json = exit %d, %v, %q; want 1 and one item added", code, err, out)
	}
	if code, out := shall(t, "diff", "-base", "no-such-ref", dir); code != 2 || !strings.Contains(out, "no-such-ref names no commit") {
		t.Errorf("diff from an unknown ref = exit %d %q, want 2 and why", code, out)
	}
	if code, out := shall(t, "diff", filepath.Join(dir, "doc")); code != 2 || !strings.Contains(out, "no shall.yaml under "+filepath.Join(dir, "doc")+" at either ref") {
		t.Errorf("diff of a directory holding only chapters = exit %d %q, want 2 and that it holds no set", code, out)
	}
	if code, out := shall(t, "diff", t.TempDir()); code != 2 || !strings.Contains(out, "shall:") {
		t.Errorf("diff outside a repository = exit %d %q, want 2 and why", code, out)
	}
	testfile.Commit(t, dir, nil)
	if code, out := shall(t, "diff", "-base", "HEAD~1", "-head", "HEAD", dir); code != 1 || !strings.Contains(out, "R1 One: added") {
		t.Errorf("diff -base HEAD~1 -head HEAD = exit %d %q, want 1 and R1 added", code, out)
	}
	if code, out := shall(t, "diff", "-base", "HEAD", "-head", "HEAD", dir); code != 0 || out != "nothing changed\n" {
		t.Errorf("diff -base HEAD -head HEAD = exit %d %q, want 0 and that nothing changed", code, out)
	}
}

// snapshot copies the files of this repository, as they stand on disk, into
// a new repository of its own and commits them there, so that pre-commit
// builds the hook from the code on disk whatever git tracks here.
func snapshot(t *testing.T, git string, env []string) string {
	t.Helper()
	src, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		testfile.WriteFile(t, filepath.Join(dst, rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "snapshot"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir, cmd.Env = dst, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dst
}
