package main

import (
	"os"
)

func main() {
	email, folder := getInputFromUser()

	if folder != "" {
		scan(folder)
	}

	stats(email)

	// There is no need to handle the errors, it is okay to keep the file in the home dir
	homeDir, err := os.UserHomeDir()
	if err == nil {
		deleteFile(".gogitlocalstats", homeDir)
	}
}
