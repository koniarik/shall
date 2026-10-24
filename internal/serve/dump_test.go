package serve

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"shall/internal/testfile"
)

func TestADumpHoldsEverySetAsTheServerAnswersIt(t *testing.T) {
	t.Attr("verifies", "R74")
	files := with(coffee(), map[string]string{
		"tea/shall.yaml":         "name: tea\nformat: 1\n",
		"tea/mission.shall.yaml": mission,
		"one/shall.yaml":         "name: twin\nformat: 1\n",
		"one/mission.shall.yaml": mission,
		"two/shall.yaml":         "name: twin\nformat: 1\n",
		"two/mission.shall.yaml": mission,
	})
	root := testfile.Write(t, files)
	var out bytes.Buffer
	if err := Dump(root, &out); err != nil {
		t.Fatal(err)
	}
	var dump struct {
		Sets []struct {
			Name  string            `json:"name"`
			Files []json.RawMessage `json:"files"`
		} `json:"sets"`
		Clashes  []clashJSON     `json:"clashes"`
		Findings json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &dump); err != nil {
		t.Fatal(err)
	}
	h := New(root, true).Handler()
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &whole); err != nil {
		t.Fatal(err)
	}
	var sets []map[string]json.RawMessage
	if err := json.Unmarshal(whole["sets"], &sets); err != nil {
		t.Fatal(err)
	}
	var names []string
	for i, s := range dump.Sets {
		names = append(names, s.Name)
		// The set as the server answers it, but for its files, which the
		// dump holds in full.
		_, body := getFrom(t, h, "/api/sets/"+url.PathEscape(s.Name), "localhost:7180")
		var served map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &served); err != nil {
			t.Fatal(err)
		}
		delete(served, "version")
		delete(served, "files")
		delete(sets[i], "files")
		got, _ := json.Marshal(sets[i])
		want, _ := json.Marshal(served)
		if string(got) != string(want) {
			t.Errorf("set %s is dumped as\n%s\nwhere the server answers\n%s", s.Name, got, want)
		}
		for _, f := range s.Files {
			var file struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(f, &file); err != nil {
				t.Fatal(err)
			}
			_, body := getFrom(t, h, "/api/sets/"+url.PathEscape(s.Name)+"/files/"+file.Path, "localhost:7180")
			if got, want := unversioned(t, f), unversioned(t, []byte(body)); got != want {
				t.Errorf("%s of %s is dumped as\n%s\nwhere the server answers\n%s", file.Path, s.Name, got, want)
			}
		}
	}
	if strings.Join(names, " ") != "coffee tea" {
		t.Errorf("the dump holds sets %v, want coffee and tea, the two served", names)
	}
	if len(dump.Clashes) != 1 || dump.Clashes[0].Name != "twin" {
		t.Errorf("the dump's clashes = %+v, want the name twin, which two sets share", dump.Clashes)
	}
	if strings.Contains(out.String(), `"version"`) {
		t.Error("the dump carries a version, which only a running server's answers have")
	}
}

func TestADumpOfNoSetFails(t *testing.T) {
	t.Attr("verifies", "R64.1")
	var out bytes.Buffer
	if err := Dump(t.TempDir(), &out); err == nil || !strings.Contains(err.Error(), "no shall.yaml") || out.Len() != 0 {
		t.Errorf("a dump of a directory with no set = %v, %q; want it to fail, writing nothing", err, out.String())
	}
}
