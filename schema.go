// Package shall embeds the format's JSON Schema into every binary built from
// this module.
package shall

import (
	"embed"
	"io/fs"
)

//go:embed schema/format-1/*.json
var schemaFiles embed.FS

// Schema returns the JSON Schema files of format 1.
func Schema() fs.FS {
	sub, err := fs.Sub(schemaFiles, "schema/format-1")
	if err != nil {
		panic(err)
	}
	return sub
}
