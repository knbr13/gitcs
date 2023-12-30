package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gookit/color"
)

var excludedFolders = []string{
	"node_modules",
	"vendor",
	// ".git", // already excluded
	".svn",
	".hg",
	".bzr",
	"_vendor",
	"godeps",
	"thirdparty",
	"bin",
	"obj",
	"testdata",
	"examples",
	"tmp",
	"build",
	// ...
}

func scanGitFolders(root string) ([]string, error) {
	var gitFolders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			color.Red.Printf("Error accessing path %q: %v\n", path, err)
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			gitFolder := filepath.Dir(path)
			gitFolders = append(gitFolders, gitFolder)
			return filepath.SkipDir // Skip further traversal within this directory
		}

		// Skip dependency directories // not needed + will slow down the tool
		if d.IsDir() && slices.Contains(excludedFolders, strings.ToLower(d.Name())) {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		color.Red.Printf("Error walking the path %q: %v\n", root, err)
		return nil, err
	}
	return gitFolders, nil
}

func scan(folder string) []string {
	repositories, err := scanGitFolders(folder)
	if err != nil {
		log.Fatal(color.Red.Sprintf("Error: %v\n", err))
	}
	return repositories
}
