package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shall/internal/chrome"
	"shall/internal/testfile"
)

// The tests in this file drive the page in a headless Chrome. They skip
// when there is no Chrome to run; SHALL_CHROME names one elsewhere.

var (
	browserOnce sync.Once
	browser     *chrome.Browser
	browserErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if browser != nil {
		browser.Close()
	}
	os.Exit(code)
}

// pageSet returns a set long enough to scroll through: a mission taller than
// the screen, and three chapters of a dozen requirements each, at different
// statuses.
func pageSet() map[string]string {
	var m strings.Builder
	m.WriteString("mission: A product with enough requirements to scroll through.\nobjectives:\n")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&m, "  - {id: O%d, title: Objective %d, text: The product reaches objective %d and keeps it for as long as anyone needs it to., measure: how far objective %d is reached}\n", i, i, i, i)
	}
	m.WriteString("stakeholders:\n  - {id: S1, title: User, text: Uses the product every day.}\n  - {id: S2, title: Owner, text: Pays for the product.}\nneeds:\n")
	for i := 1; i <= 6; i++ {
		notes := ""
		if i == 1 {
			notes = ", notes: [Heard in the field.]"
		}
		fmt.Fprintf(&m, "  - {id: N%d, title: Need %d, stakeholder: S1, text: The user needs thing %d to work without having to think about it at all., serves: [O1], status: validated%s}\n", i, i, i, notes)
	}
	chapter := func(title, word string, first int, status string, extra func(id int) string) string {
		var c strings.Builder
		fmt.Fprintf(&c, "title: %s\nrequirements:\n", title)
		for id := first; id < first+12; id++ {
			fmt.Fprintf(&c, "  - id: R%d\n    title: %s %d\n    text: When %s %d happens, the product SHALL answer %s %d in a way that takes a line or two to describe.\n",
				id, title, id, word, id, word, id)
			c.WriteString(extra(id))
			fmt.Fprintf(&c, "    status: %s\n", status)
		}
		return c.String()
	}
	return map[string]string{
		"shall.yaml":         "name: demo\nformat: 1\n",
		"mission.shall.yaml": m.String(),
		"a-alpha.shall.yaml": chapter("Alpha", "alpha", 1, "implemented", func(id int) string {
			switch id {
			case 1:
				return "    serves: [N2]\n    rationale: Alpha one has its reasons.\n    notes: [An alpha note.]\n" +
					"    links:\n      - {url: \"https://example.com/evidence\", title: Evidence}\n" +
					"    children:\n      - {id: R1.1, title: Alpha child, text: The product SHALL do the first part., status: implemented}\n"
			case 2:
				return "    serves: [R1]\n"
			case 11:
				return "    serves: [R13]\n"
			}
			return fmt.Sprintf("    serves: [N%d]\n", id%6+1)
		}) + "retired:\n  - {id: R13, title: Old Alpha}\n",
		"b-beta.shall.yaml": chapter("Beta", "beta", 20, "approved", func(id int) string {
			if id == 20 {
				return "    serves: [N1]\n    children:\n      - {id: R20.1, title: Beta child, text: The product SHALL do the approved part., status: approved}\n" +
					"      - {id: R20.2, title: Beta draft, text: The product SHALL do the draft part., status: draft}\n"
			}
			return "    serves: [N1]\n"
		}),
		"c-gamma.shall.yaml": chapter("Gamma", "gamma", 40, "draft", func(id int) string {
			switch id {
			case 40:
				return "    serves: [N99]\n"
			case 51:
				return ""
			}
			return "    serves: [N3]\n"
		}),
	}
}

// page is a tab showing the sets under a directory, as shall serve serves
// them.
type page struct {
	t    *testing.T
	p    *chrome.Page
	base string
	root string
	// stop stops the server; the tab stays open.
	stop func()
}

// openPage serves the files under a new directory and opens path there in a
// tab of a headless Chrome. It skips the test when there is no Chrome, and
// fails it when the page has a script error.
func openPage(t *testing.T, files map[string]string, path string) *page {
	t.Helper()
	exe := chrome.Find()
	if exe == "" {
		t.Skip("no Chrome to drive; set SHALL_CHROME to one")
	}
	browserOnce.Do(func() { browser, browserErr = chrome.Start(exe) })
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	root := testfile.Write(t, files)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, ln) }()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); <-done }) }
	g := &page{t: t, base: "http://" + ln.Addr().String(), root: root, stop: stop}
	g.p = g.tab()
	t.Cleanup(stop)
	g.open(path)
	return g
}

// tab opens a tab of its own, closed and checked for script errors when the
// test ends.
func (g *page) tab() *chrome.Page {
	g.t.Helper()
	p, err := browser.NewPage()
	if err != nil {
		g.t.Fatal(err)
	}
	g.t.Cleanup(func() {
		if probs := p.Problems(); len(probs) > 0 {
			g.t.Errorf("the page had script errors: %v", probs)
		}
		p.Close()
	})
	return p
}

