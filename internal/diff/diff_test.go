package diff

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shall/internal/finding"
	"shall/internal/testfile"
)

const mission = `title: M
mission: m
ranking: [O90]
items:
  - {id: O90, title: A, text: t, measure: m}
  - {id: O91, title: B, text: t, measure: m}
  - id: S92
    title: A
    text: t
    children:
      - {id: N92.1, title: A, text: t, serves: [O90], status: hypothesis}
      - {id: N92.2, title: B, text: t, serves: [O90], status: hypothesis}
`

// req returns a requirement written on one line.
func req(id, title, text, rest string) string {
	return "  - {id: " + id + ", title: " + title + ", text: " + text + ", serves: [N92.1], status: draft" + rest + "}\n"
}

// compare reads dir at base and head and compares them.
func compare(t *testing.T, dir, base, head string) *Result {
	t.Helper()
	r, err := Between(dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// items returns each item change as id:change, with its moves and the
// names of its fields.
func items(r *Result) string {
	var out []string
	for _, s := range r.Sets {
		for _, it := range s.Items {
			x := it.ID + ":" + it.Change
			if it.MovedFrom != "" {
				x += "<" + it.MovedFrom
			}
			for _, f := range it.Fields {
				x += "/" + f.Field
			}
			if it.EffectiveStatus != nil {
				x += "/effective"
			}
			out = append(out, x)
		}
	}
	return strings.Join(out, " ")
}

// text returns the diff as WriteText writes it, for a failure message.
func text(r *Result) string {
	var b bytes.Buffer
	if err := WriteText(&b, r); err != nil {
		return err.Error()
	}
	return b.String()
}

func TestItemsComparedById(t *testing.T) {
	t.Attr("verifies", "R38")
	t.Attr("verifies", "R38.1")
	t.Attr("verifies", "R38.3")
	t.Attr("verifies", "R38.4")
	dir := testfile.Repo(t)
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml": "title: A\nitems:\n" + req("R1", "One", "The tool SHALL do one thing.", "") +
			req("R2", "Two", "The tool SHALL do two.", "") + req("R3", "Three", "The tool SHALL do three.", "") +
			req("R5", "Five", "The tool SHALL do five.", "") + req("R6", "Six", "The tool SHALL do six.", ""),
		"b.shall.yaml": "title: B\nitems:\n" + req("R10", "Ten", "The tool SHALL do ten.", ""),
	})
	head := testfile.Commit(t, dir, map[string]string{
		"a.shall.yaml": "title: A\nitems:\n" + req("R3", "Three", "The tool SHALL do three.", "") +
			strings.Replace(req("R1", "One", "The tool SHALL do one other thing.", ""), "serves: [N92.1]", "serves: [N92.1, N92.2]", 1) +
			req("R4", "Four", "The tool SHALL do four.", "") + "retired:\n  - {id: R6, title: Six}\n",
		"b.shall.yaml": "title: B\nitems:\n" + req("R10", "Ten", "The tool SHALL do ten.", "") + req("R2", "Two", "The tool SHALL do two.", ""),
	})
	r := compare(t, dir, base, head)
	want := "R1:changed/text/serves R4:added R2:changed<a.shall.yaml R5:removed R6:retired"
	if got := items(r); got != want {
		t.Errorf("items = %s, want %s", got, want)
	}
	if got, want := r.Totals, (Totals{Changed: 2, Added: 1, Removed: 1, Retired: 1, Moved: 1}); got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}
	r1 := r.Sets[0].Items[0]
	if r1.Fields[0].Old != "The tool SHALL do one thing." || r1.Fields[0].New != "The tool SHALL do one other thing." {
		t.Errorf("R1's text = %+v, want the old and the new text whole", r1.Fields[0])
	}
	if old, ok := r1.Fields[1].Old.([]any); !ok || len(old) != 1 || old[0] != "N92.1" {
		t.Errorf("R1's old serves = %#v, want the list [N92.1]", r1.Fields[1].Old)
	}
}

