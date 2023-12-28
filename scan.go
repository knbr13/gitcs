package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/gookit/color"
)

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

		// Skip node_modules directories
		if d.IsDir() && (strings.ToLower(d.Name()) == "node_modules" || strings.ToLower(d.Name()) == "vendor") {
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
