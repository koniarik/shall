package model

import (
	"testing"

	"shall/internal/tree"
)

func parse(t *testing.T, files map[string]string) *Set {
	t.Helper()
	var parsed []*File
	for path, src := range files {
		n, err := tree.Parse([]byte(src))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		parsed = append(parsed, &File{Path: path, Tree: n})
	}
	return New(parsed)
}

func TestFilesInDocumentOrder(t *testing.T) {
	t.Attr("verifies", "R26")
	t.Attr("verifies", "R26.9")
	s := parse(t, map[string]string{
		"b/two.shall.yaml":     "title: B\nrequirements: []\n",
		"a.shall.yaml":         "title: A\nrequirements: []\n",
		"z/mission.shall.yaml": "mission: m\n",
		"b/one.shall.yaml":     "title: C\nrequirements: []\n",
	})
	var got []string
	for _, f := range s.Files {
		got = append(got, f.Path)
	}
	want := []string{"z/mission.shall.yaml", "a.shall.yaml", "b/one.shall.yaml", "b/two.shall.yaml"}
	if len(got) != len(want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("files = %v, want %v", got, want)
		}
	}
}

func TestEffectiveStatus(t *testing.T) {
	t.Attr("verifies", "R26.3")
	s := parse(t, map[string]string{"c.shall.yaml": `title: C
requirements:
  - id: R1
    title: A
    text: t
    status: verified
    children:
      - {id: R1.1, title: A, text: t, status: verified}
      - id: R1.2
        title: A
        text: t
        status: verified
        children:
          - {id: R1.2.1, title: A, text: t, status: implemented}
  - id: R2
    title: A
    text: t
    status: approved
    children:
      - {id: R2.1, title: A, text: t, status: waived}
  - id: R3
    title: A
    text: t
    status: waived
    children:
      - {id: R3.1, title: A, text: t}
`})
	for id, want := range map[string]string{
		"R1": "implemented", "R1.1": "verified", "R1.2": "implemented",
		"R2": "approved", "R2.1": "waived",
		"R3": "waived", "R3.1": "waived",
	} {
		if got := s.Item(id).EffectiveStatus(); got != want {
			t.Errorf("%s: effective status = %q, want %q", id, got, want)
		}
	}
}

func TestReferencesAndBacklinks(t *testing.T) {
	t.Attr("verifies", "R27.2")
	s := parse(t, map[string]string{
		"mission.shall.yaml": `mission: m
ranking: [O1]
objectives:
  - {id: O1, title: A, text: t, measure: m}
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S1, serves: [mission, O1], status: hypothesis}
`,
		"c.shall.yaml": "title: C\nrequirements:\n  - {id: R1, title: A, text: t, serves: [N1, x/R5], status: draft}\n",
	})
	named := map[string]string{}
	for _, id := range []string{"O1", "S1", "N1", "x/R5", "mission"} {
		for _, r := range s.NamedBy(id) {
			named[id] += r.From.ID + "." + r.Field + " "
		}
	}
	want := map[string]string{"O1": "N1.serves ", "S1": "N1.stakeholder ", "N1": "R1.serves ", "x/R5": "R1.serves "}
	for id, w := range want {
		if named[id] != w {
			t.Errorf("NamedBy(%s) = %q, want %q", id, named[id], w)
		}
	}
	if named["mission"] != "" {
		t.Errorf("the word mission names an item: %q", named["mission"])
	}
	if r := s.Ranking(); len(r) != 1 || r[0].ID != "O1" {
		t.Errorf("Ranking() = %+v", r)
	}
}

func TestRetiredWithAnAncestor(t *testing.T) {
	t.Attr("verifies", "R27.5")
	s := parse(t, map[string]string{"c.shall.yaml": `title: C
requirements:
  - id: R1
    title: A
    text: t
    status: draft
    retired:
      - {id: R1.2, title: Old}
retired:
  - {id: R2, title: Gone}
`})
	for id, want := range map[string]string{"R1.2": "R1.2", "R1.2.3": "R1.2", "R2.1": "R2", "R1": ""} {
		got := ""
		if e := s.RetiredWith(id); e != nil {
			got = e.ID
		}
		if got != want {
			t.Errorf("RetiredWith(%s) = %q, want %q", id, got, want)
		}
	}
	if s.Item("R1.2") != nil {
		t.Error("a retired id names an item")
	}
}
