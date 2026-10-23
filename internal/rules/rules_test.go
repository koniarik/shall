package rules

import (
	"strings"
	"testing"

	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/tree"
)

const mission = `title: M
mission: m
ranking: [O90]
items:
  - {id: O90, title: A, text: t, measure: m}
  - id: S91
    title: A
    text: t
    children:
      - {id: N91.1, title: A, text: t, serves: [O90], status: hypothesis}
`

func run(t *testing.T, manifest string, files map[string]string) []finding.Finding {
	t.Helper()
	m, err := tree.Parse([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	var parsed []*model.File
	for path, src := range files {
		n, err := tree.Parse([]byte(src))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		parsed = append(parsed, &model.File{Path: path, Tree: n})
	}
	return Check("shall.yaml", m, model.New(parsed))
}

func chapter(src string) map[string]string {
	return map[string]string{"mission.shall.yaml": mission, "c.shall.yaml": "title: C\nitems:\n" + src}
}

func expect(t *testing.T, got []finding.Finding, sev finding.Severity, id, fragment string) {
	t.Helper()
	for _, f := range got {
		if f.Severity == sev && f.ID == id && strings.Contains(f.Message, fragment) {
			return
		}
	}
	t.Errorf("no %s for %q containing %q; got %+v", sev, id, fragment, got)
}

func none(t *testing.T, got []finding.Finding) {
	t.Helper()
	if len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestValidSetHasNoFindings(t *testing.T) {
	t.Attr("verifies", "R2")
	none(t, run(t, "name: x\nformat: 1\n", chapter(`  - id: R1
    title: A
    text: When asked, the tool SHALL answer.
    serves: [N91.1]
    status: approved
    children:
      - {id: R1.1, title: B, text: The tool SHALL answer twice., status: draft}
`)))
}

func TestOneMissionFile(t *testing.T) {
	t.Attr("verifies", "R2")
	expect(t, run(t, "name: x\nformat: 1\n", map[string]string{"c.shall.yaml": "title: C\nitems: []\n"}),
		finding.Error, "", "has no mission.shall.yaml")
	expect(t, run(t, "name: x\nformat: 1\n", map[string]string{"mission.shall.yaml": mission, "a/mission.shall.yaml": mission}),
		finding.Error, "", "has exactly one")
}

func TestIdStructure(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: R1.1
    title: A
    text: The tool SHALL work.
    serves: [N91.1]
    status: draft
  - id: R2
    title: B
    text: The tool SHALL work.
    serves: [N91.1]
    status: draft
    children:
      - {id: R3.1, title: C, text: The tool SHALL work., status: draft}
  - {id: R2, title: D, text: The tool SHALL work., serves: [N91.1], status: draft}
`))
	expect(t, got, finding.Error, "R1.1", "top-level id has no")
	expect(t, got, finding.Error, "R3.1", "child of R2 has the id <letter>2.<number>")
	expect(t, got, finding.Error, "R2", "already used")
}

func TestRetiredIds(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: R1
    title: A
    text: The tool SHALL work.
    serves: [N91.1, R5]
    status: draft
    retired:
      - {id: R1.1, title: Old}
      - {id: R9, title: Wrong}
  - {id: R5, title: Back, text: The tool SHALL work., serves: [N91.1], status: draft}
retired:
  - {id: R5, title: Gone}
`))
	expect(t, got, finding.Error, "R5", "retired at")
	expect(t, got, finding.Warning, "R1", "R5 is retired")
	expect(t, got, finding.Error, "R9", "only retires its own children")
}

func TestReferences(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", map[string]string{
		"mission.shall.yaml": `title: M
mission: m
ranking: [O7, company/O1]
items:
  - {id: O1, title: A, text: t, measure: m}
  - id: S2
    title: A
    text: t
    children:
      - {id: N2.1, title: A, text: t, stakeholders: [S9, company/S1], serves: [mission, O3, company/O1], status: hypothesis}
`,
		"c.shall.yaml": `title: C
items:
  - {id: R4, title: A, text: The tool SHALL work., serves: [N5, company/R1], status: draft}
`,
	})
	expect(t, got, finding.Error, "", "O7 is no item")
	expect(t, got, finding.Error, "", "company/O1 names another set; ranking names items of this set only")
	expect(t, got, finding.Error, "N2.1", "S9 is no item")
	expect(t, got, finding.Error, "N2.1", "company/S1 names another set; stakeholders names items of this set only")
	expect(t, got, finding.Error, "N2.1", "O3 is no item")
	expect(t, got, finding.Error, "N2.1", "company/O1 names another set; serves names items of this set only")
	expect(t, got, finding.Error, "R4", "N5 is no item")
	expect(t, got, finding.Error, "R4", "does not depend on")
}