func TestEffectiveStatusOfAParent(t *testing.T) {
	t.Attr("verifies", "R38.2")
	dir := testfile.Repo(t)
	chapter := func(child string) string {
		return "title: A\nitems:\n  - id: R1\n    title: One\n    text: The tool SHALL work.\n    serves: [N92.1]\n    status: implemented\n" +
			"    children:\n      - {id: R1.1, title: Part, text: The tool SHALL work in part., status: " + child + "}\n"
	}
	base := testfile.Commit(t, dir, map[string]string{"shall.yaml": "name: x\nformat: 1\n", "mission.shall.yaml": mission, "a.shall.yaml": chapter("implemented")})
	head := testfile.Commit(t, dir, map[string]string{"a.shall.yaml": chapter("draft")})
	r := compare(t, dir, base, head)
	if got, want := items(r), "R1:changed/effective R1.1:changed/status"; got != want {
		t.Errorf("items = %s, want %s: the parent's effective status, the child's own status alone", got, want)
	}
	if e := r.Sets[0].Items[0].EffectiveStatus; e == nil || e.Old != "implemented" || e.New != "draft" {
		t.Errorf("R1's effective status = %+v, want implemented, then draft", e)
	}
}

func TestSetsChaptersAndFindings(t *testing.T) {
	t.Attr("verifies", "R39")
	t.Attr("verifies", "R39.1")
	t.Attr("verifies", "R39.2")
	t.Attr("verifies", "R39.3")
	t.Attr("verifies", "R39.4")
	dir := testfile.Repo(t)
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":              "name: x\nformat: 1\npath: main\n",
		"main/mission.shall.yaml": mission,
		"main/a.shall.yaml":       "title: A\nitems:\n  - {id: R1, title: One, text: The tool SHALL work., status: draft}\n",
		"main/c.shall.yaml":       "title: C\nitems: []\n",
	})
	head := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":                "name: y\nformat: 1\npath: main\ndepends:\n  company:\n    path: std\n",
		"main/m/mission.shall.yaml": strings.Replace(strings.Replace(mission, "mission: m", "mission: n", 1), "ranking: [O90]", "ranking: [O91, O90]", 1),
		"main/a.shall.yaml":         "title: Alpha\nitems:\n" + req("R1", "One", "The tool SHALL work.", ""),
		"main/d.shall.yaml":         "title: D\nitems: []\n",
		"other/shall.yaml":          "name: z\nformat: 1\n",
		"other/mission.shall.yaml":  mission,
	}, "main/c.shall.yaml", "main/mission.shall.yaml")
	r := compare(t, dir, base, head)
	if len(r.Sets) != 2 || r.Sets[0].Manifest != "other/shall.yaml" || r.Sets[0].Change != "added" || r.Sets[1].Manifest != "shall.yaml" || r.Sets[1].Change != "changed" {
		t.Fatalf("diff =\n%s\nwant other/shall.yaml added and shall.yaml changed", text(r))
	}
	s := r.Sets[1]
	var fields []string
	for _, f := range s.Fields {
		fields = append(fields, f.Field)
	}
	if got, want := strings.Join(fields, " "), "name depends mission ranking"; got != want {
		t.Errorf("set-wide fields = %s, want %s", got, want)
	}
	var files []string
	for _, f := range s.Files {
		x := f.Path + ":" + f.Change
		if f.Mission {
			x += "(mission)"
		}
		files = append(files, x)
	}
	if got, want := strings.Join(files, " "), "main/a.shall.yaml:retitled main/c.shall.yaml:removed main/d.shall.yaml:added main/m/mission.shall.yaml:added(mission) main/mission.shall.yaml:removed(mission)"; got != want {
		t.Errorf("files = %s, want %s", got, want)
	}
	if len(s.Findings.New) != 1 || !strings.Contains(s.Findings.New[0].Message, "cannot be read") {
		t.Errorf("new findings = %+v, want the dependency that cannot be read", s.Findings.New)
	}
	if len(s.Findings.Gone) != 1 || !strings.Contains(s.Findings.Gone[0].Message, "serves nothing") {
		t.Errorf("gone findings = %+v, want R1 serving nothing", s.Findings.Gone)
	}
	if got := items(r); !strings.Contains(got, "R1:changed/serves") || !strings.Contains(got, "N92.1:added") {
		t.Errorf("items = %s, want R1's serves changed and the new set's items added", got)
	}
	back := compare(t, dir, head, base)
	if len(back.Sets) != 2 || back.Sets[0].Manifest != "other/shall.yaml" || back.Sets[0].Change != "removed" || back.Sets[0].Name != "z" {
		t.Errorf("diff the other way =\n%s\nwant other/shall.yaml, set z, removed", text(back))
	}
}

