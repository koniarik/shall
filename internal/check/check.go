// Package check runs every check on the sets under a directory.
package check

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"shall"
	"shall/internal/finding"
	"shall/internal/rules"
	"shall/internal/schema"
	"shall/internal/set"
	"shall/internal/tree"
)

// Run checks every set whose manifest lies under root and returns what it
// found, with paths relative to root. It fails only when root itself cannot
// be read.
func Run(root string) ([]finding.Finding, error) {
	v, err := schema.Load(shall.Schema())
	if err != nil {
		return nil, err
	}
	manifests, unreadable, err := set.Manifests(root)
	if err != nil {
		return nil, err
	}
	out := unreadableFindings(root, unreadable)
	if len(manifests) == 0 {
		out = append(out, finding.Finding{Severity: finding.Error, File: ".", Message: fmt.Sprintf("no %s under this directory", set.ManifestName)})
	}
	for _, m := range manifests {
		out = append(out, checkSet(root, m, v)...)
	}
	finding.Sort(out)
	return out, nil
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}

func cause(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Sprintf("cannot %s: %v", pe.Op, pe.Err)
	}
	return err.Error()
}

func unreadableFindings(root string, unreadable []set.Unreadable) []finding.Finding {
	var out []finding.Finding
	for _, u := range unreadable {
		out = append(out, finding.Finding{Severity: finding.Error, File: rel(root, u.Path), Message: cause(u.Err)})
	}
	return out
}

func parse(root, path string) (*tree.Node, []finding.Finding) {
	f := finding.Finding{Severity: finding.Error, File: rel(root, path)}
	data, err := os.ReadFile(path)
	if err != nil {
		f.Message = cause(err)
		return nil, []finding.Finding{f}
	}
	n, err := tree.Parse(data)
	if err != nil {
		f.Message = err.Error()
		var e *tree.Error
		if errors.As(err, &e) {
			f.Line, f.Message = e.Line, e.Message
		}
		return nil, []finding.Finding{f}
	}
	return n, nil
}

func checkSet(root, manifestPath string, v *schema.Validator) []finding.Finding {
	manifestRel := rel(root, manifestPath)
	manifest, out := parse(root, manifestPath)
	if manifest == nil {
		return out
	}
	if f := manifest.Get("format"); f != nil {
		if n, ok := f.Number(); !ok || n != 1 {
			return append(out, finding.Finding{
				Severity: finding.Error, File: manifestRel, Line: f.Line, Field: "format",
				Message: fmt.Sprintf("format version %s is not one this checker reads; it reads 1", f.Text()),
			})
		}
	}
	out = append(out, v.Check("manifest.schema.json", manifestRel, manifest)...)

	dir := filepath.Dir(manifestPath)
	if p, ok := manifest.Get("path").Str(); ok {
		dir = filepath.Join(dir, filepath.FromSlash(p))
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return append(out, finding.Finding{
				Severity: finding.Error, File: manifestRel, Line: manifest.Get("path").Line, Field: "path",
				Message: fmt.Sprintf("%s is not a directory", p),
			})
		}
	}
	paths, unreadable, err := set.Files(dir)
	if err != nil {
		return append(out, finding.Finding{Severity: finding.Error, File: manifestRel, Message: cause(err)})
	}
	out = append(out, unreadableFindings(root, unreadable)...)
	var files []rules.File
	for _, p := range paths {
		n, errs := parse(root, p)
		out = append(out, errs...)
		if n == nil {
			continue
		}
		file := rel(root, p)
		kind := "chapter.schema.json"
		if filepath.Base(p) == set.MissionName {
			kind = "mission.schema.json"
		}
		out = append(out, v.Check(kind, file, n)...)
		files = append(files, rules.File{Path: file, Tree: n})
	}
	return append(out, rules.Check(manifestRel, manifest, files)...)
}
