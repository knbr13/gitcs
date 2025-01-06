package main

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func scanGitFolders(root string) ([]string, error) {
	var gitFolders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			gitFolder := filepath.Dir(path)
			gitFolders = append(gitFolders, gitFolder)
			return filepath.SkipDir // Skip further traversal within this directory
		}

		// Skip dependency directories
		_, ok := excludedFolders[strings.ToLower(d.Name())]
		if d.IsDir() && ok {
			return filepath.SkipDir
		}

		return nil
	})

	return gitFolders, err
}

// excludedFolders is a map of folder names (case-insensitive) to be excluded during the scan.
var excludedFolders = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	".svn":         {},
	".hg":          {},
	".bzr":         {},
	"_vendor":      {},
	"godeps":       {},
	"bin":          {},
	"obj":          {},
	"tmp":          {},
	"build":        {},
	".vscode":      {},
	"dist":         {},
	"__pycache__":  {},
	".cache":       {},
	"coverage":     {},
	"target":       {},
	"out":          {},
	".idea":        {},
	".gradle":      {},
	".terraform":   {},
	"env":          {},
	".ds_store":    {},
	".next":        {},
	".nuxt":        {},
	".expo":        {},
	".circleci":    {},
	".github":      {},
	".gitlab":      {},
	".vagrant":     {},
	".serverless":  {},
}
