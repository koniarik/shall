package serve

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pagesHost serves the directory dir under base as GitHub Pages serves a
// project's site: a file by its name, else by its name with .html added,
// else a directory's index.html, an address of a directory without its last
// slash sent on to one with it; and 404.html, as 404 Not Found, for any
// other address.
func pagesHost(dir, base string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path+"/" == base {
			http.Redirect(w, r, base, http.StatusMovedPermanently)
			return
		}
		if rest, ok := strings.CutPrefix(r.URL.Path, base); ok {
			p := filepath.Join(dir, filepath.FromSlash(rest))
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				if rest != "" && !strings.HasSuffix(rest, "/") {
					http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
					return
				}
				p = filepath.Join(p, "index.html")
			} else if err != nil {
				p += ".html"
			}
			if data, err := os.ReadFile(p); err == nil {
				http.ServeContent(w, r, p, time.Time{}, bytes.NewReader(data))
				return
			}
		}
		data, _ := os.ReadFile(filepath.Join(dir, "404.html"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(data)
	})
}

// openCopy exports the sets under a new directory and opens path in a tab,
// the copy served under /shall/ as GitHub Pages serves a project's site.
func openCopy(t *testing.T, files map[string]string, path string) *page {
	t.Helper()
	startBrowser(t)
	root, out := exported(t, files, "/shall/")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: pagesHost(out, "/shall/")}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	g := &page{t: t, base: "http://" + ln.Addr().String(), root: root, stop: func() {}}
	g.p = g.tab()
	g.open(path)
	return g
}

func TestCopyPageReadsItsFiles(t *testing.T) {
	t.Attr("verifies", "R60")
	t.Attr("verifies", "R60.2")
	t.Attr("verifies", "R60.4")
	g := openCopy(t, pageSet(), "/shall/")
	g.click(`.sets a[href="/shall/demo/"]`)
	g.expect(`location.pathname === '/shall/demo/' && !!document.getElementById('file:mission.shall.yaml')`, "the set's name leads to its document")
	if got := g.str(`document.querySelector('a.home').getAttribute('href')`); got != "/shall/" {
		t.Errorf("the home link leads to %q, want /shall/", got)
	}
	g.click(`[id="N65.2"] a[data-item="R1"]`)
	g.expect(`location.pathname === '/shall/demo/R1' && document.getElementById('R1')?.classList.contains('target')`, "a link unfolds its chapter and goes to the item")
	g.open("/shall/demo/R45")
	g.expect(`document.getElementById('R45')?.classList.contains('target')`, "an item's address opens the document at it")
	g.click(`.bar .filter [data-status="approved"]`)
	g.expect(`location.search.includes('status=approved')`, "choosing a status goes into the address")
	g.expect(sel("a-alpha.shall.yaml")+`.hasAttribute('data-empty')`, "a chapter with no match stays folded")
	g.click(`#outline-menu a[data-path="b-beta.shall.yaml"]`)
	g.expect(`document.getElementById('R20')?.classList.contains('context') && !!document.getElementById('R20.1') && !document.getElementById('R20.2')`, "the filter shows only what is at the status, with parents as context")
	g.find("gamma 45")
	g.expect(`location.pathname === '/shall/demo/R45' && !location.search.includes('status=')`, "words go to the one item that holds them")
	g.find("R99")
	g.expect(`document.querySelector('.notice').textContent.includes('No item has the id R99.')`, "an id of no item says so")
	g.find("R13")
	g.expect(`document.querySelector('.notice').textContent.includes('R13 was retired at a-alpha.shall.yaml:')`, "a retired id says where it was retired")
	fetched := 0
	for _, r := range g.p.Requests() {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host != strings.TrimPrefix(g.base, "http://") {
			if !strings.HasPrefix(r.URL, "data:") {
				t.Errorf("a %s request left the host: %s", r.Type, r.URL)
			}
			continue
		}
		switch r.Type {
		case "Document":
			if !strings.HasPrefix(u.Path, "/shall/") {
				t.Errorf("the page went to %s, outside /shall/", r.URL)
			}
		case "Fetch":
			fetched++
			if !strings.HasPrefix(u.Path, "/shall/api/") || !strings.HasSuffix(u.Path, ".json") || u.RawQuery != "" {
				t.Errorf("the page fetched %s, want only files under /shall/api/", r.URL)
			}
		case "EventSource":
			t.Errorf("the copy opened a stream of changes: %s", r.URL)
		default:
			if !strings.HasPrefix(u.Path, "/shall/-/static/") {
				t.Errorf("the page loaded a %s from %s, want only /shall/-/static/ files", r.Type, r.URL)
			}
		}
	}
	if fetched == 0 {
		t.Error("no fetch was seen, so the check proves nothing")
	}
}

func TestCopyPageOfASetOfAnyName(t *testing.T) {
	t.Attr("verifies", "R60")
	g := openCopy(t, with(coffee(), map[string]string{"shall.yaml": "name: Coffee Machine\nformat: 1\n"}), "/shall/")
	g.click(`.sets a[href="/shall/Coffee%20Machine/"]`)
	g.expect(`!!document.getElementById('file:mission.shall.yaml')`, "the set's name leads to its document")
	g.click(`[id="N91.1"] a[data-item="R1"]`)
	g.expect(`location.pathname === '/shall/Coffee%20Machine/R1' && document.getElementById('R1')?.classList.contains('target')`, "a link goes to its item")
	g.find("R4")
	g.expect(`location.pathname === '/shall/Coffee%20Machine/R4'`, "an id is found")
}

