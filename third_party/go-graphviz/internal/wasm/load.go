package wasm

// This file is shall's patch to go-graphviz v0.2.10; README.shall.md says
// why and what else it changes.

import (
	"fmt"
	"sync"
)

var (
	loadOnce sync.Once
	loadErr  error
	onLoad   []func()
)

// OnLoad registers f to run once Graphviz is loaded, for the packages that
// set themselves up with it.
func OnLoad(f func()) {
	onLoad = append(onLoad, f)
}

// Load compiles and starts Graphviz the first time it is called, in memory,
// then runs every function OnLoad registered; later calls return what the
// first one did. Every entry point that needs Graphviz calls it.
func Load() error {
	loadOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				loadErr = fmt.Errorf("graphviz cannot start: %v", r)
			}
		}()
		load()
		for _, f := range onLoad {
			f()
		}
	})
	return loadErr
}
