package shall

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheSourceIsFormattedAsGofmtFormatsIt(t *testing.T) {
	t.Attr("verifies", "R24.1")
	var files int
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// third_party holds other projects' code, as they wrote it.
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") && path != "." || path == "third_party") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		files++
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted, err := format.Source(src)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		if !bytes.Equal(src, formatted) {
			t.Errorf("%s is not formatted as gofmt formats it", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Error("no Go file was found, so the check proves nothing")
	}
}

func TestTheSourcePassesGoVet(t *testing.T) {
	t.Attr("verifies", "R24.2")
	if testing.Short() {
		t.Skip("runs go vet over the module")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on the path")
	}
	if out, err := exec.Command(goTool, "vet", "./...").CombinedOutput(); err != nil {
		t.Errorf("go vet: %v\n%s", err, out)
	}
}
