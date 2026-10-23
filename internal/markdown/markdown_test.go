package markdown

import (
	"encoding/json"
	"testing"
)

// js writes blocks as compact JSON, for a failure message and a comparison.
func js(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBlocks(t *testing.T) {
	t.Attr("verifies", "R26.16")
	for _, c := range []struct{ name, src, want string }{
		{"a heading and a paragraph", "# Water\n\nThe tank *holds* **water**,\nand `pumps` it.\n",
			`[{"type":"heading","level":1,"inline":[{"type":"text","text":"Water"}]},` +
				`{"type":"paragraph","inline":[{"type":"text","text":"The tank "},{"type":"emphasis","inline":[{"type":"text","text":"holds"}]},{"type":"text","text":" "},{"type":"strong","inline":[{"type":"text","text":"water"}]},{"type":"text","text":", and "},{"type":"code","text":"pumps"},{"type":"text","text":" it."}]}]`},
		{"lists", "- one\n- two\n\n3. three\n4. four\n",
			`[{"type":"list","items":[[{"type":"paragraph","inline":[{"type":"text","text":"one"}]}],[{"type":"paragraph","inline":[{"type":"text","text":"two"}]}]]},` +
				`{"type":"list","ordered":true,"start":3,"items":[[{"type":"paragraph","inline":[{"type":"text","text":"three"}]}],[{"type":"paragraph","inline":[{"type":"text","text":"four"}]}]]}]`},
		{"a table", "| State | Means |\n|:---|---:|\n| empty | refill |\n",
			`[{"type":"table","align":["left","right"],"head":[[{"type":"text","text":"State"}],[{"type":"text","text":"Means"}]],"rows":[[[{"type":"text","text":"empty"}],[{"type":"text","text":"refill"}]]]}]`},
		{"code and a diagram", "```go\nx := 1\n```\n\n```mermaid\nstateDiagram-v2\n  Full --> Empty\n```\n",
			`[{"type":"code","language":"go","text":"x := 1\n"},{"type":"diagram","text":"stateDiagram-v2\n  Full --\u003e Empty\n"}]`},
		{"a quote and a rule", "> Said once.\n\n---\n",
			`[{"type":"quote","blocks":[{"type":"paragraph","inline":[{"type":"text","text":"Said once."}]}]},{"type":"rule"}]`},
		{"links, an image and a break", "[spec](https://x.example/s \"The spec\") ![a lamp](lamp.png) <https://y.example>  \nnext\n",
			`[{"type":"paragraph","inline":[{"type":"link","url":"https://x.example/s","title":"The spec","inline":[{"type":"text","text":"spec"}]},{"type":"text","text":" "},` +
				`{"type":"link","url":"lamp.png","inline":[{"type":"text","text":"a lamp"}]},{"type":"text","text":" "},{"type":"link","url":"https://y.example","inline":[{"type":"text","text":"https://y.example"}]},{"type":"break"},{"type":"text","text":"next"}]}]`},
		{"HTML stays text", "<div onclick=\"x()\">hi</div>\n\nA <b>bold</b> claim.\n",
			`[{"type":"html","text":"\u003cdiv onclick=\"x()\"\u003ehi\u003c/div\u003e\n"},{"type":"paragraph","inline":[{"type":"text","text":"A \u003cb\u003ebold\u003c/b\u003e claim."}]}]`},
		{"escapes and entities", "\\*not emphasis\\* &amp; &#65;\n",
			`[{"type":"paragraph","inline":[{"type":"text","text":"*not emphasis* \u0026 A"}]}]`},
		{"nothing", "", `[]`},
	} {
		if got := js(t, Blocks(c.src)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}
