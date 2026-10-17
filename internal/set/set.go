// Package set finds sets and the files that belong to them.
package set

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// ManifestName is the file name of a set's manifest.
const ManifestName = "shall.yaml"

// FileSuffix ends the name of every file that belongs to a set.
const FileSuffix = ".shall.yaml"

// MissionName is the file name of a set's mission file.
const MissionName = "mission.shall.yaml"

// Unreadable is a path below the walked directory that could not be read.
type Unreadable struct {
	Path string
	Err  error
}

// Manifests returns the path of every manifest under root, and every path
// below root it could not read. It fails only when root itself cannot be
// read.
func Manifests(root string) ([]string, []Unreadable, error) {
	return walk(root, func(name string) bool { return name == ManifestName })
}

// Files returns the path of every file under dir that belongs to a set, like
// Manifests.
func Files(dir string) ([]string, []Unreadable, error) {
	return walk(dir, func(name string) bool { return strings.HasSuffix(name, FileSuffix) })
}

func walk(root string, match func(string) bool) ([]string, []Unreadable, error) {
	var found []string
	var unreadable []Unreadable
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			unreadable = append(unreadable, Unreadable{Path: path, Err: err})
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if match(d.Name()) {
			found = append(found, path)
		}
		return nil
	})
	return found, unreadable, err
}