// another opens a second tab on the same server, at path.
func (g *page) another(path string) *page {
	g.t.Helper()
	other := &page{t: g.t, base: g.base, root: g.root, stop: g.stop}
	other.p = g.tab()
	other.open(path)
	return other
}

func (g *page) open(path string) {
	g.t.Helper()
	if err := g.p.Navigate(g.base + path); err != nil {
		g.t.Fatal(err)
	}
	if err := g.p.Until(`document.querySelector("main")?.childElementCount > 0`, 5*time.Second); err != nil {
		g.t.Fatalf("%s showed nothing: %v", path, err)
	}
}

// eval evaluates expr in the page into v.
func (g *page) eval(expr string, v any) {
	g.t.Helper()
	if err := g.p.Eval(expr, v); err != nil {
		g.t.Fatal(err)
	}
}

func (g *page) str(expr string) string {
	g.t.Helper()
	var s string
	g.eval(`String(`+expr+`)`, &s)
	return s
}

func (g *page) num(expr string) float64 {
	g.t.Helper()
	var n float64
	g.eval(`Number(`+expr+`)`, &n)
	return n
}

// is tells whether expr holds now.
func (g *page) is(expr string) bool {
	g.t.Helper()
	var ok bool
	g.eval(`Boolean(`+expr+`)`, &ok)
	return ok
}

// expect ends the test unless expr comes to hold within three seconds,
// saying what the step was, what it threw, and where the page was.
func (g *page) expect(expr, what string) {
	g.t.Helper()
	if err := g.p.Until(expr, 3*time.Second); err != nil {
		g.t.Fatalf("%s: %s: %v (page at %s)", what, expr, err, g.str(`location.href`))
	}
}

func (g *page) do(expr string) {
	g.t.Helper()
	g.eval(expr, nil)
}

func (g *page) click(selector string) {
	g.t.Helper()
	g.expect(fmt.Sprintf(`document.querySelector(%q)`, selector), "there is "+selector+" to click")
	g.do(fmt.Sprintf(`document.querySelector(%q).click()`, selector))
}

// call sends a DevTools command to the page.
func (g *page) call(method string, params map[string]any) {
	g.t.Helper()
	if _, err := g.p.Call(method, params); err != nil {
		g.t.Fatal(err)
	}
}

// find types text into the find box and submits it.
func (g *page) find(text string) {
	g.t.Helper()
	g.do(fmt.Sprintf(`(() => { const f = document.querySelector('form.jump'); f.elements.id.value = %q; f.requestSubmit(); })()`, text))
}

func (g *page) save(name, src string) {
	g.t.Helper()
	testfile.WriteFile(g.t, filepath.Join(g.root, name), src)
}

func (g *page) read(name string) string {
	g.t.Helper()
	b, err := os.ReadFile(filepath.Join(g.root, name))
	if err != nil {
		g.t.Fatal(err)
	}
	return string(b)
}

// version returns the version of the files the page shows.
func (g *page) version() string { return g.str(`state.version`) }

// served returns the version of the files the server now serves.
func (g *page) served() string {
	g.t.Helper()
	resp, err := http.Get(g.base + "/api/sets")
	if err != nil {
		g.t.Fatal(err)
	}
	defer resp.Body.Close()
	var a struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		g.t.Fatal(err)
	}
	return a.Version
}

// top returns where an element stands from the top of the screen.
func (g *page) top(id string) float64 {
	g.t.Helper()
	return g.num(fmt.Sprintf(`document.getElementById(%q).getBoundingClientRect().top`, id))
}

// colour returns the colour a CSS value such as var(--muted) computes to.
func (g *page) colour(value string) string {
	g.t.Helper()
	return g.str(fmt.Sprintf(`(() => { const e = document.createElement('span'); e.style.color = %q; document.body.append(e); const c = getComputedStyle(e).color; e.remove(); return c; })()`, value))
}

// rgb reads a computed colour such as rgb(198, 40, 40).
func rgb(t *testing.T, colour string) (r, g, b int) {
	t.Helper()
	if _, err := fmt.Sscanf(colour, "rgb(%d, %d, %d)", &r, &g, &b); err != nil {
		t.Fatalf("colour %q: %v", colour, err)
	}
	return r, g, b
}

func sel(path string) string {
	return fmt.Sprintf(`document.querySelector('section[data-path=%q]')`, path)
}

func TestPageStartListsTheSetsAsLinks(t *testing.T) {
	t.Attr("verifies", "R25.4")
	g := openPage(t, pageSet(), "/")
	g.click(`.sets a[href="/demo/"]`)
	g.expect(`location.pathname === '/demo/' && !!document.getElementById('file:mission.shall.yaml')`, "the set's name leads to its document")
}

func TestPageStartShowsClashes(t *testing.T) {
	t.Attr("verifies", "R25.5")
	set := pageSet()
	files := map[string]string{}
	for name, src := range set {
		files["one/"+name] = src
		files["two/"+name] = src
	}
	g := openPage(t, files, "/")
	g.expect(`[...document.querySelectorAll('main .finding.error')].some((li) => li.textContent.includes('one/shall.yaml and two/shall.yaml share the name demo'))`, "a shared name shows as an error")
	if g.is(`document.querySelector('.sets a[href="/demo/"]')`) {
		t.Error("a name two sets share links to a document")
	}
}