func TestTheMissionFileNamedAsSuch(t *testing.T) {
	t.Attr("verifies", "R39.3")
	dir := testfile.Repo(t)
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": strings.Replace(mission, "title: M", "title: Why The Machine Exists", 1),
	})
	head := testfile.Commit(t, dir, map[string]string{
		"mission.shall.yaml": strings.Replace(mission, "title: M", "title: Why The Coffee Machine Exists", 1),
	})
	r := compare(t, dir, base, head)
	if f := r.Sets[0].Files; len(f) != 1 || !f[0].Mission || f[0].Change != "retitled" {
		t.Errorf("files = %+v, want the mission file retitled", f)
	}
	if got, want := text(r), "  mission.shall.yaml: mission file retitled Why The {+Coffee+} Machine Exists\n"; !strings.Contains(got, want) {
		t.Errorf("text =\n%s\nwant it to hold %q", got, want)
	}
}

func TestAFindingThatOnlyMoved(t *testing.T) {
	t.Attr("verifies", "R39.4")
	dir := testfile.Repo(t)
	// R1 serves nothing, and R3 serves R6, which is retired: two warnings,
	// the second naming the line that retires R6.
	unchanged := "  - {id: R1, title: One, text: The tool SHALL work., status: draft}\n" +
		strings.Replace(req("R3", "Three", "The tool SHALL do three.", ""), "serves: [N92.1]", "serves: [N92.1, R6]", 1) +
		"retired:\n  - {id: R6, title: Six}\n"
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: A\nitems:\n" + unchanged,
	})
	head := testfile.Commit(t, dir, map[string]string{
		"a.shall.yaml": "title: A\nitems:\n" + req("R2", "Two", "The tool SHALL start.", "") + unchanged,
	})
	r := compare(t, dir, base, head)
	if len(r.Sets) != 1 {
		t.Fatalf("diff =\n%s\nwant the one set changed", text(r))
	}
	if r.Sets[0].Findings != nil {
		t.Errorf("diff =\n%s\nwant no finding new or gone: each is the same, a line lower", text(r))
	}
	before, err := Read(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range before[0].Findings {
		found = found || strings.Contains(f.Message, "R6 is retired, at a.shall.yaml:")
	}
	if !found {
		t.Errorf("findings at the base = %+v, want R3's reference to the retired R6 among them", before[0].Findings)
	}
}

