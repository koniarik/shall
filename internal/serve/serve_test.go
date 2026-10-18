package serve

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mission = `mission: Coffee without anyone looking after the machine.
ranking: [O1]
objectives:
  - {id: O1, title: No Dry Brews, text: No brew starts dry., measure: dry brews}
stakeholders:
  - {id: S1, title: Office User, text: Wants coffee.}
needs:
  - {id: N1, title: Know Before Brewing, stakeholder: S1, text: Know the tank is empty., serves: [O1], status: validated, notes: [Heard twice.]}
`

const tank = `title: Water Tank
requirements:
  - id: R1
    title: Refill Lamp
    text: When the tank becomes empty, the machine SHALL light the refill lamp.
    serves: [N1]
    status: verified
    children:
      - {id: R1.1, title: Lamp Seen, text: The machine SHALL show the lamp., status: implemented}
    retired:
      - {id: R1.2, title: Refill Beep}
  - id: R2
    title: Dry Lock
    text: The machine SHALL refuse to brew dry.
    serves: [R1, R1.2, R9]
    status: approved
retired:
  - {id: R3, title: Water Handling}
`

const brew = `title: Brewing
requirements:
  - {id: R4, title: Brew Button, text: When pressed the machine SHALL brew., serves: [N1], status: draft, rationale: One press is all it takes., notes: [Seen in the trial.], links: [{url: "https://ci.example.com/runs/1", title: Trial run}]}
  - {id: R5, title: Brew Lock, text: While brewing the machine SHALL lock the lid., serves: [R4], status: draft}
`

func coffee() map[string]string {
	return map[string]string{
		"shall.yaml":         "name: coffee\nformat: 1\n",
		"mission.shall.yaml": mission,
		"b-tank.shall.yaml":  tank,
		"a-brew.shall.yaml":  brew,
	}
}

func with(files map[string]string, more map[string]string) map[string]string {
	out := maps.Clone(files)
	maps.Copy(out, more)
	return out
}

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), src)
	}
	return root
}

