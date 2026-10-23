# Licences

shall is under the MIT licence in [LICENSE](../LICENSE). It is built from the
libraries below, each pinned to the version given: the Go modules by `go.mod`
and `go.sum`, the files the page uses by being embedded in the binary.

| Dependency | Version | Licence | In shall |
|---|---|---|---|
| Go standard library | 1.27.2 | [BSD-3-Clause](https://go.dev/LICENSE) | Compiled into the binary |
| github.com/goccy/go-yaml | v1.19.2 | [MIT](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE) | Compiled into the binary; reads YAML with the line of each value |
| github.com/yuin/goldmark | v1.7.17 | [MIT](https://github.com/yuin/goldmark/blob/v1.7.17/LICENSE) | Compiled into the binary; reads a topic's Markdown |
| github.com/goccy/go-graphviz | v0.2.10 | [MIT](https://github.com/goccy/go-graphviz/blob/v0.2.10/LICENSE) | Compiled into the binary; draws a topic's DOT diagrams with Graphviz |
| Graphviz | 12.1.2 | [EPL-1.0](https://gitlab.com/graphviz/graphviz/-/blob/12.1.2/LICENSE) | Inside go-graphviz, compiled to WebAssembly; its source is at [gitlab.com/graphviz/graphviz](https://gitlab.com/graphviz/graphviz) |
| github.com/tetratelabs/wazero | v1.10.1 | [Apache-2.0](https://github.com/tetratelabs/wazero/blob/v1.10.1/LICENSE) | Compiled into the binary; runs Graphviz's WebAssembly for go-graphviz |
| github.com/fogleman/gg | v1.3.0 | [MIT](https://github.com/fogleman/gg/blob/v1.3.0/LICENSE.md) | Required by go-graphviz |
| github.com/golang/freetype | v0.0.0-20170609003504-e2365dfdc4a0 | [FreeType License](https://github.com/golang/freetype/blob/e2365dfdc4a0/LICENSE), chosen over GPL-2.0 | Required by go-graphviz |
| github.com/flopp/go-findfont | v0.1.0 | [MIT](https://github.com/flopp/go-findfont/blob/v0.1.0/LICENSE) | Required by go-graphviz |
| github.com/disintegration/imaging | v1.6.2 | [MIT](https://github.com/disintegration/imaging/blob/v1.6.2/LICENSE) | Required by go-graphviz |
| golang.org/x/image | v0.21.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/image/+/refs/tags/v0.21.0:LICENSE) | Required by go-graphviz |
| golang.org/x/text | v0.19.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/text/+/refs/tags/v0.19.0:LICENSE) | Required by go-graphviz |
| Pure.css | 3.1.0 | [BSD](../internal/serve/web/static/pure/LICENSE) | Embedded, and served with its licence beside it |
| Mermaid | 11.17.2 | [MIT](../internal/serve/web/static/mermaid/LICENSE) | Embedded, and served with its licence beside it |
