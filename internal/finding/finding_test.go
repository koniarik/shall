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
