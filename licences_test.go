package shall

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// embedded names each library embedded under internal/serve/web/static, by
// its directory, and finds its version in the file it ships.
var embedded = map[string]struct {
	name, file string
	version    *regexp.Regexp
}{
	"pure":    {"Pure.css", "pure-min.css", regexp.MustCompile(`Pure v([0-9.]+)`)},
	"mermaid": {"Mermaid", "mermaid.min.js", regexp.MustCompile(`version:"([1-9][0-9]*\.[0-9]+\.[0-9]+)"`)},
}

func TestEveryDependencyHasItsLicenceRecorded(t *testing.T) {
	t.Attr("verifies", "R18.1")
	record, err := os.ReadFile("doc/licences.md")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	inRequire := false
	for _, line := range strings.Split(string(mod), "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 2 && f[0] == "go":
			want["Go standard library"] = f[1]
		case len(f) >= 2 && f[0] == "require" && f[1] == "(":
			inRequire = true
		case inRequire && len(f) > 0 && f[0] == ")":
			inRequire = false
		case inRequire && len(f) >= 2:
			want[f[0]] = f[1]
		case len(f) >= 3 && f[0] == "require":
			want[f[1]] = f[2]
		}
	}
	licences, err := filepath.Glob("internal/serve/web/static/*/LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range licences {
		dir := filepath.Base(filepath.Dir(l))
		e, ok := embedded[dir]
		if !ok {
			t.Errorf("%s ships a licence, but the test knows no library there; name it in embedded", filepath.Dir(l))
			continue
		}
		src, err := os.ReadFile(filepath.Join(filepath.Dir(l), e.file))
		if err != nil {
			t.Fatal(err)
		}
		m := e.version.FindSubmatch(src)
		if m == nil {
			t.Errorf("%s: no version found in %s", e.name, e.file)
			continue
		}
		want[e.name] = string(m[1])
	}
	if len(want) < 5 {
		t.Fatalf("dependencies found = %v, want at least Go, two modules and two embedded libraries", want)
	}
	for name, version := range want {
		found := false
		for _, row := range strings.Split(string(record), "\n") {
			found = found || strings.HasPrefix(row, "| "+name+" | "+version+" | ")
		}
		if !found {
			t.Errorf("doc/licences.md has no row for %s %s, want its licence recorded", name, version)
		}
	}
}