func writeFile(t *testing.T, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// get requests path from a local server for root and returns the status and
// body.
func get(t *testing.T, root, path string) (int, string) {
	t.Helper()
	return getFrom(t, New(root, true).Handler(), path, "localhost:7180")
}

func getFrom(t *testing.T, h http.Handler, path, host string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// answer requests path and decodes its JSON answer into v, failing unless
// the status is want.
func answer(t *testing.T, root, path string, want int, v any) {
	t.Helper()
	code, body := get(t, root, path)
	if code != want {
		t.Fatalf("%s: status %d, want %d: %s", path, code, want, body)
	}
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("%s: %v: %s", path, err, body)
	}
}

func setOf(t *testing.T, root string) setAnswer {
	t.Helper()
	var a setAnswer
	answer(t, root, "/api/sets/coffee", http.StatusOK, &a)
	return a
}

func file(t *testing.T, root, path string) fileAnswer {
	t.Helper()
	a := fileAnswer{fileJSON: &fileJSON{}}
	answer(t, root, "/api/sets/coffee/files/"+path, http.StatusOK, &a)
	return a
}

func item(t *testing.T, root, id string) itemAnswer {
	t.Helper()
	a := itemAnswer{itemJSON: &itemJSON{}}
	answer(t, root, "/api/sets/coffee/items/"+id, http.StatusOK, &a)
	return a
}

func ids(items []*itemJSON) string {
	var out []string
	for _, it := range items {
		out = append(out, it.ID)
	}
	return strings.Join(out, " ")
}

func TestThePageAtEveryAddress(t *testing.T) {
	t.Attr("verifies", "R26.7")
	t.Attr("verifies", "R27.3")
	root := write(t, coffee())
	for _, path := range []string{"/", "/coffee/", "/coffee/R1.1", "/coffee/R99"} {
		code, body := get(t, root, path)
		if code != http.StatusOK || !strings.Contains(body, `<script src="/-/static/app.js" defer></script>`) {
			t.Errorf("%s: status %d, %.80q", path, code, body)
		}
	}
}

func TestStartPageListsEverySetByName(t *testing.T) {
	t.Attr("verifies", "R25")
	t.Attr("verifies", "R25.4")
	root := write(t, map[string]string{
		"a/shall.yaml":           "name: coffee\nformat: 1\n",
		"a/mission.shall.yaml":   mission,
		"b/c/shall.yaml":         "name: tea\nformat: 1\n",
		"b/c/mission.shall.yaml": mission,
	})
	var a setsAnswer
	answer(t, root, "/api/sets", http.StatusOK, &a)
	if len(a.Sets) != 2 || a.Sets[0].Name != "coffee" || a.Sets[1].Name != "tea" || a.Sets[1].Manifest != "b/c/shall.yaml" {
		t.Errorf("sets = %+v", a.Sets)
	}
}

func TestClashingNamesAreServedUnderNeither(t *testing.T) {
	t.Attr("verifies", "R25.5")
	root := write(t, map[string]string{
		"a/shall.yaml":         "name: coffee\nformat: 1\n",
		"a/mission.shall.yaml": mission,
		"b/shall.yaml":         "name: coffee\nformat: 1\n",
		"b/mission.shall.yaml": mission,
	})
	var a setsAnswer
	answer(t, root, "/api/sets", http.StatusOK, &a)
	if len(a.Sets) != 0 || len(a.Clashes) != 1 || a.Clashes[0].Name != "coffee" ||
		strings.Join(a.Clashes[0].Manifests, " ") != "a/shall.yaml b/shall.yaml" {
		t.Errorf("answer = %+v", a)
	}
	var e errorAnswer
	answer(t, root, "/api/sets/coffee", http.StatusNotFound, &e)
}

func TestALocalServerAnswersOnlyToLoopbackNames(t *testing.T) {
	t.Attr("verifies", "R25.2.1")
	root := write(t, coffee())
	local := New(root, true).Handler()
	for _, host := range []string{"localhost:7180", "127.0.0.1:7180", "[::1]:7180", "localhost"} {
		if code, _ := getFrom(t, local, "/api/sets", host); code != http.StatusOK {
			t.Errorf("host %s: status = %d, want 200", host, code)
		}
	}
	for _, host := range []string{"rebound.example:7180", "192.168.1.5:7180", ""} {
		if code, _ := getFrom(t, local, "/api/sets", host); code != http.StatusForbidden {
			t.Errorf("host %q: status = %d, want 403", host, code)
		}
	}
	if code, _ := getFrom(t, New(root, false).Handler(), "/api/sets", "build-box.lan:7180"); code != http.StatusOK {
		t.Errorf("a server given a network address refused another host: status %d", code)
	}
}

func TestALoopbackListenerMakesTheServerLocal(t *testing.T) {
	t.Attr("verifies", "R25.2.1")
	root := write(t, coffee())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)
	go func() { done <- Run(ctx, root, ln) }()
	defer func() {
		cancel()
		<-done
	}()
	req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+"/api/sets", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "rebound.example"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestOnlyWhatItBuilds(t *testing.T) {
	t.Attr("verifies", "R33")
	root := write(t, with(coffee(), map[string]string{"secret.txt": "keep out"}))
	for _, path := range []string{"/-/static/app.js", "/api/sets/coffee/files/a-brew.shall.yaml"} {
		if code, _ := get(t, root, path); code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, code)
		}
	}
	for _, path := range []string{
		"/-/static/../secret.txt",
		"/-/static/%2e%2e/secret.txt",
		"/api/sets/coffee/files/../secret.txt",
		"/api/sets/coffee/files/%2e%2e%2fsecret.txt",
		"/api/sets/coffee/files/secret.txt",
		"/api/sets/coffee/files/" + filepath.ToSlash(root) + "/a-brew.shall.yaml",
		"/secret.txt",
	} {
		code, body := get(t, root, path)
		if strings.Contains(body, "keep out") || (code == http.StatusOK && !strings.Contains(body, "<!doctype html>")) {
			t.Errorf("%s: status = %d, body %q", path, code, body)
		}
	}
}

