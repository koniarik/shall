package set

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func touch(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func rel(t *testing.T, root string, paths []string) []string {
	t.Helper()
	var out []string
	for _, p := range paths {
		r, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(r))
	}
	slices.Sort(out)
	return out
}

func TestManifestsAreFoundBelowTheDirectory(t *testing.T) {
	t.Attr("verifies", "R1")
	root := t.TempDir()
	touch(t, root, "shall.yaml", "a/b/shall.yaml", ".git/shall.yaml", "a/shall.yml", "a/x.shall.yaml")
	found, unreadable, err := Manifests(root)
	if err != nil || len(unreadable) != 0 {
		t.Fatalf("err = %v, unreadable = %v", err, unreadable)
	}
	if got := rel(t, root, found); !slices.Equal(got, []string{"a/b/shall.yaml", "shall.yaml"}) {
		t.Fatalf("manifests = %v", got)
	}
}

func TestFilesEndInTheSuffix(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "mission.shall.yaml", "c/one.shall.yaml", "shall.yaml", "notes.yaml")
	found, _, err := Files(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := rel(t, root, found); !slices.Equal(got, []string{"c/one.shall.yaml", "mission.shall.yaml"}) {
		t.Fatalf("files = %v", got)
	}
}

func TestUnreadableDirectoriesDoNotStopTheWalk(t *testing.T) {
	t.Attr("verifies", "R2.2")
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	root := t.TempDir()
	touch(t, root, "a/shall.yaml", "locked/shall.yaml")
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	found, unreadable, err := Manifests(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := rel(t, root, found); !slices.Equal(got, []string{"a/shall.yaml"}) {
		t.Fatalf("manifests = %v", got)
	}
	if len(unreadable) != 1 || unreadable[0].Path != locked {
		t.Fatalf("unreadable = %v, want %s", unreadable, locked)
	}
}

func TestAMissingRootIsAnError(t *testing.T) {
	if _, _, err := Manifests(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("no error for a missing directory")
	}
}
