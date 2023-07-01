package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gookit/color"
)

func getDotFilePath() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(color.Red.Sprintf("Error: %v\n", err))
	}

	dotFile := usr.HomeDir + "/.gogitlocalstats"

	return dotFile
}

func scanGitFolders(root string) ([]string, error) {
	var gitFolders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			color.Red.Printf("Error accessing path %q: %v\n", path, err)
			return nil
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
	} else {
		return gitFolders, nil
	}
}

// parseFileLinesToSlice given a file path string, gets the content
// of each line and parses it to a slice of strings.
func parseFileLinesToSlice(filePath string) ([]string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s", err)
	}
	defer file.Close()

	// Create a scanner to read the file
	scanner := bufio.NewScanner(file)

	// Create a slice to store the lines
	var lines []string

	// Read each line of the file
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error while scanning file: %s", err)
	}

	return lines, nil
}

// dumpStringsSliceToFile writes content to the file in path `filePath` (overwriting existing content)
func dumpStringsSliceToFile(lines []string, filePath string) error {
	// Join the strings into a single string with newline separators
	content := strings.Join(lines, "\n")

	// Write the content to the file, overwriting the existing content
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err)
	}

	return nil
}

// scan scans a new folder for Git repositories
func scan(folder string) {
	// Get the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(color.Red.Sprintf("Error: %v\n", err))
	}

	err = createFileIfNotExist(".gogitlocalstats", homeDir)
	if err != nil {
		log.Fatal(color.Red.Sprintf("Error: %v\n", err))
	}
	repositories, err := scanGitFolders(folder)
	if err != nil {
		log.Fatal(color.Red.Sprintf("Error: %v\n", err))
	}
	filePath := getDotFilePath()
	dumpStringsSliceToFile(repositories, filePath)
}
