package dot

import (
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