func TestEveryAnswerIsJSONWithItsVersion(t *testing.T) {
	t.Attr("verifies", "R36")
	root := write(t, coffee())
	for _, path := range []string{
		"/api/sets", "/api/sets/coffee", "/api/sets/coffee/files/a-brew.shall.yaml",
		"/api/sets/coffee/items/R1", "/api/sets/coffee/resolve?id=R1",
	} {
		var a struct {
			Version string `json:"version"`
		}
		answer(t, root, path, http.StatusOK, &a)
		if a.Version == "" {
			t.Errorf("%s: no version", path)
		}
	}
}

func TestOneDocumentInOrder(t *testing.T) {
	t.Attr("verifies", "R26")
	t.Attr("verifies", "R26.9")
	t.Attr("verifies", "R26.10")
	root := write(t, coffee())
	a := setOf(t, root)
	var paths []string
	for _, f := range a.Files {
		paths = append(paths, f.Kind+":"+f.Path)
	}
	if got := strings.Join(paths, " "); got != "mission:mission.shall.yaml chapter:a-brew.shall.yaml chapter:b-tank.shall.yaml" {
		t.Errorf("files = %s", got)
	}
	m := a.Files[0]
	if m.Mission != "Coffee without anyone looking after the machine." || ids(m.Objectives) != "O1" || ids(m.Stakeholders) != "S1" || ids(m.Needs) != "N1" {
		t.Errorf("mission = %+v", m)
	}
	tank := file(t, root, "b-tank.shall.yaml")
	if ids(tank.Requirements) != "R1 R2" || ids(tank.Requirements[0].Children) != "R1.1" {
		t.Errorf("chapter = %s, children %s", ids(tank.Requirements), ids(tank.Requirements[0].Children))
	}
}

func TestStatementsShown(t *testing.T) {
	t.Attr("verifies", "R26.1")
	root := write(t, coffee())
	r4 := file(t, root, "a-brew.shall.yaml").Requirements[0]
	if r4.ID != "R4" || r4.Title != "Brew Button" || r4.Text != "When pressed the machine SHALL brew." {
		t.Errorf("R4 = %+v", r4)
	}
}

func TestChaptersComeWithoutTheirItems(t *testing.T) {
	t.Attr("verifies", "R26.4")
	t.Attr("verifies", "R26.5")
	root := write(t, coffee())
	a := setOf(t, root)
	brew, tank := a.Files[1], a.Files[2]
	if brew.Title != "Brewing" || brew.Counts != (counts{Items: 2}) || tank.Counts != (counts{Items: 3, Findings: 2}) {
		t.Errorf("chapters = %+v, %+v", brew, tank)
	}
	if brew.Requirements != nil || tank.Requirements != nil {
		t.Error("the set's answer holds a chapter's items")
	}
}

func TestAPreviewOrBacklinkCanShowAnItemOfAFoldedChapter(t *testing.T) {
	t.Attr("verifies", "R26.5")
	root := write(t, coffee())
	if it := item(t, root, "R1"); it.Title != "Refill Lamp" {
		t.Errorf("R1 = %+v", it)
	}
}

func TestStatusesShown(t *testing.T) {
	t.Attr("verifies", "R26.3")
	root := write(t, coffee())
	r1, r2 := item(t, root, "R1"), item(t, root, "R2")
	if r1.Status != "verified" || r1.EffectiveStatus != "implemented" || r2.Status != "approved" || r2.EffectiveStatus != "approved" {
		t.Errorf("R1 %s/%s, R2 %s/%s", r1.Status, r1.EffectiveStatus, r2.Status, r2.EffectiveStatus)
	}
	if n1 := item(t, root, "N1"); n1.Status != "validated" || n1.EffectiveStatus != "" {
		t.Errorf("N1 %s/%s", n1.Status, n1.EffectiveStatus)
	}
}

