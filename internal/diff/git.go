package diff

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"shall/internal/check"
	"shall/internal/set"
)

// Read reads the sets under dir as they stand at ref, or as they stand on
// disk when ref is empty, and checks them. A directory that is not there at
// ref holds no set. A ref is read from git's object store, every file in
// its tree that the checker reads, so a set whose path leads elsewhere in
// the repository reads the same at a ref as on disk. A directory missing
// from disk is looked up in the repository from the nearest one still
// there.
func Read(dir, ref string) ([]*check.Result, error) {
	near, missing, err := nearest(dir)
	if err != nil {
		return nil, err
	}
	if ref == "" {
		if missing != "" {
			return nil, nil
		}
		up, err := git(dir, "rev-parse", "--show-cdup")
		if err != nil {
			return nil, err
		}
		results, _, err := check.SetsWithin(dir, filepath.Join(dir, up))
		return results, err
	}
	top, err := git(near, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	prefix, err := git(near, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	if missing != "" {
		prefix += missing + "/"
	}
	commit, said, err := gitSaid(near, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		// git says why on an error line, if at all; its last line is the
		// same for every ref it cannot resolve.
		msg := fmt.Sprintf("%s names no commit of the repository at %s", ref, top)
		for _, l := range strings.Split(said, "\n") {
			if why, ok := strings.CutPrefix(l, "error: "); ok {
				msg += ": " + strings.TrimPrefix(why, ref+"^{commit}: ")
			}
		}
		return nil, errors.New(msg)
	}
	// Every file the checker reads ends in the manifest's name: the
	// manifests, and the set files named *.shall.yaml.
	names, err := git(top, "ls-tree", "-r", "-z", "--name-only", commit)
	if err != nil {
		return nil, err
	}
	var read []string
	for _, name := range strings.Split(names, "\x00") {
		if strings.HasSuffix(name, set.ManifestName) {
			read = append(read, name)
		}
	}
	if len(read) == 0 {
		return nil, nil
	}
	tmp, err := os.MkdirTemp("", "shall-diff-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := extract(top, commit, read, tmp); err != nil {
		return nil, fmt.Errorf("reading %s: %w", ref, err)
	}
	root := filepath.Join(tmp, filepath.FromSlash(prefix))
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	results, _, err := check.SetsWithin(root, tmp)
	return results, err
}

// nearest returns the nearest directory on disk that holds dir, dir itself
// when it is there, and the path from it to dir in slashes, empty when dir
// is there.
func nearest(dir string) (string, string, error) {
	dir = filepath.Clean(dir)
	near := dir
	for {
		st, err := os.Stat(near)
		if err == nil && !st.IsDir() {
			return "", "", fmt.Errorf("%s is not a directory", near)
		}
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) || filepath.Dir(near) == near {
			return "", "", err
		}
		near = filepath.Dir(near)
	}
	missing, err := filepath.Rel(near, dir)
	if err != nil || missing == "." {
		return near, "", err
	}
	return near, filepath.ToSlash(missing), nil
}

// git runs git in dir and returns what it printed, trimmed, or an error
// holding what it said.
func git(dir string, args ...string) (string, error) {
	out, _, err := gitSaid(dir, args...)
	return out, err
}

// gitSaid is git, also returning what git said on its error stream.
func gitSaid(dir string, args ...string) (out, said string, err error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	said = strings.TrimSpace(stderr.String())
	switch {
	case err != nil && said != "":
		return "", said, fmt.Errorf("git %s: %s", args[0], said)
	case err != nil:
		return "", said, fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(stdout)), said, nil
}

// extract writes each named file of commit under dir, its content as
// committed. A link is read through, as on disk, while it leads to a file
// of commit's tree; one that does not is written as a link that fails as
// it fails on disk.
func extract(top, commit string, names []string, dir string) error {
	for _, name := range names {
		if strings.Contains(name, "\n") {
			return fmt.Errorf("%q holds a line break, which git cat-file cannot be asked for", name)
		}
	}
	cmd := exec.Command("git", "-C", top, "cat-file", "--batch", "--follow-symlinks")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// The names are written while the answers are read, so that neither
	// side waits on a full pipe.
	go func() {
		w := bufio.NewWriter(in)
		for _, name := range names {
			fmt.Fprintf(w, "%s:%s\n", commit, name)
		}
		w.Flush()
		in.Close()
	}()
	r := bufio.NewReader(out)
	var failed error
	for _, name := range names {
		kind, content, err := object(r)
		switch {
		case err != nil:
		case kind == "blob":
			err = place(dir, name, content)
		case kind != "missing" && kind != "commit" && kind != "tag":
			err = broken(dir, name, kind)
		}
		if err != nil {
			failed = err
			break
		}
	}
	io.Copy(io.Discard, r)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git cat-file: %s", cmp.Or(strings.TrimSpace(stderr.String()), err.Error()))
	}
	return failed
}

// object reads git cat-file's next answer: what the name it was asked for
// is, and its content when a file.
//
// A file answers "<id> blob <size>", and a directory "<id> tree <size>". A
// link that leads out of the tree answers "symlink <size>", one that leads
// to nothing "dangling <size>", to itself "loop <size>" and through a file
// "notdir <size>". Each is followed by that many bytes and a line break. A
// name the tree does not hold answers "<name> missing".
func object(r *bufio.Reader) (string, []byte, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	header = strings.TrimSuffix(header, "\n")
	if strings.HasSuffix(header, " missing") {
		return "missing", nil, nil
	}
	f := strings.Fields(header)
	size, err := strconv.Atoi(f[len(f)-1])
	if err != nil || size < 0 || len(f) < 2 || len(f) > 3 {
		return "", nil, fmt.Errorf("git cat-file answered %q", header)
	}
	body := make([]byte, size+1)
	if _, err := io.ReadFull(r, body); err != nil {
		return "", nil, err
	}
	return f[len(f)-2], body[:size], nil
}

// place writes a file of a tree under dir.
func place(dir, name string, content []byte) error {
	target, err := placed(dir, name)
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}

// broken writes a link of a tree under dir that fails as the link fails on
// disk, by what cat-file found it to be: a link to a directory, out of the
// tree, to nothing, to itself or through a file. The checker then says of
// it what it says on disk.
func broken(dir, name, kind string) error {
	target, err := placed(dir, name)
	if err != nil {
		return err
	}
	base := path.Base(name)
	var to string
	switch kind {
	case "tree":
		to = "."
	case "symlink":
		to = strings.Repeat("../", strings.Count(name, "/")+1) + "outside"
	case "loop":
		to = base
	case "notdir":
		if err := os.WriteFile(target+".file", nil, 0o644); err != nil {
			return err
		}
		to = base + ".file/x"
	default:
		to = base + ".missing"
	}
	return os.Symlink(to, target)
}

// placed returns where a file of a tree goes under dir, with its directory
// made, refusing a name that would leave dir.
func placed(dir, name string) (string, error) {
	if path.Clean(name) != name || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("the tree names %s, outside itself", name)
	}
	target := filepath.Join(dir, filepath.FromSlash(name))
	return target, os.MkdirAll(filepath.Dir(target), 0o755)
}
