package serve

import (
	"encoding/json"
	"io"

	"shall/internal/finding"
)

// dumpAnswer is everything the server answers for the sets under a
// directory, in one document: each set it serves, with every file in full;
// the names several sets share, which none of them is served under; and the
// findings that belong to no set served under its name.
type dumpAnswer struct {
	Sets     []*setAnswer      `json:"sets"`
	Clashes  []clashJSON       `json:"clashes"`
	Findings []finding.Finding `json:"findings"`
}

// Dump writes to w, as one JSON document, every set under root as the
// server answers it, each file with all its items. A root that holds no set
// fails the dump, and nothing is written.
func Dump(root string, w io.Writer) error {
	snap := load(root, "")
	if err := snap.none(root, "dump"); err != nil {
		return err
	}
	list := snap.list()
	a := dumpAnswer{Sets: []*setAnswer{}, Clashes: list.Clashes, Findings: list.Findings}
	for _, sv := range snap.sets {
		if !sv.served {
			continue
		}
		b := builder{snap, sv}
		s := b.set()
		if sv.Set != nil {
			s.Files = s.Files[:0]
			for _, f := range sv.Set.Files {
				s.Files = append(s.Files, b.file(f, true))
			}
		}
		a.Sets = append(a.Sets, &s)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(a)
}
