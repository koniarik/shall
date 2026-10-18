package tree

import (
	"errors"
	"strings"
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

func parse(t *testing.T, src string) *Node {
	t.Helper()
	n, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestScalarsOfEveryKind(t *testing.T) {
	n := parse(t, "int: 7\nnegative: -5\nfloat: 1.5\nhuge: 18446744073709551615\nyes: true\nnone: null\ntilde: ~\nlines: |\n  one\n  two\ntext: plain\n")
	for key, want := range map[string]float64{"int": 7, "negative": -5, "float": 1.5, "huge": 18446744073709551615} {
		if got, ok := n.Get(key).Number(); !ok || got != want {
			t.Errorf("%s = %v, %v; want the number %v", key, got, ok, want)
		}
	}
	if v, ok := n.Get("yes").Value.(bool); !ok || !v {
		t.Errorf("yes = %#v, want true", n.Get("yes").Value)
	}
	for _, key := range []string{"none", "tilde"} {
		if n.Get(key).Kind != Null {
			t.Errorf("%s is %v, want null", key, n.Get(key).Kind)
		}
	}
	if s, ok := n.Get("lines").Str(); !ok || s != "one\ntwo\n" {
		t.Errorf("lines = %q, want the literal block", s)
	}
	if _, ok := n.Get("text").Number(); ok {
		t.Error("text is a number")
	}
	if _, ok := n.Get("int").Str(); ok {
		t.Error("a number is text")
	}
}

func TestNodesSayWhatTheyAre(t *testing.T) {
	n := parse(t, "s: plain\ni: 3\nm: {a: 1}\nl: [1]\nz: null\n")
	for key, want := range map[string]string{"s": `"plain"`, "i": "3", "m": "a mapping", "l": "a list", "z": "null", "missing": "null"} {
		if got := n.Get(key).Text(); got != want {
			t.Errorf("%s.Text() = %s, want %s", key, got, want)
		}
	}
}

func TestLookupsOnOtherShapes(t *testing.T) {
	n := parse(t, "list: [a]\n")
	list := n.Get("list")
	if list.Get("a") != nil || list.Pair("a") != nil {
		t.Error("a list answered a key")
	}
	var none *Node
	if none.Get("a") != nil || none.Pair("a") != nil {
		t.Error("a missing node answered a key")
	}
	if _, ok := none.Str(); ok {
		t.Error("a missing node is text")
	}
	if _, ok := none.Number(); ok {
		t.Error("a missing node is a number")
	}
	if _, ok := list.Str(); ok {
		t.Error("a list is text")
	}
	if p := n.Pair("list"); p == nil || p.Key != "list" || p.KeyLine != 1 {
		t.Errorf("Pair(list) = %+v, want the pair list at line 1", p)
	}
}

func TestAnErrorNamesItsLine(t *testing.T) {
	if got := (&Error{Line: 3, Message: "broken"}).Error(); got != "line 3: broken" {
		t.Errorf("Error() = %q, want %q", got, "line 3: broken")
	}
	if e := mustFail(t, "a: [1\n"); e.Line < 1 || e.Message == "" {
		t.Errorf("error = %+v, want a line and a message", e)
	}
}

func TestASecondDocumentAfterAMarker(t *testing.T) {
	if e := mustFail(t, "a: 1\n---\nb: 2\n"); e.Line != 2 {
		t.Errorf("error line = %d, want 2, where the second document starts", e.Line)
	}
}

func TestMappingsInSequences(t *testing.T) {
	n := parse(t, "- a: 1\n- {b: 2, c: 3}\n")
	if n.Kind != Seq || len(n.Items) != 2 {
		t.Fatalf("got %+v, want a list of two items", n)
	}
	if v, _ := n.Items[0].Get("a").Number(); v != 1 || n.Items[0].Line != 1 {
		t.Errorf("first item = %+v, want a: 1 at line 1", n.Items[0])
	}
	if len(n.Items[1].Pairs) != 2 {
		t.Errorf("second item = %+v, want two pairs", n.Items[1])
	}
}

func TestNestedTagsAreRefused(t *testing.T) {
	if e := mustFail(t, "a:\n  - !!str x\n"); e.Line != 2 || !strings.Contains(e.Message, "tags") {
		t.Errorf("error = %+v, want the tag refused at line 2", e)
	}
	if e := mustFail(t, "a:\n  b: !!str x\n"); e.Line != 2 {
		t.Errorf("error = %+v, want line 2", e)
	}
}
