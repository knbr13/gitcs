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
