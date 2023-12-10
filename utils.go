package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gookit/color"
)

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

func daysAgo(t time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	hours := int(today.Sub(t).Hours())
	if hours < 0 {
		return 0
	}
	if hours%24 == 0 {
		return hours / 24
	}
	return hours/24 + 1
}
