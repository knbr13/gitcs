//go:build webembed

package main

import (
	"embed"
	"io/fs"
)

//go:embed frontend/dist
var embeddedMapAssets embed.FS

func mapAssets() (fs.FS, error) {
	return fs.Sub(embeddedMapAssets, "frontend/dist")
}
