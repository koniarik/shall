package serve

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shall/internal/set"
)

// recent is how long after a file's modification time its contents are
// compared too, for file systems that record times only to the second.
const recent = 2 * time.Second

// stamp is what a scan remembers of one file.
type stamp struct {
	mod    time.Time
	size   int64
	sum    [sha256.Size]byte
	summed bool
}

func (s *Server) watch(ctx context.Context) {
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.scan(s.dirs()) {
				s.reload()
			}
		}
	}
}

// dirs returns the directories to scan: the root, and the directory of every
// set whose files lie outside it.
func (s *Server) dirs() []string {
	root := absolute(s.root)
	out := []string{root}
	for _, sv := range s.current().sets {
		d := absolute(sv.Dir)
		if rel, err := filepath.Rel(root, d); err != nil || strings.HasPrefix(rel, "..") {
			out = append(out, d)
		}
	}
	return out
}

// scan looks at every manifest and set file under dirs and tells whether any
// appeared, vanished or changed since the last scan.
func (s *Server) scan(dirs []string) bool {
	now := time.Now()
	seen := map[string]stamp{}
	changed := false
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && p != dir {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" && p != dir {
					return fs.SkipDir
				}
				return nil
			}
			if name := d.Name(); name != set.ManifestName && !strings.HasSuffix(name, set.FileSuffix) {
				return nil
			}
			if _, dup := seen[p]; dup {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			st := stamp{mod: info.ModTime(), size: info.Size()}
			if now.Sub(st.mod) <= recent {
				if data, err := os.ReadFile(p); err == nil {
					st.sum, st.summed = sha256.Sum256(data), true
				}
			}
			old, had := s.stamps[p]
			if !had || !old.mod.Equal(st.mod) || old.size != st.size || (old.summed && st.summed && old.sum != st.sum) {
				changed = true
			}
			seen[p] = st
			return nil
		})
	}
	if len(seen) != len(s.stamps) {
		changed = true
	}
	s.stamps = seen
	return changed
}