func TestDependenciesCannotBeRead(t *testing.T) {
	t.Attr("verifies", "R2.3")
	got := run(t, "name: x\nformat: 1\ndepends:\n  company: {git: https://example.com/s, rev: v1}\n", chapter(
		"  - {id: R1, title: A, text: The tool SHALL work., serves: [company/R4], status: draft}\n"))
	expect(t, got, finding.Error, "", `set "company" cannot be read`)
	for _, f := range got {
		if f.ID == "R1" {
			t.Errorf("finding on R1 = %+v, want none", f)
		}
	}
}

func TestRequirements(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: R1, title: A, text: The tool works., status: draft}
  - {id: R2, title: B, text: The tool SHALL work and SHALL NOT stop., serves: [N91.1], status: draft}
  - {id: R3, title: C, text: The tool SHALL work and MAY rest., serves: [N91.1], status: draft}
  - {id: R4, title: D, text: Tool SHALL work., serves: [N91.1], status: draft}
  - id: R5
    title: E
    text: The tool SHALL work.
    serves: [N91.1]
    status: approved
    children:
      - {id: R5.1, title: F, text: The tool SHALL work.}
  - id: R6
    title: G
    text: The tool SHALL work.
    serves: [N91.1]
    status: waived
    children:
      - {id: R6.1, title: H, text: The tool SHALL work.}
`))
	expect(t, got, finding.Warning, "R1", "requirement serves nothing")
	expect(t, got, finding.Error, "R1", "states no SHALL")
	expect(t, got, finding.Error, "R2", "says SHALL 2 times")
	expect(t, got, finding.Error, "R3", "uses MAY")
	expect(t, got, finding.Warning, "R4", "none of the sentence forms")
	expect(t, got, finding.Error, "R5.1", `"status" is missing`)
	for _, f := range got {
		if f.ID == "R6.1" {
			t.Errorf("finding on R6.1 = %+v, want none: a child of a waived requirement needs no status", f)
		}
	}
}

func TestAnEmptyRetiredIdDoesNotStopTheCheck(t *testing.T) {
	t.Attr("verifies", "R2.2")
	run(t, "name: x\nformat: 1\n", chapter(
		"  - {id: R1, title: A, text: The tool SHALL work., serves: [N91.1], status: draft}\nretired:\n  - {id: \"\", title: Old}\n"))
}

func TestAChildOfARetiredItemIsRetired(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: R2, title: A, text: The tool SHALL work., serves: [N91.1, R1.1], status: draft}
retired:
  - {id: R1, title: Old}
`))
	expect(t, got, finding.Warning, "R2", "R1.1 is retired")
	for _, f := range got {
		if f.Severity == finding.Error {
			t.Errorf("error = %+v, want none", f)
		}
	}
}

func TestReferencesNameTheRightKind(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", map[string]string{
		"mission.shall.yaml": `title: M
mission: m
ranking: [O1]
items:
  - {id: O1, title: A, text: t, measure: m}
  - id: S2
    title: A
    text: t
    children:
      - {id: N2.1, title: A, text: t, stakeholders: [O1], serves: [O1], status: hypothesis}
      - {id: N2.2, title: B, text: t, serves: [O1], status: hypothesis, mirrors: R3}
  - {id: T4, title: Terms, text: t}
`,
		"c.shall.yaml": "title: C\nitems:\n  - {id: R3, title: A, text: The tool SHALL work., serves: [S2, T4], status: draft}\n",
	})
	expect(t, got, finding.Error, "N2.1", "O1 cannot be named in stakeholders")
	expect(t, got, finding.Error, "N2.2", "R3 cannot be named in mirrors")
	expect(t, got, finding.Error, "R3", "S2 cannot be named in serves")
	expect(t, got, finding.Error, "R3", "T4 cannot be named in serves")
}