func TestPageStartCompares(t *testing.T) {
	t.Attr("verifies", "R25.6")
	g := openPage(t, pageSet(), "/")
	const want = "39 requirements · 14 draft · 12 approved · 13 implemented"
	if got := g.str(`document.querySelector('.sets .statuses')?.textContent`); got != want {
		t.Errorf("the set's counts = %q, want %q", got, want)
	}
}

func TestPageFoldsAndUnfoldsChapters(t *testing.T) {
	t.Attr("verifies", "R26.2")
	t.Attr("verifies", "R26.4")
	g := openPage(t, pageSet(), "/demo/")
	g.expect(sel("b-beta.shall.yaml")+`.dataset.open === 'false'`, "a chapter below the screen starts folded")
	g.do(`(() => { const b = ` + sel("b-beta.shall.yaml") + `.querySelector('button.fold'); b.focus(); b.click(); })()`)
	g.expect(`!!document.getElementById('R20')`, "unfolding a chapter loads its items")
	g.expect(`location.search.includes('open=b-beta.shall.yaml')`, "the unfolded chapter goes into the address")
	g.expect(`document.activeElement?.classList.contains('fold')`, "focus stays on the fold button")
	g.do(sel("b-beta.shall.yaml") + `.querySelector('button.fold').click()`)
	g.expect(`!document.getElementById('R20')`, "folding a chapter removes its items")
	g.expect(`!location.search.includes('open=b-beta') && location.search.includes('fold=b-beta.shall.yaml')`, "the address keeps the chapter folded")
}

func TestPageStatusesShown(t *testing.T) {
	t.Attr("verifies", "R26.3")
	g := openPage(t, pageSet(), "/demo/R20")
	if got, want := g.str(`document.querySelector('#R20 > .head .status')?.textContent`), "draft · written approved"; got != want {
		t.Errorf("R20's status = %q, want %q", got, want)
	}
	if got, want := g.str(`document.querySelector('#R21 > .head .status')?.textContent`), "approved"; got != want {
		t.Errorf("R21's status = %q, want %q", got, want)
	}
	if got, want := g.str(`document.querySelector('#N1 > .head .status')?.textContent`), "validated"; got != want {
		t.Errorf("N1's status = %q, want %q", got, want)
	}
}

func TestPageFollowsALinkIntoAFoldedChapter(t *testing.T) {
	t.Attr("verifies", "R26.6")
	t.Attr("verifies", "R27.3")
	g := openPage(t, pageSet(), "/demo/")
	g.click(`#N2 a[data-item="R1"]`)
	g.expect(`!!document.getElementById('R1')`, "following a link unfolds the chapter")
	g.expect(`location.pathname === '/demo/R1'`, "the address becomes the item's")
	g.expect(`document.getElementById('R1').classList.contains('target') && Math.abs(document.getElementById('R1').getBoundingClientRect().top) < 80`, "the item is marked and at the top")
	g.expect(`document.activeElement?.id === 'R1'`, "keyboard focus moves to the item")
	g.open(strings.TrimPrefix(g.str(`location.href`), g.base))
	g.expect(`document.getElementById('R1')?.classList.contains('target')`, "the address alone reopens the view")
}

func TestPageGoesBack(t *testing.T) {
	t.Attr("verifies", "R26.2")
	g := openPage(t, pageSet(), "/demo/R2")
	g.click(`#R2 a[data-item="R1"]`)
	g.expect(`location.pathname === '/demo/R1'`, "the link goes to R1")
	g.do(`history.back()`)
	g.expect(`location.pathname === '/demo/R2' && document.getElementById('R2').classList.contains('target')`, "back returns to the previous item")
}

func TestPageIdsThatLeadNowhere(t *testing.T) {
	t.Attr("verifies", "R27.4")
	t.Attr("verifies", "R27.5")
	g := openPage(t, pageSet(), "/demo/R11?open=c-gamma.shall.yaml")
	g.expect(`!!document.getElementById('R40')`, "the chapter with an unknown id unfolds")
	if got := g.str(`(() => { const u = document.querySelector('#R40 > .serves .unknown'); return u ? u.tagName + ' ' + u.textContent + ' ' + u.hasAttribute('href') : 'none'; })()`); got != "SPAN N99 false" {
		t.Errorf("the unknown id N99 shows as %q, want plain text in a span", got)
	}
	if g.is(`document.querySelector('#R40 > .serves a')`) {
		t.Error("an unknown id is a link")
	}
	if got, want := g.str(`document.querySelector('#R11 > .serves .retired')?.textContent`), "R13 retired: Old Alpha"; got != want {
		t.Errorf("the retired id shows as %q, want %q", got, want)
	}
	if g.is(`document.querySelector('#R11 > .serves a')`) {
		t.Error("a retired id is a link")
	}
}

