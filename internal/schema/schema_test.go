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

func TestRefusesMalformedSchemas(t *testing.T) {
	t.Attr("verifies", "R21.1")
	for _, s := range []string{
		`{"$ref": 1}`,
		`{"properties": []}`,
		`{"properties": {"a": 1}}`,
		`{"properties": {"a": {"maxItems": 1}}}`,
		`{"items": {"maxItems": 1}}`,
		`{"additionalProperties": 1}`,
		`{"unevaluatedProperties": {"maxItems": 1}}`,
		`{"anyOf": {}}`,
		`{"anyOf": [1]}`,
		`{"anyOf": [{"maxItems": 1}]}`,
		`{"dependentRequired": 1}`,
		`{"dependentRequired": {"a": "b"}}`,
		`{"enum": "a"}`,
		`{"const": {"a": 1}}`,
		`{"pattern": 1}`,
		`{"minLength": "1"}`,
	} {
		refused(t, s)
	}
	if _, err := Load(fstest.MapFS{"x.schema.json": {Data: []byte("not json")}}); err == nil || !strings.Contains(err.Error(), "x.schema.json") {
		t.Errorf("a file that is no JSON: error = %v, want one naming the file", err)
	}
}

// own loads one schema of a test's own and checks src against it.
func own(t *testing.T, schema, src string) []finding.Finding {
	t.Helper()
	v, err := Load(fstest.MapFS{"t.schema.json": {Data: []byte(schema)}})
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return v.Check("t.schema.json", "f.shall.yaml", n)
}

func TestNamesTheTypeItWanted(t *testing.T) {
	t.Attr("verifies", "R3")
	got := own(t, `{"type": "object", "properties": {
		"m": {"type": "object"}, "l": {"type": "array"}, "s": {"type": "string"},
		"i": {"type": "integer"}, "b": {"type": "boolean"}}}`,
		"m: 1\nl: 1\ns: [x]\ni: one\nb: yes please\n")
	for line, want := range map[int]string{1: `field "m" should be a mapping`, 2: `field "l" should be a list`, 3: `field "s" should be text`, 4: `field "i" should be a whole number`, 5: `field "b" should be true or false`} {
		expect(t, got, line, "", fieldOf(want), want)
	}
	if got := own(t, `{"type": "object"}`, "[1]\n"); len(got) != 1 || got[0].Message != "the file should be a mapping" {
		t.Errorf("a file of the wrong shape = %+v, want that the file should be a mapping", got)
	}
	if got := own(t, `{"type": "object", "properties": {"i": {"type": "integer"}, "b": {"type": "boolean"}}}`, "i: 3\nb: true\n"); len(got) != 0 {
		t.Errorf("values of the right type = %+v, want no findings", got)
	}
}

func fieldOf(message string) string {
	_, rest, _ := strings.Cut(message, `"`)
	name, _, _ := strings.Cut(rest, `"`)
	return name
}

func TestConstantsLengthsAndNestedFields(t *testing.T) {
	t.Attr("verifies", "R3")
	got := own(t, `{"type": "object", "properties": {
		"c": {"const": 1},
		"n": {"type": "string", "minLength": 3},
		"o": {"type": "object", "properties": {"a": {"type": "string"}}, "unevaluatedProperties": false}}}`,
		"c: 2\nn: ab\no:\n  a: x\n  z: y\n")
	expect(t, got, 1, "", "c", `field "c" must be 1, not 2`)
	expect(t, got, 2, "", "n", `field "n" needs at least 3 characters`)
	expect(t, got, 5, "", "z", `field "z" is not defined here`)
}

func TestCheckingAgainstASchemaNotLoadedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("checking against a schema that is not loaded did not panic")
		}
	}()
	load(t).Check("missing.schema.json", "f.shall.yaml", &tree.Node{Kind: tree.Map})
}