func TestRefsAndTheWorkingTree(t *testing.T) {
	t.Attr("verifies", "R37")
	t.Attr("verifies", "R37.1")
	t.Attr("verifies", "R37.2")
	t.Attr("verifies", "R37.3")
	t.Attr("verifies", "R37.4")
	dir := testfile.Repo(t)
	empty := testfile.Commit(t, dir, map[string]string{"README": "nothing yet\n"})
	first := testfile.Commit(t, dir, map[string]string{
		"doc/shall.yaml":         "name: x\nformat: 1\n",
		"doc/mission.shall.yaml": mission,
		"doc/a.shall.yaml":       "title: A\nitems:\n" + req("R1", "One", "The tool SHALL work.", ""),
		"other/shall.yaml":       "name: other\nformat: 1\n",
	})
	second := testfile.Commit(t, dir, map[string]string{"doc/a.shall.yaml": "title: A\nitems:\n" + req("R1", "One", "The tool SHALL work well.", "")})
	doc := filepath.Join(dir, "doc")

	if r := compare(t, doc, empty, first); r.Totals.Added != 6 || len(r.Sets) != 1 || r.Sets[0].Manifest != "shall.yaml" {
		t.Errorf("from before the directory existed: %d sets, totals %+v; want the one set under doc, all added", len(r.Sets), r.Totals)
	}
	if got, want := items(compare(t, doc, first, second)), "R1:changed/text"; got != want {
		t.Errorf("between two commits: %s, want %s", got, want)
	}
	testfile.WriteFile(t, filepath.Join(doc, "b.shall.yaml"), "title: B\nitems:\n"+req("R2", "Two", "The tool SHALL do two.", ""))
	if got, want := items(compare(t, doc, "HEAD", "")), "R2:added"; got != want {
		t.Errorf("the working tree against HEAD: %s, want %s, from a file git does not track", got, want)
	}
	if r := compare(t, doc, second, second); r.Changed() {
		t.Errorf("a commit against itself =\n%s\nwant nothing changed", text(r))
	}
	if _, err := Read(doc, "no-such-ref"); err == nil || !strings.HasSuffix(err.Error(), "no-such-ref names no commit of the repository at "+testfile.Git(t, dir, "rev-parse", "--show-toplevel")) {
		t.Errorf("an unknown ref: %v, want an error naming it and the repository, and no more", err)
	}
	tree := testfile.Git(t, dir, "rev-parse", "HEAD^{tree}")
	if _, err := Read(doc, tree); err == nil || !strings.HasSuffix(err.Error(), tree+" names no commit of the repository at "+testfile.Git(t, dir, "rev-parse", "--show-toplevel")+": expected commit type, but the object dereferences to tree type") {
		t.Errorf("a tree: %v, want an error naming it, with git's reason", err)
	}
	if _, err := Read(t.TempDir(), "HEAD"); err == nil {
		t.Error("reading a ref outside a repository gave no error")
	}
	if _, err := Read(filepath.Join(doc, "a.shall.yaml"), "HEAD"); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("reading a file: %v, want an error saying it is not a directory", err)
	}
	if err := os.RemoveAll(doc); err != nil {
		t.Fatal(err)
	}
	if r := compare(t, doc, "HEAD", ""); len(r.Sets) != 1 || r.Sets[0].Change != "removed" || r.Totals.Removed != 6 {
		t.Errorf("a directory deleted from the working tree =\n%s\nwant its set and all six items removed", text(r))
	}
	if _, err := Between(filepath.Join(doc, "deeper"), "HEAD", ""); err == nil || !strings.Contains(err.Error(), "at either ref") {
		t.Errorf("a directory at neither ref: %v, want an error saying no set is there", err)
	}
}

func TestSetFilesElsewhereInTheRepository(t *testing.T) {
	t.Attr("verifies", "R37.3")
	dir := testfile.Repo(t)
	chapter := func(text string) string { return "title: A\nitems:\n" + req("R1", "One", text, "") }
	first := testfile.Commit(t, dir, map[string]string{
		"doc/shall.yaml":          "name: x\nformat: 1\npath: ../reqs\n",
		"reqs/mission.shall.yaml": mission,
		"reqs/a.shall.yaml":       chapter("The tool SHALL work."),
		"src/main.go":             "package main\n",
	})
	doc := filepath.Join(dir, "doc")
	if r := compare(t, doc, "HEAD", ""); r.Changed() {
		t.Errorf("the working tree against HEAD =\n%s\nwant nothing changed", text(r))
	}
	second := testfile.Commit(t, dir, map[string]string{"reqs/a.shall.yaml": chapter("The tool SHALL work well.")})
	if got, want := items(compare(t, doc, first, second)), "R1:changed/text"; got != want {
		t.Errorf("between two commits: %s, want %s", got, want)
	}
}

func TestNothingReadFromOutsideTheRepository(t *testing.T) {
	t.Attr("verifies", "R37.3.1")
	dir := testfile.Repo(t)
	testfile.WriteFile(t, filepath.Join(dir, "..", "outside", "mission.shall.yaml"), mission)
	testfile.Commit(t, dir, map[string]string{"shall.yaml": "name: x\nformat: 1\npath: ../outside\n"})
	for _, ref := range []string{"HEAD", ""} {
		results, err := Read(dir, ref)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range results[0].Findings {
			found = found || f.Field == "path" && strings.Contains(f.Message, "leads out of the repository")
		}
		if results[0].Set != nil || !found {
			t.Errorf("at %q: set read %t, findings %+v; want none read and its path found leading out of the repository", ref, results[0].Set != nil, results[0].Findings)
		}
	}
}

