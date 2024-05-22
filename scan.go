package main

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// excludedFolders is a map of folder names (case-insensitive) to be excluded during the scan.
var excludedFolders = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".svn":         true,
	".hg":          true,
	".bzr":         true,
	"_vendor":      true,
	"godeps":       true,
	"bin":          true,
	"obj":          true,
	"tmp":          true,
	"build":        true,
	".vscode":      true,
	"dist":         true,
	"__pycache__":  true,
	".cache":       true,
	"coverage":     true,
	"target":       true,
	"out":          true,
	".idea":        true,
	".gradle":      true,
	".terraform":   true,
	"env":          true,
	".ds_store":    true,
	".next":        true,
	".nuxt":        true,
	".expo":        true,
	".circleci":    true,
	".github":      true,
	".gitlab":      true,
	".vagrant":     true,
	".serverless":  true,
}

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
		if d.IsDir() && excludedFolders[strings.ToLower(d.Name())] {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return gitFolders, nil
}
