# go-graphviz, as shall builds it

This is go-graphviz v0.2.10, from <https://github.com/goccy/go-graphviz>, with
its tests, examples and CI left out, and one change: Graphviz starts only when
it is first needed, and its compiled code stays in memory.

Upstream compiles Graphviz in `init`, so every program that links the
package pays for it at start, whether it draws anything or not. It also
keeps the compiled machine code under `os.TempDir()/go-graphviz`, in a
directory any user of the machine can write, and loads it back without
checking who wrote it.

The change:

- `internal/wasm/bind.go`: `init` becomes `load`, without the compilation
  cache.
- `internal/wasm/load.go`, new: `Load` runs `load` once, then the functions
  `OnLoad` registered, and turns a panic into an error.
- `cgraph/init.go`, `gvc/init.go` and `graphviz.go`: what their `init` did
  with Graphviz runs through `OnLoad`.
- `cgraph.ParseBytes`, `cgraph.Open`, `gvc.New` and `gvc.NewWithPlugins` call
  `Load` first.

`internal/wasm/build/` holds what upstream builds `graphviz.wasm` from.