func TestWhitespaceIsNoChange(t *testing.T) {
	t.Attr("verifies", "R37.7")
	dir := testfile.Repo(t)
	block := func(text string) string {
		return "title: \"Two  Colours\"\nitems:\n  - id: R1\n    title: One\n    text: |\n" + text + "    serves: [N92.1]\n    status: draft\n"
	}
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: Two Colours\nitems:\n" + req("R1", "One", "The tool SHALL work well.", ""),
	})
	head := testfile.Commit(t, dir, map[string]string{
		"mission.shall.yaml": strings.Replace(mission, "mission: m", "mission: |\n  m\n", 1),
		"a.shall.yaml":       block("      The tool SHALL\n      work   well.  \n"),
	})
	if r := compare(t, dir, base, head); r.Changed() {
		t.Errorf("a text rewrapped, a title spaced and a mission in a block =\n%s\nwant nothing changed", text(r))
	}
	later := testfile.Commit(t, dir, map[string]string{"a.shall.yaml": block("      The tool SHALL\n      work better.\n")})
	r := compare(t, dir, head, later)
	if got, want := items(r), "R1:changed/text"; got != want {
		t.Fatalf("items = %s, want %s", got, want)
	}
	if f := r.Sets[0].Items[0].Fields[0]; f.Old != "The tool SHALL\nwork   well.  \n" || f.New != "The tool SHALL\nwork better.\n" {
		t.Errorf("R1's text = %q to %q, want both as written", f.Old, f.New)
	}
}

func TestATopicsMarkdown(t *testing.T) {
	t.Attr("verifies", "R37.7")
	t.Attr("verifies", "R40.1.1")
	dir := testfile.Repo(t)
	topic := func(text string) string {
		var b strings.Builder
		b.WriteString("title: A\nitems:\n  - id: T1\n    title: Terms\n    text: |\n")
		for _, line := range strings.SplitAfter(text, "\n") {
			if strings.TrimSpace(line) != "" {
				b.WriteString("      ")
			}
			b.WriteString(line)
		}
		return b.String()
	}
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       topic("First paragraph\ngoes on.\n\nSecond.\n\n- one\n- two\n"),
	})
	rewrapped := testfile.Commit(t, dir, map[string]string{"a.shall.yaml": topic("First paragraph goes on.\n\nSecond.\n\n- one\n- two\n")})
	merged := testfile.Commit(t, dir, map[string]string{"a.shall.yaml": topic("First paragraph\ngoes on.\nSecond.\n\n- one\n- two\n")})
	joined := testfile.Commit(t, dir, map[string]string{"a.shall.yaml": topic("First paragraph\ngoes on.\n\nSecond.\n\n- one - two\n")})
	if r := compare(t, dir, base, rewrapped); r.Changed() {
		t.Errorf("a paragraph rewrapped =\n%s\nwant nothing changed: it reads as the same blocks", text(r))
	}
	for _, c := range []struct{ name, head, want string }{
		{"two paragraphs merged", merged, "      text: First paragraph goes on.[-↵↵-]{+↵+}Second. - one - …\n"},
		{"two list items joined", joined, "      text: … on. Second. - one[-↵-]{+·+}- two\n"},
	} {
		r := compare(t, dir, base, c.head)
		if got := items(r); got != "T1:changed/text" {
			t.Errorf("%s: items = %s, want T1's text changed", c.name, got)
		}
		if got := text(r); !strings.Contains(got, c.want) {
			t.Errorf("%s: text =\n%s\nwant it to hold %q", c.name, got, c.want)
		}
	}
}

func TestNothingChanged(t *testing.T) {
	t.Attr("verifies", "R37.5")
	dir := testfile.Repo(t)
	c := testfile.Commit(t, dir, map[string]string{"shall.yaml": "name: x\nformat: 1\n", "mission.shall.yaml": mission})
	r := compare(t, dir, c, "")
	if r.Changed() {
		t.Errorf("diff =\n%s\nwant nothing changed", text(r))
	}
	var b bytes.Buffer
	if err := WriteText(&b, r); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != "nothing changed\n" {
		t.Errorf("text = %q, want %q", got, "nothing changed\n")
	}
}

