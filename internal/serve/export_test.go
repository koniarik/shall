package serve

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shall/internal/testfile"
)

// exported writes files under a new directory and exports what it serves
// into another, under base.
func exported(t *testing.T, files map[string]string, base string) (root, out string) {
	t.Helper()
	root = testfile.Write(t, files)
	out = filepath.Join(t.TempDir(), "site")
	if err := Export(root, out, base); err != nil {
		t.Fatal(err)
	}
	return root, out
}

// unversioned returns a JSON answer without its version, as compact JSON.
func unversioned(t *testing.T, data []byte) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%v: %s", err, data)
	}
	delete(v, "version")
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestACopyHoldsEveryAnswerThePageReads(t *testing.T) {
	t.Attr("verifies", "R60")
	t.Attr("verifies", "R60.1")
	root, out := exported(t, coffee(), "/shall/")
	for _, name := range []string{
		"index.html", "404.html", "coffee/index.html", "-/static/app.js", "-/static/mermaid/LICENSE",
		"api/sets.json", "api/sets/coffee.json",
		"api/sets/coffee/files/mission.shall.yaml.json", "api/sets/coffee/files/b-tank.shall.yaml.json",
	} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Error(err)
		}
	}
	for _, id := range []string{"O90", "S91", "N91.1", "S92", "R1", "R1.1", "R2", "R4", "R5", "R1.2", "R3"} {
		for _, name := range []string{"api/sets/coffee/items/" + id + ".json", "coffee/" + id + ".html"} {
			if _, err := os.Stat(filepath.Join(out, name)); err != nil {
				t.Error(err)
			}
		}
	}
	h := New(root, true).Handler()
	answers := 0
	err := filepath.WalkDir(filepath.Join(out, "api"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(out, p)
		if err != nil {
			return err
		}
		addr := "/" + strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, body := getFrom(t, h, addr, "localhost:7180")
		if got, want := unversioned(t, data), unversioned(t, []byte(body)); got != want {
			t.Errorf("%s holds\n%s\nwhere the server answers %s with\n%s", rel, got, addr, want)
		}
		answers++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := 2 + 3 + 11; answers != want {
		t.Errorf("the copy holds %d answers, want %d: the sets, the set, its 3 files and its 11 ids", answers, want)
	}
}

func TestACopyIsServedUnderItsBase(t *testing.T) {
	t.Attr("verifies", "R60.2")
	_, out := exported(t, coffee(), "/shall/")
	for _, name := range []string{"index.html", "404.html", "coffee/index.html", "coffee/R1.html"} {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		page := string(b)
		for _, want := range []string{
			`<meta name="shall-base" content="/shall/">`,
			`<meta name="shall-static" content="">`,
			`<meta http-equiv="Content-Security-Policy" content="` + pagePolicy + `">`,
			`<script src="/shall/-/static/app.js" defer></script>`,
			`<a class="home" href="/shall/">`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("%s does not hold %s", name, want)
			}
		}
		if strings.Contains(page, `"/-/`) || strings.Contains(page, `href="/"`) {
			t.Errorf("%s still points outside /shall/:\n%s", name, page)
		}
	}
}

func TestACopyOfASetOfAnyName(t *testing.T) {
	t.Attr("verifies", "R60")
	_, out := exported(t, with(coffee(), map[string]string{"shall.yaml": "name: Coffee Machine\nformat: 1\n"}), "/")
	for _, name := range []string{"Coffee Machine/index.html", "Coffee Machine/R1.html", "api/sets/Coffee Machine.json", "api/sets/Coffee Machine/items/R1.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Error(err)
		}
	}
	for _, name := range []string{"index.html", "a/b", ".."} {
		root := testfile.Write(t, with(coffee(), map[string]string{"shall.yaml": fmt.Sprintf("name: %q\nformat: 1\n", name)}))
		if err := Export(root, filepath.Join(t.TempDir(), "site"), "/"); err == nil || !strings.Contains(err.Error(), "cannot name a folder") {
			t.Errorf("a set named %q was exported: %v", name, err)
		}
	}
}

func TestACopyIsWrittenOnlyIntoANewOrEmptyDirectory(t *testing.T) {
	t.Attr("verifies", "R60.3")
	root := testfile.Write(t, coffee())
	full := t.TempDir()
	keep := filepath.Join(full, "keep.txt")
	testfile.WriteFile(t, keep, "kept")
	if err := Export(root, full, "/"); err == nil {
		t.Error("Export wrote into a directory that holds a file")
	}
	if entries, _ := os.ReadDir(full); len(entries) != 1 {
		t.Errorf("Export changed the directory it refused: %v", entries)
	}
	if err := Export(root, t.TempDir(), "/"); err != nil {
		t.Errorf("Export refused an empty directory: %v", err)
	}
	for _, base := range []string{"", "shall/", "/shall", "/a b/", `/"/`, "//"} {
		if err := Export(root, filepath.Join(t.TempDir(), "site"), base); err == nil {
			t.Errorf("Export took the base %q", base)
		}
	}
}
