package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func deleteFile(fileName, filePath string) error {
	err := os.Remove(filepath.Join(filePath, fileName))
	if err != nil {
		return fmt.Errorf("failed to delete %s file: %s", fileName, err)
	}

	return nil
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