func TestChangedWordsMarked(t *testing.T) {
	t.Attr("verifies", "R40.1")
	link := func(url string) map[string]any { return map[string]any{"title": "Spec", "url": url} }
	cases := []struct {
		old, cur any
		want     string
	}{
		{"Two Colours", "Few Colours", "[-Two-]{+Few+} Colours"},
		{"one two three four five six seven eight nine ten", "one two three four five 6 seven eight nine ten",
			"… two three four five [-six-]{+6+} seven eight nine ten"},
		{"a b c d e f g h i j k l m n o", "a b c d X f g h i j k l m n Y",
			"a b c d [-e-]{+X+} f g h i … k l m n [-o-]{+Y+}"},
		{"one two three four five six", "1 two three four five six", "[-one-]{+1+} two three four five …"},
		{"a b c d", "a b y z", "a b [-c d-]{+y z+}"},
		{"a b c d", "a x y z", "[-a b c d-]{+a x y z+}"},
		{"The server builds pages", "Scripts read one answer", "[-The server builds pages-]{+Scripts read one answer+}"},
		{"", "new words", "{+new words+}"},
		{"old words", nil, "[-old words-]"},
		{[]any{"R19"}, []any{"R19", "R36"}, "[R19, {+R36+}]"},
		{nil, []any{"N1"}, "[{+N1+}]"},
		{[]any{"R19", link("https://x")}, []any{"R19", link("https://y")},
			"[R19, [-{title: Spec, url: https://x}-]{+{title: Spec, url: https://y}+}]"},
		{[]any{"one\ntwo", "three"}, []any{"one\ntwo", "four"}, "[one\ntwo, [-three-]{+four+}]"},
		{[]any{"", "a"}, []any{"", "b"}, "[, [-a-]{+b+}]"},
	}
	var changes []change
	for _, c := range cases {
		changes = append(changes, change{old: c.old, cur: c.cur})
	}
	for _, config := range []string{"", hostile} {
		path := filepath.Join(t.TempDir(), "gitconfig")
		testfile.WriteFile(t, path, config)
		t.Setenv("GIT_CONFIG_GLOBAL", path)
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		got, err := mark(changes)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range cases {
			if got[i] != c.want {
				t.Errorf("with the git config %q: %#v to %#v = %q, want %q", config, c.old, c.cur, got[i], c.want)
			}
		}
	}
}

// hostile is a git config that would change what git diff prints, were the
// options mark sets not over it.
const hostile = `[diff]
	noprefix = true
	mnemonicPrefix = true
	renames = copies
	algorithm = patience
	indentHeuristic = false
	suppressBlankEmpty = true
	external = false
	context = 1
	orderFile = /dev/null
[color]
	ui = always
	diff = always
`

