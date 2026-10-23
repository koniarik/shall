// Package markdown reads a topic's Markdown into blocks of text, which the
// page builds as text: CommonMark with GitHub's tables, HTML kept as text,
// an image as a link to it, and a code block marked mermaid or dot as a
// diagram.
package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension"
	tast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Block is one block of a text: a heading, paragraph, list, quote, code,
// diagram, table, rule or html, as Type says.
type Block struct {
	Type string `json:"type"`
	// Level is a heading's, from 1.
	Level int `json:"level,omitempty"`
	// Inline is a heading's or a paragraph's text.
	Inline []Inline `json:"inline,omitempty"`
	// Ordered, Start and Items are a list's: whether it is numbered, its
	// first number, and each item's blocks.
	Ordered bool      `json:"ordered,omitempty"`
	Start   int       `json:"start,omitempty"`
	Items   [][]Block `json:"items,omitempty"`
	// Blocks are a quote's.
	Blocks []Block `json:"blocks,omitempty"`
	// Language is a code block's, as its fence names it, and a diagram's:
	// mermaid or dot.
	Language string `json:"language,omitempty"`
	// Text is a code block's, a diagram's or the HTML's, as written.
	Text string `json:"text,omitempty"`
	// Align, Head and Rows are a table's: each column's alignment, left,
	// center, right or "", the header's cells and the rows' cells.
	Align []string     `json:"align,omitempty"`
	Head  [][]Inline   `json:"head,omitempty"`
	Rows  [][][]Inline `json:"rows,omitempty"`
	// SVG is a diagram drawn by whoever reads the blocks, or Error why it
	// cannot be.
	SVG   string `json:"svg,omitempty"`
	Error string `json:"error,omitempty"`
}

// Inline is a run of text within a block: text, emphasis, strong, code, a
// link or a break, as Type says; or ref, an id cited, which a reader of
// the blocks resolves.
type Inline struct {
	Type string `json:"type"`
	// Text is the text of text and of code.
	Text string `json:"text,omitempty"`
	// URL and Title are a link's.
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
	// Inline is what emphasis, strong and a link hold.
	Inline []Inline `json:"inline,omitempty"`
	// Ref is what a cited id names, as the reader resolves it.
	Ref any `json:"ref,omitempty"`
}

var parser = goldmark.New(goldmark.WithExtensions(east.Table)).Parser()

// Blocks returns the blocks of a Markdown text.
func Blocks(src string) []Block {
	source := []byte(src)
	doc := parser.Parse(text.NewReader(source))
	return blocks(doc, source)
}

func blocks(parent ast.Node, source []byte) []Block {
	out := []Block{}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		out = append(out, block(n, source))
	}
	return out
}

func block(n ast.Node, source []byte) Block {
	switch n := n.(type) {
	case *ast.Heading:
		return Block{Type: "heading", Level: n.Level, Inline: inlines(n, source)}
	case *ast.Paragraph, *ast.TextBlock:
		return Block{Type: "paragraph", Inline: inlines(n, source)}
	case *ast.List:
		b := Block{Type: "list", Ordered: n.IsOrdered(), Items: [][]Block{}}
		if b.Ordered {
			b.Start = n.Start
		}
		for it := n.FirstChild(); it != nil; it = it.NextSibling() {
			b.Items = append(b.Items, blocks(it, source))
		}
		return b
	case *ast.Blockquote:
		return Block{Type: "quote", Blocks: blocks(n, source)}
	case *ast.FencedCodeBlock:
		language := string(n.Language(source))
		if language == "mermaid" || language == "dot" {
			return Block{Type: "diagram", Language: language, Text: lines(n, source)}
		}
		return Block{Type: "code", Language: language, Text: lines(n, source)}
	case *ast.CodeBlock:
		return Block{Type: "code", Text: lines(n, source)}
	case *ast.HTMLBlock:
		s := lines(n, source)
		if n.HasClosure() {
			s += string(n.ClosureLine.Value(source))
		}
		return Block{Type: "html", Text: s}
	case *ast.ThematicBreak:
		return Block{Type: "rule"}
	case *tast.Table:
		b := Block{Type: "table", Rows: [][][]Inline{}}
		for _, a := range n.Alignments {
			b.Align = append(b.Align, align(a))
		}
		for row := n.FirstChild(); row != nil; row = row.NextSibling() {
			var cells [][]Inline
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				cells = append(cells, inlines(cell, source))
			}
			if _, head := row.(*tast.TableHeader); head {
				b.Head = cells
			} else {
				b.Rows = append(b.Rows, cells)
			}
		}
		return b
	}
	// A block this package does not know reads as a paragraph of its text.
	return Block{Type: "paragraph", Inline: inlines(n, source)}
}

