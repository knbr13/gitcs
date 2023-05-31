package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "C:\\Users\\superComputer\\Documents" // Replace with the absolute root directory path

	var gitFolders []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if info.IsDir() && info.Name() == ".git" {
			gitFolder := filepath.Dir(path)
			gitFolders = append(gitFolders, gitFolder)
			return filepath.SkipDir // Skip further traversal within this directory
		}

		// Skip node_modules directories
		if info.IsDir() && strings.ToLower(info.Name()) == "node_modules" {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %q: %v\n", root, err)
	} else {
		fmt.Println("Folders containing .git:")
		for _, folder := range gitFolders {
			fmt.Println(folder)
		}
	}
}