func TestPageFindsByIdAndWords(t *testing.T) {
	t.Attr("verifies", "R28")
	t.Attr("verifies", "R28.1")
	t.Attr("verifies", "R28.3")
	g := openPage(t, pageSet(), "/demo/")
	g.find("R20.1")
	g.expect(`location.pathname === '/demo/R20.1' && !!document.getElementById('R20.1')`, "an id goes to its item")
	scroll := g.num(`scrollY`)
	g.find("R99")
	g.expect(`document.querySelector('.notice').textContent.includes('No item has the id R99')`, "an id of no item says so")
	if got := g.str(`location.pathname + ' ' + document.querySelector('.target')?.id + ' ' + scrollY`); got != fmt.Sprintf("/demo/R20.1 R20.1 %v", scroll) {
		t.Errorf("after a miss the page is at %q, want it where it was: /demo/R20.1 R20.1 %v", got, scroll)
	}
	g.do(`(() => { const i = document.querySelector('form.jump input'); i.value = 'gamma 45'; i.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	g.expect(`!document.getElementById('results').hidden && document.querySelector('#results li .ref')?.textContent === 'R45'`, "words list the items that hold them")
	g.do(`document.querySelector('form.jump input').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true}))`)
	g.expect(`document.querySelector('#results li[aria-selected="true"]')?.id === 'result-0'`, "arrow keys choose a result")
	g.do(`document.querySelector('form.jump').requestSubmit()`)
	g.expect(`location.pathname === '/demo/R45' && !!document.getElementById('R45')`, "Enter goes to it, in a folded chapter too")
	g.expect(`document.getElementById('results').hidden`, "the list closes")
}

func TestPagePreviews(t *testing.T) {
	t.Attr("verifies", "R27.1")
	g := openPage(t, pageSet(), "/demo/")
	link := `document.querySelector('#N2 a[data-preview][data-item="R1"]')`
	g.do(link + `.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}))`)
	g.expect(`!document.getElementById('preview').hidden && document.getElementById('preview').textContent.includes('When alpha 1 happens')`, "pointing at a link shows a preview")
	g.do(`document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}))`)
	g.expect(`document.getElementById('preview').hidden`, "Escape hides the preview")
	g.do(link + `.focus()`)
	g.expect(`!document.getElementById('preview').hidden`, "keyboard focus shows a preview")
	g.do(`document.activeElement.blur()`)
	g.expect(`document.getElementById('preview').hidden`, "losing focus hides it")
}

func TestPageFoldArea(t *testing.T) {
	t.Attr("verifies", "R26.13")
	t.Attr("verifies", "R27.2.1")
	g := openPage(t, pageSet(), "/demo/R1")
	if got, want := g.str(`document.querySelector('#R1 > details.more > summary')?.textContent`), "rationale, notes · named in serves: R2"; got != want {
		t.Errorf("the fold's summary = %q, want %q", got, want)
	}
	if got := g.num(`document.querySelectorAll('#R1 > details').length`); got != 1 || g.is(`document.querySelector('#R1 > details.more').open`) {
		t.Errorf("R1 has %v fold areas, want one, folded", got)
	}
	g.click(`#R1 > details.more > summary`)
	g.expect(`location.search.includes('details=R1')`, "unfolding it goes into the address")
	text := g.str(`document.querySelector('#R1 > details.more').textContent`)
	for _, want := range []string{"Alpha one has its reasons.", "An alpha note.", "Alpha 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("the fold area = %q, want it to hold %q", text, want)
		}
	}
	if strings.Contains(text, "When alpha 2 happens") {
		t.Errorf("the fold area = %q, want no text of the items that name R1", text)
	}
	if got, want := g.str(`[...document.getElementById('R1').children].map((e) => e.className || e.tagName.toLowerCase()).join(' ')`), "head serves text fields more children"; got != want {
		t.Errorf("R1's parts = %q, want %q, the fold area last before the children", got, want)
	}
}

func TestPageTitlesBesideIds(t *testing.T) {
	t.Attr("verifies", "R27.6")
	g := openPage(t, pageSet(), "/demo/R1")
	if got, want := g.str(`document.querySelector('#R1 > .serves .ref-title')?.textContent`), "Need 2"; got != want {
		t.Errorf("the title beside N2 = %q, want %q", got, want)
	}
	if got, want := g.str(`getComputedStyle(document.querySelector('#R1 > .serves .ref-title')).color`), g.colour("var(--muted)"); got != want {
		t.Errorf("the title's colour = %s, want the grey %s", got, want)
	}
	if got, want := g.str(`document.querySelector('#N1 dd .ref-title')?.textContent`), "User"; got != want {
		t.Errorf("the title beside the stakeholder = %q, want %q", got, want)
	}
}

