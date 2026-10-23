# Licences

shall is under the MIT licence in [LICENSE](../LICENSE). It is built from the
libraries below, each pinned to the version given: the Go modules by `go.mod`
and `go.sum`, the files the page uses by being embedded in the binary.

| Dependency | Version | Licence | In shall |
|---|---|---|---|
| Go standard library | 1.27.2 | [BSD-3-Clause](https://go.dev/LICENSE) | Compiled into the binary |
| github.com/goccy/go-yaml | v1.19.2 | [MIT](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE) | Compiled into the binary; reads YAML with the line of each value |
| github.com/yuin/goldmark | v1.7.17 | [MIT](https://github.com/yuin/goldmark/blob/v1.7.17/LICENSE) | Compiled into the binary; reads a topic's Markdown |
| Pure.css | 3.1.0 | [BSD](../internal/serve/web/static/pure/LICENSE) | Embedded, and served with its licence beside it |
| Mermaid | 11.17.2 | [MIT](../internal/serve/web/static/mermaid/LICENSE) | Embedded, and served with its licence beside it |
