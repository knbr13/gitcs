package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func getDotFilePath() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	dotFile := usr.HomeDir + "/.gogitlocalstats"

	return dotFile
}

func scanGitFolders(root string) ([]string, error) {
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
		if info.IsDir() && (strings.ToLower(info.Name()) == "node_modules" || strings.ToLower(info.Name()) == "vendor") {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %q: %v\n", root, err)
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

func sliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// joinSlices adds the element of the `new` slice
// into the `existing` slice, only if not already there
func joinSlices(new []string, existing []string) []string {
	for _, i := range new {
		if !sliceContains(existing, i) {
			existing = append(existing, i)
		}
	}
	return existing
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

// addNewSliceElementsToFile given a slice of strings representing paths, stores them
// to the filesystem
func addNewSliceElementsToFile(filePath string, newRepos []string) {
	existingRepos, err := parseFileLinesToSlice(filePath)
	if err != nil {
		log.Fatal(err)
	}
	repos := joinSlices(newRepos, existingRepos)
	dumpStringsSliceToFile(repos, filePath)
}

// scan scans a new folder for Git repositories
func scan(folder string) {
	err := createFileIfNotExist(".gogitlocalstats")
	if err != nil {
		log.Fatal(err)
	}
	repositories, err := scanGitFolders(folder)
	if err != nil {
		log.Fatal(err)
	}
	filePath := getDotFilePath()
	addNewSliceElementsToFile(filePath, repositories)
}

func createFileIfNotExist(fileName string) error {
	// Get the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %s", err)
	}

	filePath := filepath.Join(homeDir, fileName)

	// Check if the file exists
	_, err = os.Stat(filePath)
	if os.IsNotExist(err) {
		// Create the file if it doesn't exist
		_, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %s", err)
		}
	}

	return nil
}
