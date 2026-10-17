package schema

import (
	"strings"
	"testing"
	"testing/fstest"

	"shall"
	"shall/internal/finding"
	"shall/internal/tree"
)

func load(t *testing.T) *Validator {
	t.Helper()
	v, err := Load(shall.Schema())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func check(t *testing.T, schema, src string) []finding.Finding {
	t.Helper()
	n, err := tree.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return load(t).Check(schema, "f.shall.yaml", n)
}

func expect(t *testing.T, got []finding.Finding, line int, id, field, fragment string) {
	t.Helper()
	for _, f := range got {
		if f.File == "f.shall.yaml" && f.Line == line && f.ID == id && f.Field == field && strings.Contains(f.Message, fragment) {
			return
		}
	}
	t.Errorf("no finding at line %d, id %q, field %q containing %q; got %+v", line, id, field, fragment, got)
}

func refused(t *testing.T, schema string) {
	t.Helper()
	if _, err := Load(fstest.MapFS{"x.schema.json": {Data: []byte(schema)}}); err == nil {
		t.Errorf("schema %s was accepted", schema)
	}
}

func TestRefusesUnsupportedKeyword(t *testing.T) {
	t.Attr("verifies", "R21.1")
	_, err := Load(fstest.MapFS{"x.schema.json": {Data: []byte(`{"type": "object", "maxItems": 3}`)}})
	if err == nil || !strings.Contains(err.Error(), "maxItems") {
		t.Fatalf("error = %v, want one naming maxItems", err)
	}
}

func TestRefusesUnsupportedForms(t *testing.T) {
	t.Attr("verifies", "R21.1")
	refused(t, `{"properties": {"a": {"format": "email"}}}`)
	refused(t, `{"type": ["string", "null"]}`)
	refused(t, `{"type": "number"}`)
	refused(t, `{"items": [{"type": "string"}]}`)
	refused(t, `{"enum": [{"a": 1}]}`)
	refused(t, `{"pattern": "("}`)
	refused(t, `{"$ref": "#/$defs/missing"}`)
	refused(t, `{"required": "a"}`)
}

func TestLoadsTheFormatSchema(t *testing.T) {
	t.Attr("verifies", "R21")
	load(t)
}

func TestValidFilesHaveNoFindings(t *testing.T) {
	t.Attr("verifies", "R21")
	got := check(t, "chapter.schema.json", `title: T
requirements:
  - id: R1
    title: A
    text: The tool SHALL work.
    serves: [N1, R2, company/R4]
    status: approved
    links:
      - url: https://example.com/run/1
      - url: file:///tmp/evidence.pdf
      - url: urn:isbn:0451450523
    children:
      - id: R1.1
        title: B
        text: The tool SHALL work too.
`)
	if len(got) != 0 {
		t.Fatalf("findings = %+v, want none", got)
	}
}

func TestReportsShapeErrorsWhereTheyAre(t *testing.T) {
	t.Attr("verifies", "R21")
	t.Attr("verifies", "R3")
	got := check(t, "chapter.schema.json", `title: T
requirements:
  - id: R1
    title: A
    text: The tool SHALL work.
    status: done
    colour: red
    serves: [O1]
    links:
      - url: not a uri
    children:
      - id: R1.1
        text: x
`)
	expect(t, got, 6, "R1", "status", "is not one of draft, approved")
	expect(t, got, 7, "R1", "colour", "is not defined")
	expect(t, got, 8, "R1", "serves", `"O1" is not valid`)
	expect(t, got, 10, "R1", "url", "is not a URI")
	expect(t, got, 12, "R1.1", "title", "is missing")
}

func TestChecksTheMissionFile(t *testing.T) {
	t.Attr("verifies", "R21")
	got := check(t, "mission.schema.json", `ranking: [N1]
needs:
  - id: N01
    title: A
    text: t
    stakeholder: S1
    serves: []
`)
	expect(t, got, 1, "", "mission", "is missing")
	expect(t, got, 1, "", "ranking", `"N1" is not a valid value`)
	expect(t, got, 3, "N01", "id", "is not a valid value")
	expect(t, got, 3, "N01", "status", "is missing")
	expect(t, got, 7, "N01", "serves", "needs at least 1 entries")
}

func TestChecksTheManifest(t *testing.T) {
	t.Attr("verifies", "R21")
	got := check(t, "manifest.schema.json", `name: x
format: 1
depends:
  Company:
    git: https://example.com/standards
`)
	expect(t, got, 4, "", "name", "is not a valid value")
	expect(t, got, 5, "", "rev", "is required with \"git\"")
}

func TestFormatOnePointZeroIsOne(t *testing.T) {
	if got := check(t, "manifest.schema.json", "name: x\nformat: 1.0\n"); len(got) != 0 {
		t.Fatalf("findings = %+v, want none", got)
	}
}
