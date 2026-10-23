// Package dot draws diagrams written in Graphviz's DOT language as SVG,
// with Graphviz run inside the binary, and cleans the drawing down to its
// shapes, text and styles.
package dot

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"sync"

	graphviz "github.com/goccy/go-graphviz"
)

var (
	mu sync.Mutex
	gv *graphviz.Graphviz
)

// SVG returns a DOT diagram drawn as SVG, or why it cannot be drawn.
func SVG(src string) (string, error) {
	graph, err := graphviz.ParseBytes([]byte(src))
	if err != nil {
		return "", err
	}
	defer graph.Close()
	mu.Lock()
	defer mu.Unlock()
	ctx := context.Background()
	if gv == nil {
		if gv, err = graphviz.New(ctx); err != nil {
			gv = nil
			return "", err
		}
	}
	var b bytes.Buffer
	if err := gv.Render(ctx, graph, graphviz.SVG, &b); err != nil {
		return "", err
	}
	return Clean(b.Bytes())
}

// shapes are the elements a drawing keeps, and the attributes each may
// carry are those named in attrs.
var shapes = map[string]bool{
	"svg": true, "g": true, "title": true, "polygon": true, "polyline": true, "path": true,
	"ellipse": true, "circle": true, "rect": true, "line": true, "text": true, "tspan": true,
	"defs": true, "linearGradient": true, "radialGradient": true, "stop": true,
}

var attrs = map[string]bool{
	"width": true, "height": true, "viewBox": true, "transform": true, "id": true, "class": true,
	"points": true, "d": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true,
	"x": true, "y": true, "x1": true, "y1": true, "x2": true, "y2": true, "dx": true, "dy": true,
	"fill": true, "fill-opacity": true, "stroke": true, "stroke-width": true, "stroke-opacity": true,
	"stroke-dasharray": true, "stroke-linecap": true, "stroke-linejoin": true, "opacity": true,
	"font-family": true, "font-size": true, "font-weight": true, "font-style": true,
	"text-anchor": true, "text-decoration": true, "offset": true, "stop-color": true,
	"stop-opacity": true, "gradientUnits": true, "gradientTransform": true, "fx": true, "fy": true,
	"preserveAspectRatio": true,
}

// Clean returns an SVG drawing with only its shapes, text and styles: no
// script, link, image or anything else that runs or loads from elsewhere.
// A link keeps what it holds; any other element not kept goes with
// everything inside it. A value may point only within the drawing.
func Clean(svg []byte) (string, error) {
	const (
		kept = iota
		unwrapped
		dropped
	)
	d := xml.NewDecoder(bytes.NewReader(svg))
	d.Strict = false
	var b strings.Builder
	e := xml.NewEncoder(&b)
	var open []int
	inside := 0 // elements open within a dropped one, itself included
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case inside > 0 || !shapes[t.Name.Local] && t.Name.Local != "a":
				inside++
				open = append(open, dropped)
			case t.Name.Local == "a":
				open = append(open, unwrapped)
			default:
				open = append(open, kept)
				if err := e.EncodeToken(cleaned(t)); err != nil {
					return "", err
				}
			}
		case xml.EndElement:
			state := open[len(open)-1]
			open = open[:len(open)-1]
			switch state {
			case kept:
				if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: t.Name.Local}}); err != nil {
					return "", err
				}
			case dropped:
				inside--
			}
		case xml.CharData:
			if inside == 0 && len(open) > 0 {
				if err := e.EncodeToken(t.Copy()); err != nil {
					return "", err
				}
			}
		}
	}
	if err := e.Flush(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// cleaned returns an element with only the attributes a drawing keeps.
func cleaned(t xml.StartElement) xml.StartElement {
	out := xml.StartElement{Name: xml.Name{Local: t.Name.Local}}
	if t.Name.Local == "svg" {
		out.Attr = append(out.Attr, xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"})
	}
	for _, a := range t.Attr {
		v := strings.TrimSpace(a.Value)
		if a.Name.Space != "" || !attrs[a.Name.Local] || strings.Contains(v, "url(") && !strings.HasPrefix(v, "url(#") {
			continue
		}
		out.Attr = append(out.Attr, xml.Attr{Name: xml.Name{Local: a.Name.Local}, Value: v})
	}
	return out
}