func align(a tast.Alignment) string {
	switch a {
	case tast.AlignLeft:
		return "left"
	case tast.AlignCenter:
		return "center"
	case tast.AlignRight:
		return "right"
	}
	return ""
}

// lines returns a block's lines as written.
func lines(n ast.Node, source []byte) string {
	var b strings.Builder
	ls := n.Lines()
	for i := 0; i < ls.Len(); i++ {
		seg := ls.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

func inlines(parent ast.Node, source []byte) []Inline {
	var out []Inline
	add := func(x Inline) {
		if x.Type == "text" && x.Text == "" {
			return
		}
		if x.Type == "text" && len(out) > 0 && out[len(out)-1].Type == "text" {
			out[len(out)-1].Text += x.Text
			return
		}
		out = append(out, x)
	}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			add(Inline{Type: "text", Text: plain(n.Segment.Value(source), n.IsRaw())})
			switch {
			case n.HardLineBreak():
				add(Inline{Type: "break"})
			case n.SoftLineBreak():
				add(Inline{Type: "text", Text: " "})
			}
		case *ast.String:
			add(Inline{Type: "text", Text: plain(n.Value, n.IsRaw())})
		case *ast.CodeSpan:
			var b strings.Builder
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					b.Write(t.Segment.Value(source))
				} else if s, ok := c.(*ast.String); ok {
					b.Write(s.Value)
				}
			}
			add(Inline{Type: "code", Text: b.String()})
		case *ast.Emphasis:
			kind := "emphasis"
			if n.Level > 1 {
				kind = "strong"
			}
			add(Inline{Type: kind, Inline: inlines(n, source)})
		case *ast.Link:
			add(Inline{Type: "link", URL: plain(n.Destination, false), Title: plain(n.Title, false), Inline: inlines(n, source)})
		case *ast.Image:
			label := inlines(n, source)
			if len(label) == 0 {
				label = []Inline{{Type: "text", Text: plain(n.Destination, false)}}
			}
			add(Inline{Type: "link", URL: plain(n.Destination, false), Title: plain(n.Title, false), Inline: label})
		case *ast.AutoLink:
			url := string(n.URL(source))
			add(Inline{Type: "link", URL: url, Inline: []Inline{{Type: "text", Text: string(n.Label(source))}}})
		case *ast.RawHTML:
			var b strings.Builder
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				b.Write(seg.Value(source))
			}
			add(Inline{Type: "text", Text: b.String()})
		default:
			for _, x := range inlines(n, source) {
				add(x)
			}
		}
	}
	if out == nil {
		return []Inline{}
	}
	return out
}

// plain returns written text as it reads: escapes and entities resolved.
func plain(b []byte, raw bool) string {
	if raw {
		return string(b)
	}
	return string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(b))))
}

// Prose returns the runs of text a Markdown text reads as, leaving out its
// code, diagrams and HTML: the text in which it can cite.
func Prose(src string) []string {
	var out []string
	var inline func([]Inline)
	inline = func(list []Inline) {
		for _, x := range list {
			switch x.Type {
			case "text":
				out = append(out, x.Text)
			case "emphasis", "strong", "link":
				inline(x.Inline)
			}
		}
	}
	var walk func([]Block)
	walk = func(list []Block) {
		for _, b := range list {
			inline(b.Inline)
			for _, it := range b.Items {
				walk(it)
			}
			walk(b.Blocks)
			for _, c := range b.Head {
				inline(c)
			}
			for _, row := range b.Rows {
				for _, c := range row {
					inline(c)
				}
			}
		}
	}
	walk(Blocks(src))
	return out
}
