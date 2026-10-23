// Package testfile writes the files a test reads, in a directory of the
// test's own, and the git repositories that hold them.
package testfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Repo returns a new git repository in a directory of t's, and gives git
// settings of its own for the rest of the test, so the user's are not read.
// It skips the test when git is not installed.
func Repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	config := filepath.Join(t.TempDir(), "gitconfig")
	WriteFile(t, config, "[user]\n\tname = Test\n\temail = test@example.com\n")
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	Git(t, dir, "init", "-q")
	return dir
}

// Git runs git in dir and returns what it printed, trimmed.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Commit writes files into the repository at dir, removes those named in
// gone, commits everything and returns the commit.
func Commit(t testing.TB, dir string, files map[string]string, gone ...string) string {
	t.Helper()
	for name, src := range files {
		WriteFile(t, filepath.Join(dir, filepath.FromSlash(name)), src)
	}
	for _, name := range gone {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	Git(t, dir, "add", "-A")
	Git(t, dir, "commit", "-q", "--allow-empty", "-m", "change")
	return Git(t, dir, "rev-parse", "HEAD")
}