func TestRetiredIdsAreUniqueAcrossFiles(t *testing.T) {
	t.Attr("verifies", "R2")
	files := chapter("  - {id: R1, title: A, text: The tool SHALL work., serves: [N91.1], status: draft}\nretired:\n  - {id: R4, title: Old}\n")
	files["d.shall.yaml"] = "title: D\nitems: []\nretired:\n  - {id: T4, title: Older}\n"
	expect(t, run(t, "name: x\nformat: 1\n", files), finding.Error, "T4", "already retired at c.shall.yaml:5, as R4")
}

func TestKeywordsCountAsWholeWords(t *testing.T) {
	t.Attr("verifies", "R2")
	none(t, run(t, "name: x\nformat: 1\n", chapter(
		"  - {id: R1, title: A, text: The SHALLOW pool SHALL NOT overflow., serves: [N91.1], status: draft}\n")))
}

func TestTopLevelRetiredListsHoldTopLevelIds(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", map[string]string{
		"mission.shall.yaml": mission + "retired:\n  - {id: R5, title: Any kind}\n",
		"c.shall.yaml":       "title: C\nitems: []\nretired:\n  - {id: R6.1, title: A child}\n  - {id: N7, title: Any kind}\n",
	})
	expect(t, got, finding.Error, "R6.1", "this list holds top-level ids")
	for _, f := range got {
		if f.ID == "R5" || f.ID == "N7" {
			t.Errorf("finding on %s = %+v, want none: a chapter retires top-level items of any kind", f.ID, f)
		}
	}
}

func TestKindsNestAsTheFormatAllows(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: S1
    title: A
    text: t
    children:
      - {id: N1.1, title: B, text: t, serves: [O90], status: hypothesis, children: [{id: R1.1.1, title: C, text: The tool SHALL work., status: draft}]}
      - {id: O1.2, title: D, text: t, measure: m}
      - id: T1.3
        title: E
        children:
          - {id: O1.3.1, title: F, text: t, measure: m}
          - {id: R1.3.2, title: G, text: The tool SHALL work., serves: [N1.1], status: draft, children: [{id: T1.3.2.1, title: H, text: t}]}
`))
	expect(t, got, finding.Error, "R1.1.1", "R1.1.1 cannot sit under N1.1; a need holds only items of its own kind and topics")
	expect(t, got, finding.Error, "O1.2", "O1.2 cannot sit under S1; a stakeholder class holds classes, needs and topics")
	for _, f := range got {
		if strings.Contains(f.Message, "cannot sit under") && f.ID != "R1.1.1" && f.ID != "O1.2" {
			t.Errorf("finding = %+v, want none: a topic holds anything, and sits under anything", f)
		}
	}
}

func TestAPathNamesOneItem(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: R1, title: A, text: The tool SHALL work., serves: [N91.1], status: draft}
  - {id: T1, title: B, text: t}
  - {id: T5, title: C, text: t}
  - {id: R6, title: D, text: The tool SHALL work., serves: [N91.1, T1, N1, R5, N5], status: draft}
retired:
  - {id: R5, title: Gone}
`))
	expect(t, got, finding.Error, "T1", "T1 is already used at c.shall.yaml:3, as R1; a path names one item, whatever its letter")
	expect(t, got, finding.Error, "T5", "T5 cannot return: its path is retired at c.shall.yaml:8, as R5")
	expect(t, got, finding.Error, "R6", "N1 names the path of R1, at c.shall.yaml:3; the letter does not match")
	expect(t, got, finding.Error, "R6", "T1 names the path of R1")
	expect(t, got, finding.Error, "R6", "N5 names the path of R5, retired at c.shall.yaml:8; the letter does not match")
	expect(t, got, finding.Warning, "R6", "R5 is retired")
}

