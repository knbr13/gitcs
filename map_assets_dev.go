//go:build !webembed

package main

import (
	"fmt"
	"io/fs"
	"os"
)

func mapAssets() (fs.FS, error) {
	assets := os.DirFS("frontend/dist")
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		return nil, fmt.Errorf("Svelte map is not built; run npm run build in frontend: %w", err)
	}
	return assets, nil
}