func TestPageOutsideLinksAndText(t *testing.T) {
	t.Attr("verifies", "R26.14")
	t.Attr("verifies", "R31.3")
	files := pageSet()
	files["a-alpha.shall.yaml"] = strings.Replace(files["a-alpha.shall.yaml"], "title: Alpha 3\n", "title: \"<b>Bold</b> & <img src=x onerror=alert(1)>\"\n", 1)
	g := openPage(t, files, "/demo/R1")
	if got := g.str(`document.querySelector('#R1 a[href="https://example.com/evidence"]')?.target`); got != "_blank" {
		t.Errorf("the outside link's target = %q, want _blank", got)
	}
	if got, want := g.str(`document.querySelector('#R3 .title')?.textContent`), "<b>Bold</b> & <img src=x onerror=alert(1)>"; got != want {
		t.Errorf("the title with markup shows %q, want the text %q", got, want)
	}
	if g.is(`document.querySelector('#R3 .title b, #R3 .title img')`) {
		t.Error("a title's markup became elements")
	}
}

func TestPageFindingsInPlace(t *testing.T) {
	t.Attr("verifies", "R30")
	t.Attr("verifies", "R30.1")
	files := pageSet()
	files["shall.yaml"] = "name: demo\nformat: 1\nowner: me\n"
	files["c-gamma.shall.yaml"] = files["c-gamma.shall.yaml"] + "extra: 1\n"
	g := openPage(t, files, "/demo/R40")
	if got := g.str(`document.querySelector('#R40 > .findings .finding.error')?.textContent`); !strings.Contains(got, "error · serves: N99 is no item of this set") {
		t.Errorf("R40's finding = %q, want the unknown N99 as an error on R40", got)
	}
	if got := g.str(`document.querySelector('#R51 > .findings .finding.warning')?.textContent`); !strings.Contains(got, "warning · serves: the requirement serves nothing") {
		t.Errorf("R51's finding = %q, want the missing serves as a warning on R51", got)
	}
	if got := g.str(`document.querySelector('main > .findings')?.textContent`); !strings.Contains(got, "owner") || !strings.Contains(got, "shall.yaml:3") {
		t.Errorf("the findings at the top = %q, want the manifest's owner field", got)
	}
	if got := g.str(sel("c-gamma.shall.yaml") + `.querySelector(':scope > .findings')?.textContent`); !strings.Contains(got, "extra") {
		t.Errorf("the findings at the head of Gamma = %q, want its extra field", got)
	}
}

func TestPageFollowsTheFiles(t *testing.T) {
	t.Attr("verifies", "R29")
	t.Attr("verifies", "R29.2")
	t.Attr("verifies", "R26.8")
	g := openPage(t, pageSet(), "/demo/?open=a-alpha.shall.yaml")
	src := g.read("a-alpha.shall.yaml")
	g.do(`document.getElementById('R5').scrollIntoView({block: 'start'}); scrollBy(0, -20)`)
	var place struct {
		ID  string  `json:"id"`
		Top float64 `json:"top"`
	}
	g.eval(`(() => { const e = [...document.querySelectorAll('main [data-anchor]')].find((el) => el.getBoundingClientRect().top >= 0); return {id: e.id, top: e.getBoundingClientRect().top}; })()`, &place)
	before := g.version()
	added := strings.Replace(src, "  - id: R1\n", "  - id: R60\n    title: Inserted\n    text: The product SHALL show this.\n    serves: [N1]\n    status: draft\n    notes: [one, two, three]\n  - id: R1\n", 1)
	saved := time.Now()
	g.save("a-alpha.shall.yaml", added)
	if err := g.p.Until(fmt.Sprintf(`state.version !== %q && !!document.getElementById('R60')`, before), 2*time.Second); err != nil {
		t.Fatalf("a saved change did not show within two seconds (%v): %v", time.Since(saved), err)
	}
	if after := g.top(place.ID); after-place.Top > 2 || place.Top-after > 2 {
		t.Errorf("the item at the top, %s, moved from %v to %v; want it to stay", place.ID, place.Top, after)
	}
	g.do(`document.getElementById('R60').scrollIntoView({block: 'start'}); scrollBy(0, -5)`)
	top := g.top("R60")
	g.save("a-alpha.shall.yaml", src)
	g.expect(`!document.getElementById('R60')`, "a removed item goes")
	if next := g.top("R1"); next-top > 2 || top-next > 2 {
		t.Errorf("R1 stands at %v after the item above it went, want %v, where that item stood", next, top)
	}
	g.save("a-alpha.shall.yaml", "title: [oops\n")
	g.expect(sel("a-alpha.shall.yaml")+`?.classList.contains('broken') && !!`+sel("a-alpha.shall.yaml")+`.querySelector('.finding.error')`, "a broken file shows in place")
	if !g.is(`!!` + sel("b-beta.shall.yaml")) {
		t.Error("the rest of the set went with the broken file")
	}
	g.save("a-alpha.shall.yaml", src)
	g.expect(`!!document.getElementById('R5')`, "the file comes back")
}