func TestADiffToRead(t *testing.T) {
	t.Attr("verifies", "R40")
	dir := testfile.Repo(t)
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml":       "title: Two Colours\nitems:\n" + req("R1", "One", "The tool SHALL work.", "") + req("R2", "Two", "The tool SHALL do two.", ""),
	})
	head := testfile.Commit(t, dir, map[string]string{
		"a.shall.yaml": "title: Few Colours\nitems:\n" + req("R1", "One", "The tool SHALL work well.", ""),
		"b.shall.yaml": "title: B\nitems:\n" + req("R2", "Two", "The tool SHALL do two.", "") + req("R3", "Three", "The tool SHALL do three.", ""),
	})
	var b bytes.Buffer
	if err := WriteText(&b, compare(t, dir, base, head)); err != nil {
		t.Fatal(err)
	}
	want := `shall.yaml: set x
  a.shall.yaml: chapter retitled [-Two-]{+Few+} Colours
    R1 One: changed
      text: The tool SHALL [-work.-]{+work well.+}
  b.shall.yaml: chapter added
    R2 Two: changed, moved from a.shall.yaml
    R3 Three: added
2 items changed, 1 added, 0 removed, 0 retired, 1 moved
`
	if got := b.String(); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestEverythingADiffShows(t *testing.T) {
	t.Attr("verifies", "R40")
	r := &Result{
		Sets: []*SetDiff{
			{Manifest: "other/shall.yaml", Name: "z", Change: "added"},
			{
				Manifest: "shall.yaml", Name: "y", Change: "changed",
				Fields: []FieldChange{
					{Field: "name", Old: "x", New: "y"},
					{Field: "depends", New: map[string]any{"company": map[string]any{"path": "std", "rev": "v1"}}},
				},
				Files: []FileChange{
					{Path: "c.shall.yaml", Change: "removed", Title: &Values{Old: "C"}},
					{Path: "m/mission.shall.yaml", Mission: true, Change: "added"},
					{Path: "mission.shall.yaml", Mission: true, Change: "removed"},
				},
				Items: []ItemChange{{
					ID: "R1", Title: "One", File: "a.shall.yaml", Change: "changed",
					Fields:          []FieldChange{{Field: "links", New: []any{map[string]any{"url": "https://x", "title": "Spec"}}}},
					EffectiveStatus: &Values{Old: "implemented", New: "draft"},
				}},
				Findings: &Findings{
					New:  []finding.Finding{{Severity: finding.Error, File: "a.shall.yaml", Line: 3, ID: "R1", Field: "serves", Message: "names N9, which is not there"}},
					Gone: []finding.Finding{{Severity: finding.Warning, File: "shall.yaml", Message: "the set has no license"}},
				},
			},
		},
		Totals: Totals{Changed: 1},
	}
	var b bytes.Buffer
	if err := WriteText(&b, r); err != nil {
		t.Fatal(err)
	}
	want := `other/shall.yaml: set z: added
shall.yaml: set y
  name: [-x-]{+y+}
  depends: {+{company: {path: std, rev: v1}}+}
  a.shall.yaml
    R1 One: changed
      links: [{+{title: Spec, url: https://x}+}]
      effective status: [-implemented-]{+draft+}
  c.shall.yaml: chapter removed
  m/mission.shall.yaml: mission file added
  mission.shall.yaml: mission file removed
  findings: 1 new, 1 gone
    new: a.shall.yaml:3: error: R1 serves: names N9, which is not there
    gone: shall.yaml: warning: the set has no license
1 item changed, 0 added, 0 removed, 0 retired, 0 moved
`
	if got := b.String(); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestADiffForScripts(t *testing.T) {
	t.Attr("verifies", "R40.2")
	dir := testfile.Repo(t)
	because := ", rationale: because"
	base := testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		"a.shall.yaml": "title: A\nitems:\n" + req("R1", "One", "The tool SHALL work.", "") +
			req("R2", "Two", "The tool SHALL do two.", because) + req("R3", "Three", "The tool SHALL do three.", because),
	})
	head := testfile.Commit(t, dir, map[string]string{
		"a.shall.yaml": "title: A\nitems:\n" + req("R2", "Two", "The tool SHALL do two.", ", rationale: null") +
			req("R3", "Three", "The tool SHALL do three.", ""),
		"b.shall.yaml": "title: B\nitems:\n" + strings.Replace(req("R1", "One", "The tool SHALL work.", ""), "serves: [N92.1]", "serves: [N92.1, N92.2]", 1),
	})
	out, err := json.Marshal(compare(t, dir, base, head))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Base, Head string
		Sets       []struct {
			Manifest, Change string
			Items            []struct {
				ID, File, Change, MovedFrom string
				Fields                      []map[string]json.RawMessage
			}
			Findings map[string][]any
		}
		Totals Totals
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	byID := map[string]int{}
	for i, it := range got.Sets[0].Items {
		byID[it.ID] = i
	}
	r1, r2, r3 := got.Sets[0].Items[byID["R1"]], got.Sets[0].Items[byID["R2"]], got.Sets[0].Items[byID["R3"]]
	if got.Base != base || got.Head != head || r1.ID != "R1" || r1.File != "b.shall.yaml" || r1.MovedFrom != "a.shall.yaml" || got.Totals.Moved != 1 {
		t.Errorf("JSON = %s, want R1 moved from a.shall.yaml to b.shall.yaml", out)
	}
	if f := r1.Fields[0]; string(f["field"]) != `"serves"` || string(f["old"]) != `["N92.1"]` || string(f["new"]) != `["N92.1","N92.2"]` {
		t.Errorf("R1's serves in JSON = %s, want the old and new lists whole", f)
	}
	if f := r2.Fields[0]; r2.ID != "R2" || string(f["field"]) != `"rationale"` || string(f["new"]) != "null" {
		t.Errorf("R2's rationale written null = %s, want the new value null", f)
	}
	if f, ok := r3.Fields[0]["new"]; r3.ID != "R3" || ok {
		t.Errorf("R3's rationale removed = new %s, want no new value", f)
	}
	if _, ok := got.Sets[0].Findings["gone"]; ok || len(got.Sets[0].Findings["new"]) == 0 {
		t.Errorf("findings = %v, want those new and no list of gone ones", got.Sets[0].Findings)
	}
	if bytes.Contains(out, []byte(`"mission"`)) || bytes.Contains(out, []byte(`"effectiveStatus"`)) || bytes.Count(out, []byte(`"movedFrom"`)) != 1 {
		t.Errorf("JSON = %s, want what is empty left out", out)
	}
	none, err := json.Marshal(compare(t, dir, head, head))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(none, []byte(`"sets":[]`)) {
		t.Errorf("JSON with nothing changed = %s, want the sets an empty list", none)
	}
}

