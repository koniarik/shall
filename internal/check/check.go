// Package check runs every check on the sets under a directory.
package check

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"shall"
	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/rules"
	"shall/internal/schema"
	"shall/internal/set"
	"shall/internal/tree"
)

// Result is one set as read from disk and what the check found in it.
type Result struct {
	// Manifest is the path of the set's manifest, relative to the checked
	// directory.
	Manifest string
	// Tree is the manifest's content, or nil when it could not be read.
	Tree *tree.Node
	// Name is the name the manifest gives the set, or "".
	Name string
	// Dir is the directory holding the set's files, as a path on disk
	// rather than relative to the checked directory.
	Dir string
	// Set is the set's files and items, or nil when the manifest says too
	// little to find them.
	Set *model.Set
	// Findings are what the check found in the set, in file and line order.
	Findings []finding.Finding
}

// Run checks every set whose manifest lies under root and returns what it
// found, with paths relative to root. It fails only when root itself cannot
// be read.
func Run(root string) ([]finding.Finding, error) {
	results, out, err := Sets(root)
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		out = append(out, r.Findings...)
	}
	finding.Sort(out)
	return out, nil
}

// Sets checks every set whose manifest lies under root, like Run, and
// returns each set apart, in manifest path order, together with the findings
// that belong to no set.
func Sets(root string) ([]*Result, []finding.Finding, error) {
	return SetsWithin(root, "")
}

// SetsWithin is Sets for the sets of a repository whose root is top: a set
// whose path leads out of top, or a file of a set that is a link leading
// out of it, is found wrong rather than read. An empty top bounds nothing.
func SetsWithin(root, top string) ([]*Result, []finding.Finding, error) {
	v, err := schema.Load(shall.Schema())
	if err != nil {
		return nil, nil, err
	}
	manifests, unreadable, err := set.Manifests(root)
	if err != nil {
		return nil, nil, err
	}
	out := unreadableFindings(root, unreadable)
	if len(manifests) == 0 {
		out = append(out, finding.Finding{Severity: finding.Error, File: ".", Message: fmt.Sprintf("no %s under this directory", set.ManifestName)})
	}
	var results []*Result
	for _, m := range manifests {
		r := checkSet(root, top, m, v)
		finding.Sort(r.Findings)
		results = append(results, r)
	}
	return results, out, nil
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}

// leadsOut tells whether path is a link that leads out of top, itself or
// through the links it leads to; it looks no further than a system follows
// links.
func leadsOut(top, path string) bool {
	for range 40 {
		to, err := os.Readlink(path)
		if err != nil {
			return false
		}
		if !filepath.IsAbs(to) {
			to = filepath.Join(filepath.Dir(path), to)
		}
		if !within(top, to) {
			return true
		}
		path = to
	}
	return false
}

func outOfTheRepository(root, path string) finding.Finding {
	return finding.Finding{Severity: finding.Error, File: rel(root, path), Message: "the file is a link that leads out of the repository"}
}

// within tells whether path lies in dir.
func within(dir, path string) bool {
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	r, err := filepath.Rel(d, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
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

func checkSet(root, top, manifestPath string, v *schema.Validator) *Result {
	r := &Result{Manifest: rel(root, manifestPath), Dir: filepath.Dir(manifestPath)}
	if top != "" && leadsOut(top, manifestPath) {
		r.Findings = append(r.Findings, outOfTheRepository(root, manifestPath))
		return r
	}
	manifest, out := parse(root, manifestPath)
	r.Findings = out
	if manifest == nil {
		return r
	}
	r.Tree = manifest
	r.Name, _ = manifest.Get("name").Str()
	if f := manifest.Get("format"); f != nil {
		if n, ok := f.Number(); !ok || n != 1 {
			r.Findings = append(r.Findings, finding.Finding{
				Severity: finding.Error, File: r.Manifest, Line: f.Line, Field: "format",
				Message: fmt.Sprintf("format version %s is not one this checker reads; it reads 1", f.Text()),
			})
			return r
		}
	}
	r.Findings = append(r.Findings, v.Check("manifest.schema.json", r.Manifest, manifest)...)

	if p, ok := manifest.Get("path").Str(); ok {
		r.Dir = filepath.Join(r.Dir, filepath.FromSlash(p))
		if top != "" && !within(top, r.Dir) {
			r.Findings = append(r.Findings, finding.Finding{
				Severity: finding.Error, File: r.Manifest, Line: manifest.Get("path").Line, Field: "path",
				Message: fmt.Sprintf("%s leads out of the repository", p),
			})
			return r
		}
		if st, err := os.Stat(r.Dir); err != nil || !st.IsDir() {
			r.Findings = append(r.Findings, finding.Finding{
				Severity: finding.Error, File: r.Manifest, Line: manifest.Get("path").Line, Field: "path",
				Message: fmt.Sprintf("%s is not a directory", p),
			})
			return r
		}
	}
	paths, unreadable, err := set.Files(r.Dir)
	if err != nil {
		r.Findings = append(r.Findings, finding.Finding{Severity: finding.Error, File: r.Manifest, Message: cause(err)})
		return r
	}
	r.Findings = append(r.Findings, unreadableFindings(root, unreadable)...)
	var files []*model.File
	for _, p := range paths {
		if top != "" && leadsOut(top, p) {
			r.Findings = append(r.Findings, outOfTheRepository(root, p))
			continue
		}
		n, errs := parse(root, p)
		r.Findings = append(r.Findings, errs...)
		file := &model.File{Path: rel(root, p), Tree: n}
		files = append(files, file)
		if n == nil {
			continue
		}
		kind := "chapter.schema.json"
		if filepath.Base(p) == set.MissionName {
			kind = "mission.schema.json"
		}
		r.Findings = append(r.Findings, v.Check(kind, file.Path, n)...)
	}
	r.Set = model.New(files)
	r.Findings = append(r.Findings, rules.Check(r.Manifest, manifest, r.Set)...)
	return r
}
