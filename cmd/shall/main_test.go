package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

const mission = `mission: m
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S1, serves: [mission], status: hypothesis}
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
		"a.shall.yaml":       "title: A\nrequirements: [\n",
		"b.shall.yaml":       "title: B\nrequirements:\n  - {id: R1, title: X, text: no promise, serves: [N1], status: draft}\n",
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
		"c.shall.yaml":       "title: C\nrequirements:\n  - id: R1\n    title: X\n    text: The tool SHALL work.\n    serves: [N1]\n    status: done\n",
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
		"c.shall.yaml":       "title: C\nrequirements:\n  - {id: R1, title: X, text: The tool SHALL work., serves: [N1], status: done}\n",
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
		"c.shall.yaml":       "title: C\nrequirements:\n  - {id: R1, title: X, text: Tool SHALL work., serves: [N1], status: draft}\n",
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
	if dir, err := exec.Command(goTool, "list", "-m", "-f", "{{.Dir}}", "github.com/goccy/go-yaml").Output(); err != nil || strings.TrimSpace(string(dir)) == "" {
		t.Skip("the module cache lacks goccy/go-yaml; build shall once first")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
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
	testfile.WriteFile(t, filepath.Join(dir, "c.shall.yaml"), "title: C\nrequirements:\n  - {id: R1, title: A, text: no promise, serves: [N1], status: draft}\n")
	if out, err := in(git, "add", "."); err != nil {
		t.Fatal(err, out)
	}
	if out, err := in(preCommit, "try-repo", repo, "shall-check", "--all-files"); err == nil || !strings.Contains(out, "states no SHALL") {
		t.Errorf("the hook passed a set with an error: %v\n%s", err, out)
	}
}