func TestPageFollowsTheFilesInEveryTab(t *testing.T) {
	t.Attr("verifies", "R29.1")
	t.Attr("verifies", "R32.1")
	g := openPage(t, pageSet(), "/demo/R1")
	tabs := []*page{g, g.another("/demo/R1"), g.another("/demo/R2")}
	g.do(`Object.defineProperty(document, 'hidden', {configurable: true, get: () => true}); document.dispatchEvent(new Event('visibilitychange'))`)
	if !g.is(`events === null`) {
		t.Error("a hidden page kept its stream")
	}
	before := g.version()
	g.save("a-alpha.shall.yaml", strings.Replace(g.read("a-alpha.shall.yaml"), "title: Alpha 1\n", "title: Alpha One\n", 1))
	for _, other := range tabs[1:] {
		other.expect(`document.querySelector('#R1 .title')?.textContent === 'Alpha One'`, "every visible tab follows the files")
	}
	if g.served() == before || g.version() != before {
		t.Errorf("the hidden page shows version %s of the server's %s, want it to stay at %s", g.version(), g.served(), before)
	}
	g.do(`Object.defineProperty(document, 'hidden', {configurable: true, get: () => false}); document.dispatchEvent(new Event('visibilitychange'))`)
	g.expect(`document.querySelector('#R1 .title')?.textContent === 'Alpha One'`, "shown again, the page catches up")
}

func TestPageOutline(t *testing.T) {
	t.Attr("verifies", "R26.11")
	t.Attr("verifies", "R26.11.1")
	t.Attr("verifies", "R26.11.2")
	g := openPage(t, pageSet(), "/demo/")
	if got, want := g.str(`[...document.querySelectorAll('#outline-menu > ul > li > a')].map((a) => a.textContent).join('|')`), "Mission|Alpha|Beta|Gamma"; got != want {
		t.Errorf("the outline = %q, want %q", got, want)
	}
	if g.is(`document.querySelector('#outline-menu a[data-item]')`) {
		t.Error("the outline lists items of folded chapters")
	}
	g.click(`#outline-menu a[data-path="b-beta.shall.yaml"]`)
	g.expect(`!!document.getElementById('R20')`, "an outline entry unfolds its chapter")
	g.expect(`Math.abs(document.getElementById('file:b-beta.shall.yaml').getBoundingClientRect().top - 56) < 12`, "and goes to it")
	g.expect(`!!document.querySelector('#outline-menu a[data-item="R20"]')`, "an unfolded chapter lists its items")
	g.expect(`document.querySelector('#outline-menu > ul > li.pure-menu-selected')?.dataset.for === 'file:b-beta.shall.yaml'`, "the outline marks the chapter at the top")
	g.click(`#outline-menu a[data-item="R25"]`)
	g.expect(`location.pathname === '/demo/R25'`, "an outline item goes to its item")
	g.expect(`document.querySelector('#outline-menu li.pure-menu-selected > ul > li.pure-menu-selected > a')?.dataset.item === 'R25'`, "the outline marks the item the reader is in")
	g.click(`#outline-menu a[data-scroll="file:mission.shall.yaml:Needs"]`)
	g.expect(`document.querySelector('#outline-menu > ul > li.pure-menu-selected')?.dataset.for === 'file:mission.shall.yaml'`, "the mark follows the reader")
}

func TestPageOnAPhone(t *testing.T) {
	t.Attr("verifies", "R26.11.3")
	g := openPage(t, pageSet(), "/demo/")
	g.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	g.open("/demo/?status=implemented")
	g.expect(`getComputedStyle(document.getElementById('outline')).display === 'none' && getComputedStyle(document.querySelector('button.menu')).display !== 'none'`, "a narrow screen folds the outline behind a button")
	if got, width := g.num(`document.documentElement.scrollWidth`), g.num(`innerWidth`); got > width {
		t.Errorf("the page is %v wide on a %v screen, want no sideways scrolling", got, width)
	}
	if got, width := g.num(`document.querySelector('button.scheme').getBoundingClientRect().right`), g.num(`innerWidth`); got > width {
		t.Errorf("the colour switch ends at %v on a %v screen, want it on screen", got, width)
	}
	if got := g.str(`getComputedStyle(document.querySelector('section.file:not(.broken) > h2 .path')).display`); got != "none" {
		t.Errorf("a chapter's path shows as %q on a phone, want it hidden", got)
	}
	if got, want := g.str(`document.querySelector('button.menu').textContent`), "Outline •"; got != want {
		t.Errorf("the Outline button reads %q while a filter is on, want %q", got, want)
	}
	g.click(`button.menu`)
	g.expect(`getComputedStyle(document.getElementById('outline')).display !== 'none' && getComputedStyle(document.querySelector('.outline .filter')).display !== 'none'`, "the button unfolds the outline, with the filter")
	g.click(`#outline-menu a[data-path="a-alpha.shall.yaml"]`)
	g.expect(`!reaching && !!document.getElementById('R1') && Math.abs(document.getElementById('file:a-alpha.shall.yaml').getBoundingClientRect().top - 56) < 12`, "an outline entry lands its chapter under the bar")
	g.expect(`getComputedStyle(document.getElementById('outline')).display === 'none'`, "following an entry folds the outline again")
}

