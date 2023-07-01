package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/color"
)

func deleteFile(fileName, filePath string) error {
	err := os.Remove(filepath.Join(filePath, fileName))
	if err != nil {
		return fmt.Errorf("failed to delete %s file: %s", fileName, err)
	}

	return nil
}

func createFileIfNotExist(fileName string, dir string) error {

	filePath := filepath.Join(dir, fileName)

	// Check if the file exists
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// Create the file if it doesn't exist
		_, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %s", err)
		}
	}

	return nil
}

func isValidFolderPath(folder string) bool {
	// Check if the folder exists and is a directory
	info, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatal(err)
	}

	return info.IsDir()
}

func getFolderFromUser(reader *bufio.Reader) string {
	for {
		fmt.Print("Enter the folder path to scan for Git repositories: ")
		folder, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		folder = strings.TrimSpace(folder)

		if isValidFolderPath(folder) {
			return folder
		}

		color.Red.Println("Invalid folder path. Please try again.")
	}
}
