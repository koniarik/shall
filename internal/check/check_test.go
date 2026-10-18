package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shall/internal/finding"
	"shall/internal/testfile"
)

const mission = `mission: m
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S1, serves: [mission], status: hypothesis}
`

func run(t *testing.T, root string) []finding.Finding {
	t.Helper()
	got, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func expect(t *testing.T, got []finding.Finding, file, fragment string) {
	t.Helper()
	for _, f := range got {
		if f.File == file && strings.Contains(f.Message, fragment) {
			return
		}
	}
	t.Errorf("no finding in %s containing %q; got %+v", file, fragment, got)
}

func TestTheManifestPathLocatesTheFiles(t *testing.T) {
	t.Attr("verifies", "R1")
	got := run(t, testfile.Write(t, map[string]string{
		"shall.yaml":                       "name: x\nformat: 1\npath: doc/req\n",
		"doc/req/mission.shall.yaml":       mission,
		"doc/req/sub/chapter.shall.yaml":   "title: C\nrequirements:\n  - {id: R1, title: A, text: no promise, serves: [N1], status: draft}\n",
		"elsewhere/ignored.shall.yaml":     "not: [valid\n",
		"doc/req/sub/notes-not-a-set.yaml": "anything: goes\n",
	}))
	expect(t, got, "doc/req/sub/chapter.shall.yaml", "states no SHALL")
	for _, f := range got {
		if strings.HasPrefix(f.File, "elsewhere") {
			t.Errorf("a file outside the manifest's path was checked: %+v", f)
		}
	}
}

func TestAMissingPathIsAnError(t *testing.T) {
	expect(t, run(t, testfile.Write(t, map[string]string{"shall.yaml": "name: x\nformat: 1\npath: nowhere\n"})),
		"shall.yaml", "nowhere is not a directory")
}

func TestUnreadableFilesDoNotStopTheCheck(t *testing.T) {
	t.Attr("verifies", "R2.2")
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	root := testfile.Write(t, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: A\nrequirements: []\n",
		"b.shall.yaml":       "title: B\nrequirements:\n  - {id: R1, title: A, text: no promise, serves: [N1], status: draft}\n",
	})
	locked := filepath.Join(root, "a.shall.yaml")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	got := run(t, root)
	expect(t, got, "a.shall.yaml", "cannot open: permission denied")
	expect(t, got, "b.shall.yaml", "states no SHALL")
	for _, f := range got {
		if strings.Contains(f.Message, root) {
			t.Errorf("message names the absolute path: %+v", f)
		}
	}
}

func TestEachSetIsCheckedOnItsOwn(t *testing.T) {
	t.Attr("verifies", "R1")
	got := run(t, testfile.Write(t, map[string]string{
		"a/shall.yaml":         "name: a\nformat: 1\n",
		"a/mission.shall.yaml": mission,
		"b/shall.yaml":         "name: b\nformat: 1\n",
		"b/mission.shall.yaml": mission,
	}))
	if len(got) != 0 {
		t.Fatalf("findings = %+v, want none: two sets may use the same ids", got)
	}
}

func TestFormatOnePointZeroIsRead(t *testing.T) {
	t.Attr("verifies", "R2.1")
	if got := run(t, testfile.Write(t, map[string]string{"shall.yaml": "name: x\nformat: 1.0\n", "mission.shall.yaml": mission})); len(got) != 0 {
		t.Fatalf("findings = %+v, want none", got)
	}
}

func TestNoSetUnderTheDirectory(t *testing.T) {
	t.Attr("verifies", "R1")
	expect(t, run(t, testfile.Write(t, map[string]string{"notes.txt": "nothing here"})), ".", "no shall.yaml under this directory")
}

func TestAMissingDirectoryFailsTheRun(t *testing.T) {
	if _, err := Run(filepath.Join(t.TempDir(), "nowhere")); err == nil {
		t.Error("checking a directory that does not exist did not fail")
	}
	if _, _, err := Sets(filepath.Join(t.TempDir(), "nowhere")); err == nil {
		t.Error("Sets on a directory that does not exist did not fail")
	}
}

func TestFilesThatAreNoYAML(t *testing.T) {
	t.Attr("verifies", "R2.2")
	t.Attr("verifies", "R3")
	got := run(t, testfile.Write(t, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: A\nrequirements:\n  - [unclosed\n",
		"b/shall.yaml":       "name: [unclosed\n",
	}))
	for _, file := range []string{"a.shall.yaml", "b/shall.yaml"} {
		found := false
		for _, f := range got {
			if f.File == file && f.Line > 0 && f.Severity == finding.Error {
				found = true
			}
		}
		if !found {
			t.Errorf("no error with a line for %s; got %+v", file, got)
		}
	}
}

func TestAnUnknownFormatStopsThatSet(t *testing.T) {
	t.Attr("verifies", "R2.1")
	got := run(t, testfile.Write(t, map[string]string{
		"shall.yaml":         "name: x\nformat: 2\n",
		"a.shall.yaml":       "not: [valid\n",
		"mission.shall.yaml": mission,
	}))
	expect(t, got, "shall.yaml", "format version 2 is not one this checker reads; it reads 1")
	if len(got) != 1 {
		t.Errorf("the set's files were checked anyway: %+v", got)
	}
}

func TestUnreadableDirectoriesDoNotStopTheCheck(t *testing.T) {
	t.Attr("verifies", "R2.2")
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	root := testfile.Write(t, map[string]string{
		"shall.yaml":               "name: x\nformat: 1\n",
		"mission.shall.yaml":       mission,
		"locked/inside.shall.yaml": "title: A\nrequirements: []\n",
		"open/chapter.shall.yaml":  "title: B\nrequirements:\n  - {id: R1, title: A, text: no promise, serves: [N1], status: draft}\n",
	})
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	got := run(t, root)
	expect(t, got, "locked", "permission denied")
	expect(t, got, "open/chapter.shall.yaml", "states no SHALL")
}

func TestSetsComeApart(t *testing.T) {
	t.Attr("verifies", "R1")
	root := testfile.Write(t, map[string]string{
		"a/shall.yaml":             "name: alpha\nformat: 1\npath: req\n",
		"a/req/mission.shall.yaml": mission,
		"b/shall.yaml":             "name: beta\nformat: 1\n",
		"b/mission.shall.yaml":     mission,
		"c/shall.yaml":             "name: [broken\n",
	})
	results, outside, err := Sets(root)
	if err != nil || len(outside) != 0 || len(results) != 3 {
		t.Fatalf("Sets = %d results, %+v, %v; want 3 results, nothing outside, no error", len(results), outside, err)
	}
	a, b, c := results[0], results[1], results[2]
	if a.Name != "alpha" || a.Manifest != "a/shall.yaml" || a.Dir != filepath.Join(root, "a", "req") || a.Set == nil || a.Tree == nil {
		t.Errorf("alpha = %+v, want name alpha, manifest a/shall.yaml, files in a/req, read", a)
	}
	if b.Name != "beta" || len(b.Set.Files) != 1 || len(b.Findings) != 0 {
		t.Errorf("beta = %+v, want name beta with one file and no findings", b)
	}
	if c.Name != "" || c.Set != nil || c.Tree != nil || len(c.Findings) != 1 {
		t.Errorf("a set whose manifest is no YAML = %+v, want no name, set or tree, and one finding", c)
	}
}
