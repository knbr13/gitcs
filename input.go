package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

func getEmailFromUser(reader *bufio.Reader) string {
	for {
		fmt.Print("Enter your email address: ")
		email, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		email = strings.TrimSpace(email)

		if isValidEmail(email) {
			return email
		}

		fmt.Println("Invalid email address. Please try again.")
	}
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return emailRegex.MatchString(email)
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
