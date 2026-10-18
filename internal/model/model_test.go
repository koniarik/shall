package model

import (
	"strings"
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
		t.Errorf("NamedBy(mission) = %q, want nothing: the word mission names no item", named["mission"])
	}
	if r := s.Ranking(); len(r) != 1 || r[0].ID != "O1" {
		t.Errorf("Ranking() = %+v, want O1 alone", r)
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

func TestTitlesTextsAndChapterTitles(t *testing.T) {
	s := parse(t, map[string]string{"c.shall.yaml": "title: Water Tank\nrequirements:\n  - {id: R1, title: Refill Lamp, text: The lamp SHALL light., status: draft}\n"})
	f := s.Files[0]
	if f.Title() != "Water Tank" || f.Mission {
		t.Errorf("file = %q, mission %v; want Water Tank, a chapter", f.Title(), f.Mission)
	}
	r1 := s.Item("R1")
	if r1.Title() != "Refill Lamp" || r1.Text() != "The lamp SHALL light." || r1.Status() != "draft" {
		t.Errorf("R1 = %q %q %q, want Refill Lamp, its text, draft", r1.Title(), r1.Text(), r1.Status())
	}
	broken := &File{Path: "broken.shall.yaml"}
	if broken.Title() != "" {
		t.Error("a file that cannot be read has a title")
	}
}

func TestAStatusOutsideTheOrder(t *testing.T) {
	s := parse(t, map[string]string{"c.shall.yaml": "title: C\nrequirements:\n  - {id: R1, title: A, text: t, status: pending}\n  - {id: R2, title: A, text: t}\n"})
	if got := s.Item("R1").EffectiveStatus(); got != "pending" {
		t.Errorf("effective status = %q, want the status as written", got)
	}
	if got := s.Item("R2").EffectiveStatus(); got != "" {
		t.Errorf("effective status of no status = %q, want none", got)
	}
}

func TestEntriesWithoutAnIdAreLeftOut(t *testing.T) {
	s := parse(t, map[string]string{"c.shall.yaml": `title: C
requirements:
  - {title: No Id, text: t, children: [{id: R9, title: A, text: t}]}
  - id: R1
    title: A
    text: t
    status: draft
    children:
      - {title: Also No Id, text: t}
    retired:
      - {title: Nameless}
      - {id: R1.2, title: Gone}
retired:
  - {title: Nameless}
`})
	if len(s.Items) != 1 || s.Items[0].ID != "R1" || len(s.Items[0].Children) != 0 {
		t.Errorf("items = %+v, want R1 alone, with no children", s.Items)
	}
	if len(s.Retired) != 1 || s.Retired[0].ID != "R1.2" {
		t.Errorf("retired = %+v, want R1.2 alone", s.Retired)
	}
	if s.Item("R9") != nil {
		t.Error("the child of an item without an id was read")
	}
}

func TestRetiredEntryIsExact(t *testing.T) {
	s := parse(t, map[string]string{"c.shall.yaml": "title: C\nrequirements: []\nretired:\n  - {id: R2, title: Gone}\n  - {id: R2, title: Again}\n"})
	if e := s.RetiredEntry("R2"); e == nil || e.Title != "Gone" {
		t.Errorf("RetiredEntry(R2) = %+v, want the first entry", e)
	}
	if s.RetiredEntry("R2.1") != nil {
		t.Error("RetiredEntry matched a child of a retired id")
	}
	if e := s.RetiredWith("R2.1"); e == nil || e.ID != "R2" {
		t.Errorf("RetiredWith(R2.1) = %+v, want the entry of R2", e)
	}
}

func TestRankingSkipsWhatIsNoId(t *testing.T) {
	s := parse(t, map[string]string{"mission.shall.yaml": "mission: m\nranking: [O1, 7, {a: b}, O2]\n"})
	var got []string
	for _, r := range s.Ranking() {
		got = append(got, r.ID)
	}
	if len(got) != 2 || got[0] != "O1" || got[1] != "O2" {
		t.Errorf("ranking = %v, want O1 and O2", got)
	}
}

func TestABrokenFileHoldsNothing(t *testing.T) {
	s := New([]*File{
		{Path: "b.shall.yaml"},
		{Path: "mission.shall.yaml", Tree: mustParse(t, "mission: m\nranking: [O1]\n")},
		{Path: "a.shall.yaml", Tree: mustParse(t, "title: A\nrequirements:\n  - {id: R1, title: A, text: t, status: draft}\n")},
	})
	var paths []string
	for _, f := range s.Files {
		paths = append(paths, f.Path)
	}
	if strings.Join(paths, " ") != "mission.shall.yaml a.shall.yaml b.shall.yaml" {
		t.Errorf("files = %v, want the mission file, then a and b in path order", paths)
	}
	if len(s.Items) != 1 || len(s.Ranking()) != 1 || s.Files[2].Items != nil {
		t.Errorf("items = %d, ranking = %d, broken file items = %v; want 1, 1 and none", len(s.Items), len(s.Ranking()), s.Files[2].Items)
	}
}

func mustParse(t *testing.T, src string) *tree.Node {
	t.Helper()
	n, err := tree.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