func TestPageUnfoldsChaptersAsTheyAreReached(t *testing.T) {
	t.Attr("verifies", "R26.12")
	g := openPage(t, pageSet(), "/demo/")
	if !g.is(sel("a-alpha.shall.yaml") + `.dataset.open === 'false' && ` + sel("b-beta.shall.yaml") + `.dataset.open === 'false'`) {
		t.Fatal("chapters below the screen did not start folded")
	}
	g.do(sel("a-alpha.shall.yaml") + `.scrollIntoView({block: 'end'})`)
	g.expect(sel("a-alpha.shall.yaml")+`.dataset.open === 'true' && !reaching`, "a chapter unfolds when its heading comes into view")
	if g.is(sel("b-beta.shall.yaml") + `.dataset.open !== 'false'`) {
		t.Error("the next chapter unfolded before it was reached")
	}
	if got := g.str(`location.search`); got != "" {
		t.Errorf("the address = %q after an unfolding on reach, want it unchanged", got)
	}
	g.do(sel("a-alpha.shall.yaml") + `.scrollIntoView({block: 'start'})`)
	g.do(sel("a-alpha.shall.yaml") + `.querySelector('button.fold').click()`)
	g.expect(sel("a-alpha.shall.yaml")+`.dataset.open === 'false'`, "folding by hand folds it")
	g.expect(sel("b-beta.shall.yaml")+`.dataset.open === 'true' && !reaching`, "the chapter after it unfolds when reached")
	if !g.is(sel("a-alpha.shall.yaml") + `.dataset.open === 'false' && ` + sel("a-alpha.shall.yaml") + `.hasAttribute('data-held')`) {
		t.Error("a chapter folded by hand unfolded again while in view")
	}
	g.open(strings.TrimPrefix(g.str(`location.href`), g.base))
	g.do(sel("a-alpha.shall.yaml") + `.scrollIntoView({block: 'start'})`)
	g.expect(sel("b-beta.shall.yaml")+`.dataset.open === 'true' && !reaching`, "after reopening the address, the chapter after it unfolds when reached")
	if !g.is(sel("a-alpha.shall.yaml") + `.dataset.open === 'false'`) {
		t.Error("reopening the address unfolded a chapter folded by hand")
	}

	// Scrolling up into a folded chapter unfolds it above the reader, who
	// stays in place.
	g.open("/demo/R25")
	g.expect(`document.getElementById('R25')?.classList.contains('target') && !reaching`, "the item's address opens at the item")
	g.do(`scrollBy(0, ` + sel("a-alpha.shall.yaml") + `.querySelector('h2').getBoundingClientRect().top + 10)`)
	below := g.top("file:b-beta.shall.yaml")
	g.expect(sel("a-alpha.shall.yaml")+`.dataset.open === 'true' && !reaching`, "a heading at the top edge unfolds its chapter")
	if after := g.top("file:b-beta.shall.yaml"); after-below > 2 || below-after > 2 {
		t.Errorf("the chapter below moved from %v to %v, want it to stay", below, after)
	}
}

func TestPageColourScheme(t *testing.T) {
	t.Attr("verifies", "R34")
	t.Attr("verifies", "R34.1")
	t.Attr("verifies", "R34.2")
	t.Attr("verifies", "R35.1")
	g := openPage(t, pageSet(), "/demo/")
	t.Cleanup(func() { g.p.Eval(`localStorage.removeItem('shall-scheme')`, nil) })
	g.do(`localStorage.removeItem('shall-scheme')`)
	scheme := func(s string) {
		g.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": s}}})
	}
	background := `getComputedStyle(document.body).backgroundColor`
	colours := func(name string) {
		t.Helper()
		bad := g.str(`getComputedStyle(document.querySelector('#R40 .finding.error')).color`)
		warning := g.str(`getComputedStyle(document.querySelector('#R51 .finding.warning')).color`)
		link := g.str(`getComputedStyle(document.querySelector('#R40 a.ref')).color`)
		for _, c := range []struct{ what, got, want string }{
			{"an error", bad, g.colour("var(--problem)")},
			{"a warning", warning, g.colour("var(--warning)")},
			{"a link", link, g.colour("var(--link)")},
		} {
			if c.got != c.want {
				t.Errorf("%s: %s is %s, want %s", name, c.what, c.got, c.want)
			}
		}
		if r, gr, b := rgb(t, bad); r < gr+60 || r < b+60 {
			t.Errorf("%s: an error is %s, want red", name, bad)
		}
		if r, gr, b := rgb(t, warning); r < b+60 || gr < b+30 || gr > r {
			t.Errorf("%s: a warning is %s, want amber", name, warning)
		}
		if r, gr, b := rgb(t, link); r < gr+40 || b < gr+40 {
			t.Errorf("%s: a link is %s, want purple", name, link)
		}
	}
	scheme("dark")
	g.open("/demo/R40?open=c-gamma.shall.yaml")
	g.expect(background+` === 'rgb(0, 0, 0)'`, "without a choice, a dark system gets white on black")
	colours("on black")
	scheme("light")
	g.expect(background+` === 'rgb(255, 255, 255)'`, "and a light system black on white")
	colours("on white")
	address := g.str(`location.href`)
	g.click(`button.scheme`)
	g.expect(background+` === 'rgb(0, 0, 0)'`, "the switch turns the page white on black")
	if got := g.str(`localStorage.getItem('shall-scheme')`); got != "dark" {
		t.Errorf("the browser keeps %q, want dark", got)
	}
	if got := g.str(`location.href`); got != address {
		t.Errorf("the address = %s after switching, want it unchanged: %s", got, address)
	}
	g.open("/")
	g.expect(background+` === 'rgb(0, 0, 0)'`, "every page keeps the choice")
	g.click(`button.scheme`)
	g.expect(background+` === 'rgb(255, 255, 255)'`, "switching back gives black on white")
}