func TestBrokenFilesInPlace(t *testing.T) {
	t.Attr("verifies", "R26.8")
	root := write(t, with(coffee(), map[string]string{"ab-broken.shall.yaml": "title: [oops\n"}))
	a := setOf(t, root)
	if len(a.Files) != 4 || a.Files[2].Path != "ab-broken.shall.yaml" || !a.Files[2].Broken ||
		len(a.Files[2].Findings) != 1 || a.Files[2].Findings[0].Severity != "error" {
		t.Fatalf("files = %+v", a.Files)
	}
	if a.Files[3].Title != "Water Tank" {
		t.Error("the rest of the set is missing")
	}
}

func TestIdsAreResolved(t *testing.T) {
	t.Attr("verifies", "R27")
	t.Attr("verifies", "R27.4")
	t.Attr("verifies", "R27.5")
	root := write(t, coffee())
	got := item(t, root, "R2").Serves
	want := []refJSON{
		{ID: "R1", State: "item", Set: "coffee", Title: "Refill Lamp", File: "b-tank.shall.yaml"},
		{ID: "R1.2", State: "retired", Set: "coffee", Title: "Refill Beep"},
		{ID: "R9", State: "unknown"},
	}
	if len(got) != len(want) {
		t.Fatalf("serves = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("serves[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if s := item(t, root, "N1").Stakeholder; s == nil || *s != (refJSON{ID: "S1", State: "item", Set: "coffee", Title: "Office User", File: "mission.shall.yaml"}) {
		t.Errorf("stakeholder = %+v", s)
	}
}

func TestIdsOfAnotherServedSetAreResolved(t *testing.T) {
	t.Attr("verifies", "R27")
	root := write(t, map[string]string{
		".git/HEAD":                  "",
		"std/shall.yaml":             "name: standard\nformat: 1\n",
		"std/mission.shall.yaml":     mission,
		"product/shall.yaml":         "name: coffee\nformat: 1\ndepends:\n  company:\n    path: std\n",
		"product/mission.shall.yaml": "mission: m\nstakeholders:\n  - {id: S1, title: A, text: t}\nneeds:\n  - {id: N1, title: A, text: t, stakeholder: S1, serves: [mission], status: hypothesis, mirrors: company/N1}\n",
	})
	n1 := item(t, root, "N1")
	want := refJSON{ID: "company/N1", State: "item", Set: "standard", ItemID: "N1", Title: "Know Before Brewing", File: "std/mission.shall.yaml"}
	if n1.Mirrors == nil || *n1.Mirrors != want {
		t.Errorf("mirrors = %+v, want %+v", n1.Mirrors, want)
	}
	if len(n1.Serves) != 1 || n1.Serves[0] != (refJSON{ID: "mission", State: "mission"}) {
		t.Errorf("serves = %+v", n1.Serves)
	}
}

func TestHoverPreview(t *testing.T) {
	t.Attr("verifies", "R27.1")
	root := write(t, coffee())
	r1 := item(t, root, "R1")
	if r1.Title != "Refill Lamp" || r1.Text != "When the tank becomes empty, the machine SHALL light the refill lamp." {
		t.Errorf("R1 = %+v", r1)
	}
}

func TestBacklinks(t *testing.T) {
	t.Attr("verifies", "R27.2")
	t.Attr("verifies", "R27.2.1")
	root := write(t, coffee())
	var got []string
	for _, n := range item(t, root, "N1").NamedBy {
		got = append(got, n.Field+":"+n.ID+":"+n.Title)
	}
	if strings.Join(got, " ") != "serves:R4:Brew Button serves:R1:Refill Lamp" {
		t.Errorf("namedBy = %v", got)
	}
}

func TestJumpToAnId(t *testing.T) {
	t.Attr("verifies", "R28")
	t.Attr("verifies", "R28.1")
	t.Attr("verifies", "R28.2")
	root := write(t, with(coffee(), map[string]string{
		"tea/shall.yaml":         "name: tea\nformat: 1\n",
		"tea/mission.shall.yaml": mission,
	}))
	for _, c := range []struct {
		id, state, set, itemID, message string
	}{
		{"R1.1", "item", "coffee", "", ""},
		{" r4 ", "item", "coffee", "R4", ""},
		{"tea/N1", "item", "tea", "N1", ""},
		{"R7", "unknown", "", "", "No item has the id R7."},
		{"R1.2", "retired", "coffee", "", "R1.2 was retired at b-tank.shall.yaml:11."},
		{"R3", "retired", "coffee", "", "R3 was retired at b-tank.shall.yaml:18."},
		{"milk/N1", "unknown", "", "", "No set named milk is served here."},
	} {
		var a resolveAnswer
		answer(t, root, "/api/sets/coffee/resolve?id="+strings.ReplaceAll(c.id, " ", "+"), http.StatusOK, &a)
		if a.Ref.State != c.state || a.Ref.Set != c.set || a.Ref.ItemID != c.itemID || a.Message != c.message {
			t.Errorf("resolve %q = %+v %q, want %s in %q as %q, %q", c.id, a.Ref, a.Message, c.state, c.set, c.itemID, c.message)
		}
	}
}

func TestAnItemThatIsNotThere(t *testing.T) {
	root := write(t, coffee())
	var e errorAnswer
	answer(t, root, "/api/sets/coffee/items/R7", http.StatusNotFound, &e)
	if e.Error != "No item of coffee has the id R7." || e.Retired != nil {
		t.Errorf("answer = %+v", e)
	}
	answer(t, root, "/api/sets/coffee/items/R3", http.StatusNotFound, &e)
	if e.Retired == nil || *e.Retired != (retiredJSON{ID: "R3", Title: "Water Handling", File: "b-tank.shall.yaml", Line: 18}) {
		t.Errorf("answer = %+v", e)
	}
}

func TestFindingsInPlace(t *testing.T) {
	t.Attr("verifies", "R30")
	t.Attr("verifies", "R30.1")
	root := write(t, with(coffee(), map[string]string{
		"shall.yaml":         "name: coffee\nformat: 1\nowner: me\n",
		"c-loose.shall.yaml": "title: Loose\nrequirements:\n  - id: R6\n    title: A\n    text: The machine SHALL hum.\n    serves: [N1]\n    status: draft\n    retired:\n      - {id: R7.1, title: Elsewhere}\nextra: 1\n",
	}))
	a := setOf(t, root)
	if len(a.Findings) != 1 || a.Findings[0].Field != "owner" || a.Findings[0].File != "shall.yaml" {
		t.Errorf("set findings = %+v", a.Findings)
	}
	var messages []string
	for _, f := range item(t, root, "R2").Findings {
		messages = append(messages, string(f.Severity)+": "+f.Message)
	}
	if got := strings.Join(messages, "; "); !strings.Contains(got, "warning: R1.2 is retired") || !strings.Contains(got, "error: R9 is no item of this set") {
		t.Errorf("R2 findings = %s", got)
	}
	loose := file(t, root, "c-loose.shall.yaml")
	if len(loose.Findings) != 1 || loose.Findings[0].Field != "extra" {
		t.Errorf("file findings = %+v", loose.Findings)
	}
	if f := loose.Requirements[0].Findings; len(f) != 1 || !strings.Contains(f[0].Message, "R7.1 cannot be retired under R6") {
		t.Errorf("R6 findings = %+v", f)
	}
}

func TestTheAnswersCarryTextAsWritten(t *testing.T) {
	root := write(t, with(coffee(), map[string]string{
		"a-brew.shall.yaml": "title: Brewing\nrequirements:\n  - {id: R4, title: \"<script>alert(1)</script>\", text: The machine SHALL brew., serves: [N1], status: draft}\n",
	}))
	code, body := get(t, root, "/api/sets/coffee/files/a-brew.shall.yaml")
	if code != http.StatusOK || strings.Contains(body, "<script>") {
		t.Errorf("status %d, body %s", code, body)
	}
	if r4 := file(t, root, "a-brew.shall.yaml").Requirements[0]; r4.Title != "<script>alert(1)</script>" {
		t.Errorf("title = %q", r4.Title)
	}
}

func TestPureCSSAndItsLicence(t *testing.T) {
	t.Attr("verifies", "R35")
	t.Attr("verifies", "R35.2")
	root := write(t, coffee())
	_, page := get(t, root, "/coffee/")
	if !strings.Contains(page, `<link rel="stylesheet" href="/-/static/pure/pure-min.css">`) || !strings.Contains(page, `<link rel="stylesheet" href="/-/static/style.css">`) {
		t.Errorf("page = %s", page)
	}
	code, css := get(t, root, "/-/static/pure/pure-min.css")
	if code != http.StatusOK || !strings.Contains(css, "Pure v3.1.0") {
		t.Errorf("pure-min.css: status %d, %.80q", code, css)
	}
	code, licence := get(t, root, "/-/static/pure/LICENSE")
	if code != http.StatusOK || !strings.Contains(licence, "Software License Agreement (BSD License)") {
		t.Errorf("licence: status %d, %.80q", code, licence)
	}
}

func TestTheScanNoticesEveryChange(t *testing.T) {
	t.Attr("verifies", "R32.2")
	t.Attr("verifies", "R32.3")
	root := write(t, coffee())
	s := New(root, true)
	if s.scan(s.dirs()) {
		t.Fatal("a scan with nothing changed reported a change")
	}
	path := filepath.Join(root, "a-brew.shall.yaml")
	mod := time.Now().Truncate(time.Second)
	save := func(src string) {
		writeFile(t, path, src)
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	save(strings.Replace(brew, "draft", "drift", 1))
	s.scan(s.dirs())
	save(strings.Replace(brew, "draft", "dreft", 1))
	if !s.scan(s.dirs()) {
		t.Error("a save that kept the size and time went unnoticed")
	}
	writeFile(t, filepath.Join(root, "c-new.shall.yaml"), brew)
	if !s.scan(s.dirs()) {
		t.Error("a new file went unnoticed")
	}
	writeFile(t, filepath.Join(root, "notes.txt"), "not a set file")
	if s.scan(s.dirs()) {
		t.Error("a file of no set counted as a change")
	}
	os.Remove(filepath.Join(root, "c-new.shall.yaml"))
	if !s.scan(s.dirs()) {
		t.Error("a removed file went unnoticed")
	}
	writeFile(t, filepath.Join(root, "tea", "shall.yaml"), "name: tea\nformat: 1\n")
	if !s.scan(s.dirs()) {
		t.Error("a new manifest went unnoticed")
	}
}

func TestVersionsDifferAcrossStarts(t *testing.T) {
	t.Attr("verifies", "R29")
	root := write(t, coffee())
	a, b := New(root, true), New(root, true)
	if a.current().version == b.current().version {
		t.Errorf("two starts share version %s, so a page open across a restart would not catch up", a.current().version)
	}
}

// stream opens the server's stream of versions and returns them as they
// come, and a function that closes it.
func stream(t *testing.T, addr string) (<-chan string, func()) {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	versions := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				versions <- v
			}
		}
		close(versions)
	}()
	return versions, func() { resp.Body.Close() }
}

func TestFollowsTheFiles(t *testing.T) {
	t.Attr("verifies", "R32")
	root := write(t, coffee())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(root, true)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)
	go func() { done <- s.Serve(ctx, ln) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	versions, closeStream := stream(t, ln.Addr().String())
	first := <-versions
	if first != s.current().version {
		t.Fatalf("first version = %s, want %s", first, s.current().version)
	}
	saved := time.Now()
	writeFile(t, filepath.Join(root, "a-brew.shall.yaml"), strings.Replace(brew, "Brew Button", "Big Brew Button", 1))
	select {
	case v := <-versions:
		if v == first {
			t.Fatalf("version after a change = %s, the same as before", v)
		}
	case <-time.After(3 * pollEvery):
		t.Fatalf("no change after %v", time.Since(saved))
	}
	resp, err := http.Get("http://" + ln.Addr().String() + "/api/sets/coffee/items/R4")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Big Brew Button") {
		t.Errorf("R4 after the change: %s", body)
	}
	closeStream()
	for deadline := time.Now().Add(2 * time.Second); s.subscribers() > 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	if n := s.subscribers(); n != 0 {
		t.Errorf("%d streams still open after the reader left", n)
	}
}
