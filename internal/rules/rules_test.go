package rules

import (
	"strings"
	"testing"

	"shall/internal/finding"
	"shall/internal/model"
	"shall/internal/tree"
)

const mission = `mission: m
ranking: [O1]
objectives:
  - {id: O1, title: A, text: t, measure: m}
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S1, serves: [O1], status: hypothesis}
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
	return map[string]string{"mission.shall.yaml": mission, "c.shall.yaml": "title: C\nrequirements:\n" + src}
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
    serves: [N1]
    status: approved
    children:
      - {id: R1.1, title: B, text: The tool SHALL answer twice., status: draft}
`)))
}

func TestOneMissionFile(t *testing.T) {
	t.Attr("verifies", "R2")
	expect(t, run(t, "name: x\nformat: 1\n", map[string]string{"c.shall.yaml": "title: C\nrequirements: []\n"}),
		finding.Error, "", "has no mission.shall.yaml")
	expect(t, run(t, "name: x\nformat: 1\n", map[string]string{"mission.shall.yaml": mission, "a/mission.shall.yaml": mission}),
		finding.Error, "", "has exactly one")
}

func TestIdStructure(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: R1.1
    title: A
    text: The tool SHALL work.
    serves: [N1]
    status: draft
  - id: R2
    title: B
    text: The tool SHALL work.
    serves: [N1]
    status: draft
    children:
      - {id: R3.1, title: C, text: The tool SHALL work., status: draft}
  - {id: R2, title: D, text: The tool SHALL work., serves: [N1], status: draft}
`))
	expect(t, got, finding.Error, "R1.1", "top-level id has no")
	expect(t, got, finding.Error, "R3.1", "child of R2 has the id R2.<number>")
	expect(t, got, finding.Error, "R2", "already used")
}

func TestRetiredIds(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - id: R1
    title: A
    text: The tool SHALL work.
    serves: [N1, R5]
    status: draft
    retired:
      - {id: R1.1, title: Old}
      - {id: R9, title: Wrong}
  - {id: R5, title: Back, text: The tool SHALL work., serves: [N1], status: draft}
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
		"mission.shall.yaml": `mission: m
ranking: [O7]
objectives:
  - {id: O1, title: A, text: t, measure: m}
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: S9, serves: [mission, O2], status: hypothesis}
`,
		"c.shall.yaml": `title: C
requirements:
  - {id: R1, title: A, text: The tool SHALL work., serves: [N4, company/R1], status: draft}
`,
	})
	expect(t, got, finding.Error, "", "O7 is no item")
	expect(t, got, finding.Error, "N1", "S9 is no item")
	expect(t, got, finding.Error, "N1", "O2 is no item")
	expect(t, got, finding.Error, "R1", "N4 is no item")
	expect(t, got, finding.Error, "R1", "does not depend on")
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
  - {id: R2, title: B, text: The tool SHALL work and SHALL NOT stop., serves: [N1], status: draft}
  - {id: R3, title: C, text: The tool SHALL work and MAY rest., serves: [N1], status: draft}
  - {id: R4, title: D, text: Tool SHALL work., serves: [N1], status: draft}
  - id: R5
    title: E
    text: The tool SHALL work.
    serves: [N1]
    status: approved
    children:
      - {id: R5.1, title: F, text: The tool SHALL work.}
  - id: R6
    title: G
    text: The tool SHALL work.
    serves: [N1]
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
		"  - {id: R1, title: A, text: The tool SHALL work., serves: [N1], status: draft}\nretired:\n  - {id: \"\", title: Old}\n"))
}

func TestAChildOfARetiredItemIsRetired(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", chapter(`  - {id: R2, title: A, text: The tool SHALL work., serves: [N1, R1.1], status: draft}
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
		"mission.shall.yaml": `mission: m
ranking: [O1]
objectives:
  - {id: O1, title: A, text: t, measure: m}
stakeholders:
  - {id: S1, title: A, text: t}
needs:
  - {id: N1, title: A, text: t, stakeholder: O1, serves: [O1], status: hypothesis}
  - {id: N2, title: B, text: t, stakeholder: S1, serves: [O1], status: hypothesis, mirrors: R1}
`,
		"c.shall.yaml": "title: C\nrequirements:\n  - {id: R1, title: A, text: The tool SHALL work., serves: [S1], status: draft}\n",
	})
	expect(t, got, finding.Error, "N1", "O1 cannot be named in stakeholder")
	expect(t, got, finding.Error, "N2", "R1 cannot be named in mirrors")
	expect(t, got, finding.Error, "R1", "S1 cannot be named in serves")
}

func TestRetiredIdsAreUniqueAcrossFiles(t *testing.T) {
	t.Attr("verifies", "R2")
	files := chapter("  - {id: R1, title: A, text: The tool SHALL work., serves: [N1], status: draft}\nretired:\n  - {id: R4, title: Old}\n")
	files["d.shall.yaml"] = "title: D\nrequirements: []\nretired:\n  - {id: R4, title: Older}\n"
	expect(t, run(t, "name: x\nformat: 1\n", files), finding.Error, "R4", "already retired")
}

func TestKeywordsCountAsWholeWords(t *testing.T) {
	t.Attr("verifies", "R2")
	none(t, run(t, "name: x\nformat: 1\n", chapter(
		"  - {id: R1, title: A, text: The SHALLOW pool SHALL NOT overflow., serves: [N1], status: draft}\n")))
}

func TestTopLevelRetiredListsHoldTopLevelIdsOfTheirKinds(t *testing.T) {
	t.Attr("verifies", "R2")
	got := run(t, "name: x\nformat: 1\n", map[string]string{
		"mission.shall.yaml": mission + "retired:\n  - {id: R5, title: Wrong kind}\n",
		"c.shall.yaml":       "title: C\nrequirements: []\nretired:\n  - {id: R6.1, title: A child}\n  - {id: N7, title: Wrong kind}\n",
	})
	expect(t, got, finding.Error, "R5", "this list holds top-level ids of kinds OSN")
	expect(t, got, finding.Error, "R6.1", "this list holds top-level ids of kinds R")
	expect(t, got, finding.Error, "N7", "this list holds top-level ids of kinds R")
}

func TestATextThatIsNoTextIsLeftToTheSchema(t *testing.T) {
	t.Attr("verifies", "R2.2")
	got := run(t, "name: x\nformat: 1\n", chapter("  - {id: R1, title: A, text: [not, text], serves: [N1], status: draft}\n"+
		"  - {id: R2, title: B, text: The tool SHALL work., status: draft}\n"))
	for _, f := range got {
		if f.Field == "text" {
			t.Errorf("the rules judged a text that is no text: %+v; want that left to the schema", f)
		}
	}
	expect(t, got, finding.Warning, "R2", "the requirement serves nothing")
}
