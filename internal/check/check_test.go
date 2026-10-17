package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shall/internal/finding"
)

const mission = `mission: m
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S1, serves: [mission], status: hypothesis}
`

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

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
	got := run(t, write(t, map[string]string{
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
	expect(t, run(t, write(t, map[string]string{"shall.yaml": "name: x\nformat: 1\npath: nowhere\n"})),
		"shall.yaml", "nowhere is not a directory")
}

func TestUnreadableFilesDoNotStopTheCheck(t *testing.T) {
	t.Attr("verifies", "R2.2")
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	root := write(t, map[string]string{
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
	got := run(t, write(t, map[string]string{
		"a/shall.yaml":         "name: a\nformat: 1\n",
		"a/mission.shall.yaml": mission,
		"b/shall.yaml":         "name: b\nformat: 1\n",
		"b/mission.shall.yaml": mission,
	}))
	if len(got) != 0 {
		t.Fatalf("findings = %+v; two sets with the same ids must not collide", got)
	}
}

func TestFormatOnePointZeroIsRead(t *testing.T) {
	t.Attr("verifies", "R2.1")
	if got := run(t, write(t, map[string]string{"shall.yaml": "name: x\nformat: 1.0\n", "mission.shall.yaml": mission})); len(got) != 0 {
		t.Fatalf("findings = %+v, want none", got)
	}
}
