package tree

import (
	"errors"
	"testing"
)

func mustFail(t *testing.T, src string) *Error {
	t.Helper()
	_, err := Parse([]byte(src))
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Parse(%q) error = %v, want an *Error", src, err)
	}
	return e
}

func TestParseKeepsLines(t *testing.T) {
	t.Attr("verifies", "R3")
	t.Attr("verifies", "R20")
	n, err := Parse([]byte("title: T\nrequirements:\n  - id: R1\n    text: one\n      two\n"))
	if err != nil {
		t.Fatal(err)
	}
	req := n.Get("requirements").Items[0]
	if req.Line != 3 {
		t.Errorf("requirement line = %d, want 3", req.Line)
	}
	text := req.Get("text")
	if s, _ := text.Str(); s != "one two" || text.Line != 4 {
		t.Errorf("text = %q at line %d, want \"one two\" at line 4", s, text.Line)
	}
}

func TestParseReportsTheErrorLine(t *testing.T) {
	t.Attr("verifies", "R3")
	if e := mustFail(t, "a: 1\nb: [1, 2\nc: 3\n"); e.Line != 3 {
		t.Fatalf("error line = %d, want 3", e.Line)
	}
}

func TestDuplicateKeysAreErrors(t *testing.T) {
	if e := mustFail(t, "a: 1\na: 2\n"); e.Line != 2 {
		t.Fatalf("error line = %d, want 2", e.Line)
	}
}

func TestRefusesAliases(t *testing.T) {
	mustFail(t, "a: &x 1\nb: *x\n")
}

func TestRefusesTags(t *testing.T) {
	if e := mustFail(t, "a: 1\nformat: !!str 1\n"); e.Line != 2 {
		t.Fatalf("error line = %d, want 2", e.Line)
	}
}

func TestRefusesASecondDocument(t *testing.T) {
	if e := mustFail(t, "mission: first\n---\nmission: second\n"); e.Line != 2 {
		t.Fatalf("error line = %d, want 2", e.Line)
	}
}

func TestIgnoresAByteOrderMark(t *testing.T) {
	n, err := Parse([]byte("\xEF\xBB\xBFname: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Get("name") == nil {
		t.Fatalf("keys = %+v, want name", n.Pairs)
	}
}

func TestIntegersAreInt64(t *testing.T) {
	n, err := Parse([]byte("format: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := n.Get("format").Value.(int64); !ok || v != 1 {
		t.Fatalf("format = %#v, want int64 1", n.Get("format").Value)
	}
}

func TestEmptyDocumentIsNull(t *testing.T) {
	n, err := Parse([]byte(""))
	if err != nil || n.Kind != Null {
		t.Fatalf("got %v, %v; want a null node", n, err)
	}
}
