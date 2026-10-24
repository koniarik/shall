package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"shall"
	"shall/internal/markdown"
	"shall/internal/schema"
	"shall/internal/testfile"
	"shall/internal/tree"
)

// formatTopics returns the text of every topic of the format's chapter of
// shall's own set, by id.
func formatTopics(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "doc", "requirements", "10-format.shall.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var walk func(*tree.Node)
	walk = func(list *tree.Node) {
		if list == nil || list.Kind != tree.Seq {
			return
		}
		for _, it := range list.Items {
			id, _ := it.Get("id").Str()
			if text, ok := it.Get("text").Str(); ok && strings.HasPrefix(id, "T") {
				out[id] = text
			}
			walk(it.Get("children"))
		}
	}
	walk(n.Get("items"))
	return out
}

// yamlExamples returns the YAML code blocks of a topic's Markdown.
func yamlExamples(text string) []string {
	var out []string
	for _, b := range markdown.Blocks(text) {
		if b.Type == "code" && b.Language == "yaml" {
			out = append(out, b.Text)
		}
	}
	return out
}

// indent puts two spaces before every line of src.
func indent(src string) string {
	return "  " + strings.ReplaceAll(strings.TrimRight(src, "\n"), "\n", "\n  ") + "\n"
}

// inASet checks items written as a format topic's example writes them, in a
// set that also holds what they name and do not write themselves: objective
// O1, and class S3 with its need N3.1. Objectives, classes and needs go in
// the mission file, other items in a chapter.
func inASet(t *testing.T, example string) []string {
	t.Helper()
	n, err := tree.Parse([]byte(example))
	if err != nil {
		t.Fatalf("the example is no YAML: %v\n%s", err, example)
	}
	var ids []string
	for _, it := range n.Items {
		id, _ := it.Get("id").Str()
		ids = append(ids, id)
	}
	mission := "title: M\nmission: m\nitems:\n"
	if !slices.Contains(ids, "O1") {
		mission += "  - {id: O1, title: No Dry Brews, text: No brew starts while the tank is empty., measure: dry brews}\n"
	}
	if !slices.Contains(ids, "S3") {
		mission += "  - id: S3\n    title: Office User\n    text: Wants coffee without looking after the machine.\n    children:\n" +
			"      - {id: N3.1, title: Know Before Brewing, text: Know before I brew that the tank needs water., serves: [O1], status: hypothesis}\n"
	}
	files := map[string]string{"shall.yaml": "name: x\nformat: 1\n"}
	if strings.ContainsAny(ids[0][:1], "OSN") {
		mission += indent(example)
	} else {
		files["c.shall.yaml"] = "title: C\nitems:\n" + indent(example)
	}
	files["mission.shall.yaml"] = mission
	var got []string
	for _, f := range run(t, testfile.Write(t, files)) {
		got = append(got, f.ID+" "+f.Field+": "+f.Message)
	}
	return got
}

func TestTheFormatsExamplesHold(t *testing.T) {
	t.Attr("verifies", "R2")
	t.Attr("verifies", "R21")
	topics := formatTopics(t)
	v, err := schema.Load(shall.Schema())
	if err != nil {
		t.Fatal(err)
	}
	checked := map[string]bool{}
	only := func(id string) string {
		t.Helper()
		checked[id] = true
		examples := yamlExamples(topics[id])
		if len(examples) != 1 {
			t.Fatalf("%s holds %d YAML examples, want 1", id, len(examples))
		}
		return examples[0]
	}

	// A kind's good and bad examples, together. A bad example breaks what its
	// comment says, and most break only what nothing checks, their wording
	// [T63.13]; the findings name those a rule catches.
	for id, want := range map[string][]string{
		"T63.7":  {`O2 measure: field "measure" is missing`},
		"T63.8":  nil,
		"T63.9":  nil,
		"T63.10": nil,
		"T63.11": nil,
	} {
		if got := inASet(t, only(id)); !slices.Equal(got, want) {
			t.Errorf("%s's examples give %q, want %q", id, got, want)
		}
	}

	// Fragments of a file, which fit the schema in a file around them.
	manifest, err := tree.Parse([]byte("name: x\nformat: 1\n" + only("T63.1")))
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Check("manifest.schema.json", "shall.yaml", manifest); len(got) != 0 {
		t.Errorf("T63.1's dependencies do not fit the schema: %+v", got)
	}
	chapter, err := tree.Parse([]byte("title: C\nitems:\n  - id: R1\n    title: A\n    text: The tool SHALL work.\n    status: draft\n" +
		indent(indent(only("T63.5")))))
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Check("chapter.schema.json", "c.shall.yaml", chapter); len(got) != 0 {
		t.Errorf("T63.5's links do not fit the schema: %+v", got)
	}

	// Whole sets, each file headed by its path; a file named again starts
	// the next set, which keeps the manifest.
	checked["T63.14"] = true
	var sets []map[string]string
	for _, example := range yamlExamples(topics["T63.14"]) {
		first, rest, _ := strings.Cut(example, "\n")
		path, ok := strings.CutPrefix(first, "# ")
		if !ok {
			t.Fatalf("a T63.14 example does not begin with its path: %q", first)
		}
		if len(sets) == 0 || sets[len(sets)-1][path] != "" {
			set := map[string]string{}
			if len(sets) > 0 {
				set["shall.yaml"] = sets[0]["shall.yaml"]
			}
			sets = append(sets, set)
		}
		sets[len(sets)-1][path] = rest
	}
	if len(sets) != 2 {
		t.Fatalf("T63.14 holds %d sets, want the smallest and the fuller", len(sets))
	}
	for i, files := range sets {
		if got := run(t, testfile.Write(t, files)); len(got) != 0 {
			t.Errorf("T63.14's set %d gives %+v, want no finding", i+1, got)
		}
	}

	// Every example of the format is checked here.
	for id, text := range topics {
		if len(yamlExamples(text)) > 0 && !checked[id] {
			t.Errorf("%s holds a YAML example this test does not check", id)
		}
	}
}
