package dot

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestADiagramDrawn(t *testing.T) {
	t.Attr("verifies", "R59")
	svg, err := SVG(`digraph {
  rankdir=LR
  node [shape=box]
  build -> test -> root
  { rank=same; test; a [label="jig A"]; b [label="jig B"] }
  test -> a [dir=none]
  test -> b [dir=none]
  a -> b [style=invis]
}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg"`) || !strings.Contains(svg, ">jig A<") || !strings.Contains(svg, "<polygon") {
		t.Errorf("svg = %.300s, want a drawing with jig A in it", svg)
	}
	for _, gone := range []string{"<?xml", "<!DOCTYPE", "<!--", "xlink"} {
		if strings.Contains(svg, gone) {
			t.Errorf("svg holds %q, want only the drawing", gone)
		}
	}
	if _, err := SVG("digraph { a -> }"); err == nil || !strings.Contains(err.Error(), "syntax error in line 1") {
		t.Errorf("a broken diagram: %v, want Graphviz's syntax error", err)
	}
}

func TestADrawingCleaned(t *testing.T) {
	t.Attr("verifies", "R59.1")
	got, err := Clean([]byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="10" onload="x()">
<script>alert(1)</script>
<g class="node" onclick="x()" style="fill:red">
<a xlink:href="https://elsewhere.example/" target="_blank"><text x="1" fill="black">kept</text></a>
<image href="https://elsewhere.example/a.png"/>
<foreignObject><div onclick="x()">html</div></foreignObject>
<rect fill="url(https://elsewhere.example/p)" stroke="url(#local)"/>
</g>
</svg>`))
	if err != nil {
		t.Fatal(err)
	}
	want := `<svg xmlns="http://www.w3.org/2000/svg" width="10">` + "\n\n" +
		`<g class="node">` + "\n" + `<text x="1" fill="black">kept</text>` + "\n\n\n" +
		`<rect stroke="url(#local)"></rect>` + "\n" + `</g>` + "\n" + `</svg>`
	if got != want {
		t.Errorf("cleaned =\n%s\nwant\n%s", got, want)
	}
}

func TestGraphvizStartsOnlyToDrawAndKeepsNothing(t *testing.T) {
	t.Attr("verifies", "R59.2")
	// A process of its own, with a temporary directory of its own, shows what
	// Graphviz does from the start of a program.
	tmp := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDrawingHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), "SHALL_DOT_HELPER=1", "TMPDIR="+tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the drawing process failed: %v\n%s", err, out)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("drawing left %s in the temporary directory", e.Name())
	}
}

// TestDrawingHelper is the process TestGraphvizStartsOnlyToDrawAndKeepsNothing
// starts: it fails when Graphviz started before the first drawing, which then
// allocates tens of megabytes compiling it.
func TestDrawingHelper(t *testing.T) {
	if os.Getenv("SHALL_DOT_HELPER") != "1" {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if before := m.TotalAlloc; before > 8<<20 {
		fmt.Fprintf(os.Stderr, "%d MB allocated before the first drawing; Graphviz started without being asked\n", before>>20)
		os.Exit(1)
	}
	if _, err := SVG("digraph { a -> b }"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