func TestCopyPageAtAnAddressItLacks(t *testing.T) {
	t.Attr("verifies", "R60")
	g := openCopy(t, pageSet(), "/shall/demo/R99")
	// The host answers both the address and the item's file with 404.
	g.allowed = []string{"404"}
	g.expect(`document.querySelector('main .missing')?.textContent === 'No item of demo has the id R99.' && !!document.getElementById('file:mission.shall.yaml')`, "an address of no item shows the document and says so")
	g.open("/shall/milk/")
	g.expect(`document.querySelector('main .missing')?.textContent === 'No set named milk is served here.'`, "an address of no set says so")
	g.open("/shall/demo/%E0%A4")
	g.expect(`document.querySelector('main .missing')?.textContent === 'No item of demo has the id %E0%A4.'`, "an address that does not decode is read as written")
	for _, path := range []string{"/shall/index.html", "/shall/demo/index.html", "/shall/demo/R45.html"} {
		g.open(path)
		g.expect(`!document.querySelector('main .missing')`, path+" shows what its file is the page for")
	}
	g.expect(`document.getElementById('R45')?.classList.contains('target')`, "an item's file opens the document at the item")
}

func TestCopySearchesAsTheServerDoes(t *testing.T) {
	t.Attr("verifies", "R60.1")
	files := map[string]string{
		".git/HEAD":              "",
		"tea/shall.yaml":         "name: tea\nformat: 1\n",
		"tea/mission.shall.yaml": mission,
		// product names the set standard company.
		"std/shall.yaml":             "name: standard\nformat: 1\n",
		"std/mission.shall.yaml":     mission,
		"product/shall.yaml":         "name: product\nformat: 1\ndepends:\n  company:\n    path: std\n",
		"product/mission.shall.yaml": "title: M\nmission: m\nitems:\n  - {id: S1, title: Buyer, text: Buys it.}\n",
		// unread has no files to read.
		"unread/shall.yaml": "name: unread\nformat: 1\npath: nowhere\n",
	}
	for name, src := range coffee() {
		files["coffee/"+name] = src
	}
	for name, src := range pageSet() {
		files["demo/"+name] = src
	}
	// odd holds the ids a checker refuses: two items at one path, one id
	// twice, and items at retired paths.
	files["odd/shall.yaml"] = "name: odd\nformat: 1\n"
	files["odd/mission.shall.yaml"] = `title: Odd
mission: Ids that clash.
items:
  - {id: S1, title: Clash One, text: First at path one.}
  - {id: N1, title: Clash Two, text: Second at path one.}
  - {id: S2, title: Twice, text: The first.}
  - {id: S2, title: Twice Again, text: The second.}
  - id: S3
    title: Gone
    text: At a retired path.
    children:
      - {id: S3.1, title: Gone Below, text: Below a retired path.}
  - {id: S4, title: İstanbul ΟΔΟΣ, text: Cased apart.}
  - id: S5
    title: Retired Twice
    text: Two entries retire one path.
    retired: [{id: R5.9, title: Above}]
    children:
      - {id: S5.1, title: Below, text: Holds the other., retired: [{id: N5.9, title: Below}]}
retired:
  - {id: R3, title: Retired Three}
`
	g := openCopy(t, files, "/shall/coffee/")
	h := New(g.root, true).Handler()
	for _, c := range []struct {
		set     string
		queries []string
	}{
		{"coffee", []string{
			"lamp", "Tank EMPTY", "r1", "R1.1", " r4 ", "brew", "machine shall", "Refill", "91", "o90", "S9",
			"tea/n91.1", "tea/N91.1", "tea/R7", "tea/", "coffee/R1", "coffee/r1", "milk/N91.1", "/", "a/b/c",
			"lamp door", "r2 brew", "refill\u0085lamp", "refill\ufefflamp", "\u0085lamp\ufeff", "a/b c", "R3", "r3", "N3", "x3", "R1.2", "r1.2", "R1.2.7", "N1.2", "r99", "R7", "", "   ", ".", "é3",
		}},
		{"demo", []string{
			"the", "product", "alpha", "beta draft", "gamma 45", "Need 3", "objective", "owner",
			"R13", "r13", "N13", "R13.4", "R20.2", "N99", "N65", "n65.1", "S66", "demo/R1",
		}},
		{"odd", []string{
			"clash", "twice", "gone", "S1", "N1", "n1", "s2", "S3", "S3.1", "R3", "N3.1", "odd/S2", "first",
			"istanbul", "οδοσ", "οδος", "R5.9", "N5.9",
		}},
		{"product", []string{
			"company/N91.1", "company/n91.1", "company/R7", "company/", "standard/N91.1", "constructor/S1", "product/s1",
		}},
		{"unread", []string{"lamp", "R1", "a/b"}},
	} {
		g.open("/shall/" + c.set + "/")
		for _, q := range c.queries {
			_, want := getFrom(t, h, "/api/sets/"+c.set+"/search?q="+url.QueryEscape(q), "localhost:7180")
			var got string
			g.eval(fmt.Sprintf(`searchCopy(%q).then((a) => JSON.stringify(a))`, q), &got)
			if unversioned(t, []byte(got)) != unversioned(t, []byte(want)) {
				t.Errorf("in %s the copy finds %q as\n%s\nwhere the server finds\n%s", c.set, q, unversioned(t, []byte(got)), unversioned(t, []byte(want)))
			}
		}
	}
}
