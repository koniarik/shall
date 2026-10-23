package finding

import "testing"

func TestSortByFileThenLineKeepingOrder(t *testing.T) {
	t.Attr("verifies", "R3")
	fs := []Finding{
		{File: "b.yaml", Line: 2, Message: "b2"},
		{File: "a.yaml", Line: 9, Message: "a9"},
		{File: "b.yaml", Line: 1, Message: "b1 first"},
		{File: "a.yaml", Line: 9, Message: "a9 second"},
		{File: "b.yaml", Line: 1, Message: "b1 second"},
	}
	Sort(fs)
	var got []string
	for _, f := range fs {
		got = append(got, f.Message)
	}
	want := []string{"a9", "a9 second", "b1 first", "b1 second", "b2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %q, want %q", got, want)
		}
	}
}

func TestAFindingAsALine(t *testing.T) {
	t.Attr("verifies", "R3")
	for _, c := range []struct {
		f    Finding
		want string
	}{
		{Finding{Severity: Error, File: "a.shall.yaml", Line: 3, ID: "R1", Field: "serves", Message: "N9 is no item"}, "a.shall.yaml:3: error: R1 serves: N9 is no item"},
		{Finding{Severity: Warning, File: "shall.yaml", Message: "the set has no licence"}, "shall.yaml: warning: the set has no licence"},
	} {
		if got := c.f.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}