func TestInheritedFieldsLookPastTopics(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: T1
    title: A
    children:
      - {id: O1.1, title: B, text: t}
      - {id: R1.2, title: C, text: The tool SHALL work.}
      - {id: N1.3, title: D, text: t, stakeholders: [S91], status: hypothesis}
  - id: R2
    title: E
    text: The tool SHALL work.
    serves: [N91.1]
    status: waived
    children:
      - {id: T2.1, title: F, children: [{id: R2.1.1, title: G, text: The tool SHALL work.}]}
  - id: O3
    title: H
    text: t
    measure: m
    children:
      - {id: T3.1, title: I, children: [{id: O3.1.1, title: J, text: t}]}
`))
	expect(t, got, finding.Error, "O1.1", `"measure" is missing`)
	expect(t, got, finding.Warning, "R1.2", "the requirement serves nothing")
	expect(t, got, finding.Error, "R1.2", `"status" is missing`)
	expect(t, got, finding.Error, "N1.3", `"serves" is missing`)
	for _, f := range got {
		if f.ID == "R2.1.1" || f.ID == "O3.1.1" {
			t.Errorf("finding on %s = %+v, want none: it inherits through a topic, or sits below a waived requirement", f.ID, f)
		}
	}
}

func TestANeedsClasses(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: N1, title: A, text: t, serves: [O90], status: hypothesis}
  - {id: N2, title: B, text: t, stakeholders: [S91], serves: [O90], status: hypothesis, children: [{id: N2.1, title: C, text: t, status: hypothesis}]}
  - id: S3
    title: D
    text: t
    children:
      - {id: N3.1, title: E, text: t, stakeholders: [S3, S91], serves: [O90], status: hypothesis}
      - {id: T3.2, title: F, children: [{id: N3.2.1, title: G, text: t, serves: [O90], status: hypothesis}]}
`))
	expect(t, got, finding.Warning, "N1", "the need has no stakeholder class")
	expect(t, got, finding.Warning, "N3.1", "S3 is the class the need sits under; naming it again says it twice")
	for _, f := range got {
		if f.ID == "N2" || f.ID == "N2.1" || f.ID == "N3.2.1" {
			t.Errorf("finding on %s = %+v, want none: it names a class, inherits one, or sits under one", f.ID, f)
		}
		if f.ID == "N3.1" && strings.Contains(f.Message, "S91") {
			t.Errorf("finding = %+v, want none for a class besides the one it sits under", f)
		}
	}
}

func TestATopicExplainsOrGroups(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: T1, title: Empty}
  - {id: T2, title: Text, text: t}
  - {id: T3, title: Group, children: [{id: T3.1, title: Text, text: t}]}
`))
	expect(t, got, finding.Warning, "T1", "neither text nor children")
	if len(got) != 1 {
		t.Errorf("findings = %+v, want T1's alone", got)
	}
}

func TestATextThatIsNoTextIsLeftToTheSchema(t *testing.T) {
	t.Attr("verifies", "R2.2")
	got := run(t, "name: x\nformat: 1\n", chapter("  - {id: R1, title: A, text: [not, text], serves: [N91.1], status: draft}\n"+
		"  - {id: R2, title: B, text: The tool SHALL work., status: draft}\n"))
	for _, f := range got {
		if f.Field == "text" {
			t.Errorf("the rules judged a text that is no text: %+v; want that left to the schema", f)
		}
	}
	expect(t, got, finding.Warning, "R2", "the requirement serves nothing")
}

func TestCitationsAreWarnings(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", map[string]string{
		"mission.shall.yaml": strings.Replace(mission, "mission: m", "mission: as [O99] says", 1),
		"c.shall.yaml": `title: C
items:
  - id: R1
    title: A
    text: The tool SHALL do as [T2], [N1] and [nowhere/R1] say.
    rationale: See [R5], and [T2.1].
    serves: [N91.1]
    status: draft
  - {id: T2, title: Terms, text: "Read [R1] first."}
retired:
  - {id: R5, title: Gone}
`,
	})
	expect(t, got, finding.Warning, "", "O99 is no item")
	expect(t, got, finding.Warning, "R1", "N1 names the path of R1")
	expect(t, got, finding.Warning, "R1", `nowhere/R1 names set "nowhere"`)
	expect(t, got, finding.Warning, "R1", "R5 is retired")
	expect(t, got, finding.Warning, "R1", "T2.1 is no item")
	for _, f := range got {
		if f.Severity == finding.Error {
			t.Errorf("error = %+v, want every citation's finding a warning", f)
		}
		if strings.Contains(f.Message, "[T2]") || strings.Contains(f.Message, "T2 cannot be named") {
			t.Errorf("finding = %+v, want a topic citable", f)
		}
	}
}
