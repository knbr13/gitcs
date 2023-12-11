package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/color"
)

func scanGitFolders(root string) ([]string, error) {
	var gitFolders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			color.Red.Printf("Error accessing path %q: %v\n", path, err)
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			gitFolder := filepath.Dir(path)
			gitFolders = append(gitFolders, gitFolder)
			return filepath.SkipDir // Skip further traversal within this directory
		}

		// Skip node_modules directories
		if info.IsDir() && (strings.ToLower(info.Name()) == "node_modules" || strings.ToLower(info.Name()) == "vendor") {
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
