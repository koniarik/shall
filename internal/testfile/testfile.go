// Package testfile writes the files a test reads, in a directory of the
// test's own.
package testfile

import (
	"os"
	"path/filepath"
	"testing"
)

// Write writes files, each a path with slashes and its content, under a new
// temporary directory of t's, and returns that directory.
func Write(t testing.TB, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		WriteFile(t, filepath.Join(root, filepath.FromSlash(name)), src)
	}
	return root
}

// WriteFile writes src to path, making the directories it needs.
func WriteFile(t testing.TB, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}