func TestARefReadsAsOnDisk(t *testing.T) {
	t.Attr("verifies", "R41")
	dir := testfile.Repo(t)
	chapter := func(id, title string) string {
		return "title: " + title + "\nitems:\n" + req(id, "One", "The tool SHALL work.", "")
	}
	testfile.WriteFile(t, filepath.Join(dir, "src", "a.yaml"), chapter("R1", "A"))
	testfile.WriteFile(t, filepath.Join(dir, "..", "elsewhere.yaml"), chapter("R9", "Elsewhere"))
	// Each link that fails on disk is found as it is there, and the one
	// leading out of the repository is not read, though its file is there.
	links := map[string]string{
		"a.shall.yaml":        "src/a.yaml",
		"out.shall.yaml":      "../elsewhere.yaml",
		"none.shall.yaml":     "src/none.yaml",
		"loop.shall.yaml":     "loop.shall.yaml",
		"dir.shall.yaml":      "src",
		"through.shall.yaml":  "src/a.yaml/x",
		"sub/deep.shall.yaml": "../../elsewhere.yaml",
	}
	found := map[string]string{
		"out.shall.yaml":      "leads out of the repository",
		"none.shall.yaml":     "no such file",
		"loop.shall.yaml":     "too many levels",
		"dir.shall.yaml":      "is a directory",
		"through.shall.yaml":  "not a directory",
		"sub/deep.shall.yaml": "leads out of the repository",
	}
	for link, target := range links {
		testfile.WriteFile(t, filepath.Join(dir, link), "")
		os.Remove(filepath.Join(dir, link))
		if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
			t.Skip("no symlinks here:", err)
		}
	}
	testfile.Commit(t, dir, map[string]string{
		"shall.yaml":         "name: x\nformat: 1\n",
		"mission.shall.yaml": mission,
		".gitattributes":     "b.shall.yaml export-ignore\nc.shall.yaml export-subst\n",
		"b.shall.yaml":       chapter("R2", "B"),
		"c.shall.yaml":       chapter("R3", "C $Format:%H$"),
	})
	if r := compare(t, dir, "HEAD", ""); r.Changed() {
		t.Errorf("HEAD against a working tree just committed =\n%s\nwant nothing changed", text(r))
	}
	results, err := Read(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"R1", "R2", "R3"} {
		if results[0].Set.Item(id) == nil {
			t.Errorf("%s at HEAD: missing, want it read through a link, or whatever the attributes say", id)
		}
	}
	if results[0].Set.Item("R9") != nil {
		t.Error("R9 at HEAD: read, want the link that leads to it out of the repository not read")
	}
	for file, message := range found {
		ok := false
		for _, f := range results[0].Findings {
			ok = ok || f.File == file && strings.Contains(f.Message, message)
		}
		if !ok {
			t.Errorf("findings at HEAD = %+v, want %s found to %s", results[0].Findings, file, message)
		}
	}
}

func TestATreeStaysInItsDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../outside", "a/../../outside", "/outside"} {
		if err := place(filepath.Join(dir, "in"), name, []byte("x")); err == nil {
			t.Errorf("a file named %s was written, want it refused", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "outside")); !os.IsNotExist(err) {
		t.Errorf("a file outside the directory: %v, want none", err)
	}
}