func TestPageFilter(t *testing.T) {
	t.Attr("verifies", "R26.15")
	t.Attr("verifies", "R26.15.1")
	g := openPage(t, pageSet(), "/demo/")
	if got, want := g.str(`[...document.querySelectorAll('.bar .filter .status-filter')].map((b) => b.dataset.status).join(' ')`), "draft approved implemented"; got != want {
		t.Errorf("the filter offers %q, want %q", got, want)
	}
	g.click(`.bar .filter [data-status="approved"]`)
	g.expect(`location.search.includes('status=approved')`, "choosing a status goes into the address")
	g.expect(sel("a-alpha.shall.yaml")+`.hasAttribute('data-empty')`, "a chapter with no match stays folded")
	g.click(`#outline-menu a[data-path="b-beta.shall.yaml"]`)
	g.expect(`document.getElementById('R20')?.classList.contains('context') && !!document.getElementById('R20.1') && !document.getElementById('R20.2')`, "a parent below the status shows as context, with only its matching children")
	g.do(`document.getElementById('R31').scrollIntoView()`)
	var at struct{ Scroll, Top, Bottom float64 }
	g.eval(`(() => { const r = document.querySelector('.bar .filter .status-filter').getBoundingClientRect(); return {Scroll: scrollY, Top: r.top, Bottom: r.bottom}; })()`, &at)
	if at.Scroll < 1000 || at.Top < 0 || at.Bottom > 60 {
		t.Errorf("scrolled to %v, the filter spans %v to %v; want it in the bar, on screen", at.Scroll, at.Top, at.Bottom)
	}
	g.find("R45")
	g.expect(`!location.search.includes('status=') && location.pathname === '/demo/R45' && !!document.getElementById('R45')`, "going to an item the filter hides drops the filter")
}

func TestPageUsesOnlyTheAPI(t *testing.T) {
	t.Attr("verifies", "R36.1")
	g := openPage(t, pageSet(), "/demo/")
	g.click(`#outline-menu a[data-path="b-beta.shall.yaml"]`)
	g.expect(`!!document.getElementById('R20')`, "a chapter unfolds")
	g.do(`document.querySelector('#N2 a[data-preview][data-item="R1"]').dispatchEvent(new MouseEvent('mouseover', {bubbles: true}))`)
	g.expect(`!document.getElementById('preview').hidden`, "a preview shows")
	g.find("gamma 45")
	g.expect(`location.pathname === '/demo/R45'`, "a search goes")
	fetched := false
	for _, r := range g.p.Requests() {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host != strings.TrimPrefix(g.base, "http://") {
			if !strings.HasPrefix(r.URL, "data:") {
				t.Errorf("a %s request left the server: %s", r.Type, r.URL)
			}
			continue
		}
		switch r.Type {
		case "Document":
		case "Fetch", "XHR", "EventSource":
			fetched = fetched || r.Type == "Fetch"
			if !strings.HasPrefix(u.Path, "/api/") {
				t.Errorf("the page fetched %s, want only /api/ addresses", r.URL)
			}
		default:
			if !strings.HasPrefix(u.Path, "/-/static/") {
				t.Errorf("the page loaded a %s from %s, want only /-/static/ files", r.Type, r.URL)
			}
		}
	}
	if !fetched {
		t.Error("no fetch was seen, so the check proves nothing")
	}
}

func TestPageWhenTheServerGoes(t *testing.T) {
	g := openPage(t, pageSet(), "/demo/")
	g.stop()
	g.do(sel("c-gamma.shall.yaml") + `.querySelector('button.fold').click()`)
	g.expect(sel("c-gamma.shall.yaml")+`.hasAttribute('data-failed')`, "a chapter that cannot be fetched is marked")
	if !g.is(sel("c-gamma.shall.yaml") + `.dataset.open === 'false'`) {
		t.Error("a chapter that cannot be fetched unfolded")
	}
	g.find("R20")
	if !g.is(`location.pathname === '/demo/'`) {
		t.Error("a search with no server moved the page")
	}
}
