# Licences

shall is under the MIT licence in [LICENSE](../LICENSE). It is built from the
libraries below, each pinned to the version given: the Go modules by `go.mod`
and `go.sum`, the files the page uses by being embedded in the binary.

| Dependency | Version | Licence | In shall |
|---|---|---|---|
| Go standard library | 1.27.2 | [BSD-3-Clause](https://go.dev/LICENSE) | Compiled into the binary |
| github.com/goccy/go-yaml | v1.19.2 | [MIT](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE) | Compiled into the binary; reads YAML with the line of each value |
| github.com/yuin/goldmark | v1.7.17 | [MIT](https://github.com/yuin/goldmark/blob/v1.7.17/LICENSE) | Compiled into the binary; reads a topic's Markdown |
| github.com/goccy/go-graphviz | v0.2.10 | [MIT](https://github.com/goccy/go-graphviz/blob/v0.2.10/LICENSE) | Compiled into the binary from a copy in [third_party/go-graphviz](../third_party/go-graphviz/README.shall.md), changed so Graphviz starts only to draw; draws a topic's DOT diagrams with Graphviz |
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

## Inside the dependencies

These reach shall's binary inside a dependency's file above: compiled into
`graphviz.wasm`, or bundled into `mermaid.min.js` or `pure-min.css`.

| Library | Version | Licence | Inside |
|---|---|---|---|
| Expat | 2.6.3 | [MIT](https://github.com/libexpat/libexpat/blob/R_2_6_3/expat/COPYING) | `graphviz.wasm`, as go-graphviz's [build](../third_party/go-graphviz/internal/wasm/build/Dockerfile) compiles it |
| wasi-libc | as wasi-sdk 24 ships it | [Apache-2.0 with LLVM exception, Apache-2.0 or MIT; parts CC0, MIT, BSD-2-Clause and BSD-3-Clause](https://github.com/WebAssembly/wasi-libc/blob/main/LICENSE) | `graphviz.wasm`, linked by wasi-sdk 24 |
| [@antfu/install-pkg](https://www.npmjs.com/package/@antfu/install-pkg/v/1.1.0) | 1.1.0 | MIT | `mermaid.min.js` |
| [@antfu/utils](https://www.npmjs.com/package/@antfu/utils/v/9.2.0) | 9.2.0 | MIT | `mermaid.min.js` |
| [@braintree/sanitize-url](https://www.npmjs.com/package/@braintree/sanitize-url/v/7.1.2) | 7.1.2 | MIT | `mermaid.min.js` |
| [@chevrotain/cst-dts-gen](https://www.npmjs.com/package/@chevrotain/cst-dts-gen/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [@chevrotain/gast](https://www.npmjs.com/package/@chevrotain/gast/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [@chevrotain/regexp-to-ast](https://www.npmjs.com/package/@chevrotain/regexp-to-ast/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [@chevrotain/types](https://www.npmjs.com/package/@chevrotain/types/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [@chevrotain/utils](https://www.npmjs.com/package/@chevrotain/utils/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [@iconify/types](https://www.npmjs.com/package/@iconify/types/v/2.0.0) | 2.0.0 | MIT | `mermaid.min.js` |
| [@iconify/utils](https://www.npmjs.com/package/@iconify/utils/v/3.0.2) | 3.0.2 | MIT | `mermaid.min.js` |
| [@upsetjs/venn.js](https://www.npmjs.com/package/@upsetjs/venn.js/v/2.0.0) | 2.0.0 | MIT | `mermaid.min.js` |
| [acorn](https://www.npmjs.com/package/acorn/v/8.16.0) | 8.16.0 | MIT | `mermaid.min.js` |
| [chevrotain](https://www.npmjs.com/package/chevrotain/v/11.1.2) | 11.1.2 | Apache-2.0 | `mermaid.min.js` |
| [chevrotain-allstar](https://www.npmjs.com/package/chevrotain-allstar/v/0.3.1) | 0.3.1 | MIT | `mermaid.min.js` |
| [commander](https://www.npmjs.com/package/commander/v/7.2.0) | 7.2.0 | MIT | `mermaid.min.js` |
| [commander](https://www.npmjs.com/package/commander/v/8.3.0) | 8.3.0 | MIT | `mermaid.min.js` |
| [confbox](https://www.npmjs.com/package/confbox/v/0.1.8) | 0.1.8 | MIT | `mermaid.min.js` |
| [confbox](https://www.npmjs.com/package/confbox/v/0.2.2) | 0.2.2 | MIT | `mermaid.min.js` |
| [cose-base](https://www.npmjs.com/package/cose-base/v/1.0.3) | 1.0.3 | MIT | `mermaid.min.js` |
| [cose-base](https://www.npmjs.com/package/cose-base/v/2.2.0) | 2.2.0 | MIT | `mermaid.min.js` |
| [cytoscape](https://www.npmjs.com/package/cytoscape/v/3.34.0) | 3.34.0 | MIT | `mermaid.min.js` |
| [cytoscape-cose-bilkent](https://www.npmjs.com/package/cytoscape-cose-bilkent/v/4.1.0) | 4.1.0 | MIT | `mermaid.min.js` |
| [cytoscape-fcose](https://www.npmjs.com/package/cytoscape-fcose/v/2.2.0) | 2.2.0 | MIT | `mermaid.min.js` |
| [d3](https://www.npmjs.com/package/d3/v/7.9.0) | 7.9.0 | ISC | `mermaid.min.js` |
| [d3-array](https://www.npmjs.com/package/d3-array/v/2.12.1) | 2.12.1 | BSD-3-Clause | `mermaid.min.js` |
| [d3-array](https://www.npmjs.com/package/d3-array/v/3.2.4) | 3.2.4 | ISC | `mermaid.min.js` |
| [d3-axis](https://www.npmjs.com/package/d3-axis/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [d3-brush](https://www.npmjs.com/package/d3-brush/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [d3-chord](https://www.npmjs.com/package/d3-chord/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-color](https://www.npmjs.com/package/d3-color/v/3.1.0) | 3.1.0 | ISC | `mermaid.min.js` |
| [d3-contour](https://www.npmjs.com/package/d3-contour/v/4.0.2) | 4.0.2 | ISC | `mermaid.min.js` |
| [d3-delaunay](https://www.npmjs.com/package/d3-delaunay/v/6.0.4) | 6.0.4 | ISC | `mermaid.min.js` |
| [d3-dispatch](https://www.npmjs.com/package/d3-dispatch/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-drag](https://www.npmjs.com/package/d3-drag/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [d3-dsv](https://www.npmjs.com/package/d3-dsv/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-ease](https://www.npmjs.com/package/d3-ease/v/3.0.1) | 3.0.1 | BSD-3-Clause | `mermaid.min.js` |
| [d3-fetch](https://www.npmjs.com/package/d3-fetch/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-force](https://www.npmjs.com/package/d3-force/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [d3-format](https://www.npmjs.com/package/d3-format/v/3.1.0) | 3.1.0 | ISC | `mermaid.min.js` |
| [d3-geo](https://www.npmjs.com/package/d3-geo/v/3.1.1) | 3.1.1 | ISC | `mermaid.min.js` |
| [d3-hierarchy](https://www.npmjs.com/package/d3-hierarchy/v/3.1.2) | 3.1.2 | ISC | `mermaid.min.js` |
| [d3-interpolate](https://www.npmjs.com/package/d3-interpolate/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-path](https://www.npmjs.com/package/d3-path/v/1.0.9) | 1.0.9 | BSD-3-Clause | `mermaid.min.js` |
| [d3-path](https://www.npmjs.com/package/d3-path/v/3.1.0) | 3.1.0 | ISC | `mermaid.min.js` |
| [d3-polygon](https://www.npmjs.com/package/d3-polygon/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-quadtree](https://www.npmjs.com/package/d3-quadtree/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-random](https://www.npmjs.com/package/d3-random/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-sankey](https://www.npmjs.com/package/d3-sankey/v/0.12.3) | 0.12.3 | BSD-3-Clause | `mermaid.min.js` |
| [d3-scale](https://www.npmjs.com/package/d3-scale/v/4.0.2) | 4.0.2 | ISC | `mermaid.min.js` |
| [d3-scale-chromatic](https://www.npmjs.com/package/d3-scale-chromatic/v/3.1.0) | 3.1.0 | ISC | `mermaid.min.js` |
| [d3-selection](https://www.npmjs.com/package/d3-selection/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [d3-shape](https://www.npmjs.com/package/d3-shape/v/1.3.7) | 1.3.7 | BSD-3-Clause | `mermaid.min.js` |
| [d3-shape](https://www.npmjs.com/package/d3-shape/v/3.2.0) | 3.2.0 | ISC | `mermaid.min.js` |
| [d3-time](https://www.npmjs.com/package/d3-time/v/3.1.0) | 3.1.0 | ISC | `mermaid.min.js` |
| [d3-time-format](https://www.npmjs.com/package/d3-time-format/v/4.1.0) | 4.1.0 | ISC | `mermaid.min.js` |
| [d3-timer](https://www.npmjs.com/package/d3-timer/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-transition](https://www.npmjs.com/package/d3-transition/v/3.0.1) | 3.0.1 | ISC | `mermaid.min.js` |
| [d3-zoom](https://www.npmjs.com/package/d3-zoom/v/3.0.0) | 3.0.0 | ISC | `mermaid.min.js` |
| [dagre-d3-es](https://www.npmjs.com/package/dagre-d3-es/v/7.0.14) | 7.0.14 | MIT | `mermaid.min.js` |
| [dayjs](https://www.npmjs.com/package/dayjs/v/1.11.21) | 1.11.21 | MIT | `mermaid.min.js` |
| [debug](https://www.npmjs.com/package/debug/v/4.4.3) | 4.4.3 | MIT | `mermaid.min.js` |
| [delaunator](https://www.npmjs.com/package/delaunator/v/5.0.1) | 5.0.1 | ISC | `mermaid.min.js` |
| [dompurify](https://www.npmjs.com/package/dompurify/v/3.4.12) | 3.4.12 | MPL-2.0 or Apache-2.0 | `mermaid.min.js` |
| [es-toolkit](https://www.npmjs.com/package/es-toolkit/v/1.45.1) | 1.45.1 | MIT | `mermaid.min.js` |
| [exsolve](https://www.npmjs.com/package/exsolve/v/1.0.7) | 1.0.7 | MIT | `mermaid.min.js` |
| [fastdom](https://www.npmjs.com/package/fastdom/v/1.0.12) | 1.0.12 | MIT | `mermaid.min.js` |
| [globals](https://www.npmjs.com/package/globals/v/15.15.0) | 15.15.0 | MIT | `mermaid.min.js` |
| [hachure-fill](https://www.npmjs.com/package/hachure-fill/v/0.5.2) | 0.5.2 | MIT | `mermaid.min.js` |
| [iconv-lite](https://www.npmjs.com/package/iconv-lite/v/0.6.3) | 0.6.3 | MIT | `mermaid.min.js` |
| [internmap](https://www.npmjs.com/package/internmap/v/1.0.1) | 1.0.1 | ISC | `mermaid.min.js` |
| [internmap](https://www.npmjs.com/package/internmap/v/2.0.3) | 2.0.3 | ISC | `mermaid.min.js` |
| [katex](https://www.npmjs.com/package/katex/v/0.16.47) | 0.16.47 | MIT | `mermaid.min.js` |
| [khroma](https://www.npmjs.com/package/khroma/v/2.1.0) | 2.1.0 | MIT, in its repository; its package declares none | `mermaid.min.js` |
| [kolorist](https://www.npmjs.com/package/kolorist/v/1.8.0) | 1.8.0 | MIT | `mermaid.min.js` |
| [langium](https://www.npmjs.com/package/langium/v/4.2.1) | 4.2.1 | MIT | `mermaid.min.js` |
| [layout-base](https://www.npmjs.com/package/layout-base/v/1.0.2) | 1.0.2 | MIT | `mermaid.min.js` |
| [layout-base](https://www.npmjs.com/package/layout-base/v/2.0.1) | 2.0.1 | MIT | `mermaid.min.js` |
| [local-pkg](https://www.npmjs.com/package/local-pkg/v/1.1.2) | 1.1.2 | MIT | `mermaid.min.js` |
| [lodash-es](https://www.npmjs.com/package/lodash-es/v/4.17.23) | 4.17.23 | MIT | `mermaid.min.js` |
| [lodash-es](https://www.npmjs.com/package/lodash-es/v/4.18.1) | 4.18.1 | MIT | `mermaid.min.js` |
| [marked](https://www.npmjs.com/package/marked/v/16.3.0) | 16.3.0 | MIT | `mermaid.min.js` |
| [mlly](https://www.npmjs.com/package/mlly/v/1.8.0) | 1.8.0 | MIT | `mermaid.min.js` |
| [ms](https://www.npmjs.com/package/ms/v/2.1.3) | 2.1.3 | MIT | `mermaid.min.js` |
| [package-manager-detector](https://www.npmjs.com/package/package-manager-detector/v/1.3.0) | 1.3.0 | MIT | `mermaid.min.js` |
| [path-data-parser](https://www.npmjs.com/package/path-data-parser/v/0.1.0) | 0.1.0 | MIT | `mermaid.min.js` |
| [pathe](https://www.npmjs.com/package/pathe/v/2.0.3) | 2.0.3 | MIT | `mermaid.min.js` |
| [pkg-types](https://www.npmjs.com/package/pkg-types/v/1.3.1) | 1.3.1 | MIT | `mermaid.min.js` |
| [pkg-types](https://www.npmjs.com/package/pkg-types/v/2.3.0) | 2.3.0 | MIT | `mermaid.min.js` |
| [points-on-curve](https://www.npmjs.com/package/points-on-curve/v/0.2.0) | 0.2.0 | MIT | `mermaid.min.js` |
| [points-on-path](https://www.npmjs.com/package/points-on-path/v/0.2.1) | 0.2.1 | MIT | `mermaid.min.js` |
| [quansync](https://www.npmjs.com/package/quansync/v/0.2.11) | 0.2.11 | MIT | `mermaid.min.js` |
| [robust-predicates](https://www.npmjs.com/package/robust-predicates/v/3.0.2) | 3.0.2 | Unlicense | `mermaid.min.js` |
| [roughjs](https://www.npmjs.com/package/roughjs/v/4.6.6) | 4.6.6 | MIT | `mermaid.min.js` |
| [rw](https://www.npmjs.com/package/rw/v/1.3.3) | 1.3.3 | BSD-3-Clause | `mermaid.min.js` |
| [safer-buffer](https://www.npmjs.com/package/safer-buffer/v/2.1.2) | 2.1.2 | MIT | `mermaid.min.js` |
| [strictdom](https://www.npmjs.com/package/strictdom/v/1.0.1) | 1.0.1 | MIT | `mermaid.min.js` |
| [stylis](https://www.npmjs.com/package/stylis/v/4.3.6) | 4.3.6 | MIT | `mermaid.min.js` |
| [tinyexec](https://www.npmjs.com/package/tinyexec/v/1.2.4) | 1.2.4 | MIT | `mermaid.min.js` |
| [ts-dedent](https://www.npmjs.com/package/ts-dedent/v/2.2.0) | 2.2.0 | MIT | `mermaid.min.js` |
| [ufo](https://www.npmjs.com/package/ufo/v/1.6.1) | 1.6.1 | MIT | `mermaid.min.js` |
| [uuid](https://www.npmjs.com/package/uuid/v/14.0.0) | 14.0.0 | MIT | `mermaid.min.js` |
| [vscode-jsonrpc](https://www.npmjs.com/package/vscode-jsonrpc/v/8.2.0) | 8.2.0 | MIT | `mermaid.min.js` |
| [vscode-languageserver](https://www.npmjs.com/package/vscode-languageserver/v/9.0.1) | 9.0.1 | MIT | `mermaid.min.js` |
| [vscode-languageserver-protocol](https://www.npmjs.com/package/vscode-languageserver-protocol/v/3.17.5) | 3.17.5 | MIT | `mermaid.min.js` |
| [vscode-languageserver-textdocument](https://www.npmjs.com/package/vscode-languageserver-textdocument/v/1.0.12) | 1.0.12 | MIT | `mermaid.min.js` |
| [vscode-languageserver-types](https://www.npmjs.com/package/vscode-languageserver-types/v/3.17.5) | 3.17.5 | MIT | `mermaid.min.js` |
| [vscode-uri](https://www.npmjs.com/package/vscode-uri/v/3.1.0) | 3.1.0 | MIT | `mermaid.min.js` |
| normalize.css | 8.0.1 | [MIT](https://github.com/necolas/normalize.css/blob/master/LICENSE.md) | `pure-min.css` |

Mermaid's rows are the libraries its release depends on at run time, as its
lockfile at the release tag names them, together with chevrotain and langium,
which its parser bundles; type definitions, which hold no code, are left out.
The bundle holds what of each Mermaid uses, and each licence is the one the
library's package declares for that version.

## Files taken as released

Each file below is as its source released it, and the test of this record
checks its SHA-256.

| File | Source | SHA-256 |
|---|---|---|
| `internal/serve/web/static/mermaid/mermaid.min.js` | `dist/mermaid.min.js` of [mermaid 11.17.2 on npm](https://www.npmjs.com/package/mermaid/v/11.17.2) | `581ed7d74bd9048d0e3a91363927d72ef22942d7722546b27f7cc29e35390eb8` |
| `internal/serve/web/static/mermaid/LICENSE` | `LICENSE` of the same package | `ec9fb67dcb25eccc416ed56e1aab819222c805a2a4bfe4cb19e7556bf2ffde80` |
| `internal/serve/web/static/pure/pure-min.css` | `build/pure-min.css` of [purecss 3.1.0 on npm](https://www.npmjs.com/package/purecss/v/3.1.0) | `09e55dd770f449aabdb59ac23f60d0c04f3f5bc755a2117bcedfac5625e53a91` |
| `internal/serve/web/static/pure/LICENSE` | `LICENSE` of the same package | `66fe80191874ef13229c0f0f87d28ac25f0942b32c002a1fe49d9cd331e6b3dc` |
| `third_party/go-graphviz/internal/wasm/graphviz.wasm` | the same file of github.com/goccy/go-graphviz v0.2.10 | `7cc5b0a5f176aeaa69760ed5dc3282ceda2b1b04229b22de77bb0a5eab45578e` |
