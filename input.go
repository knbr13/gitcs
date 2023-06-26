package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func getInputFromUser() (string, string, uint8) {
	reader := bufio.NewReader(os.Stdin)

	email := getEmailFromUser(reader)
	folder := getFolderFromUser(reader)
	statsType := getStatsType(reader)

	return email, folder, statsType
}

func getStatsType(reader *bufio.Reader) uint8 {
	var number uint8
	for {
		fmt.Print("Enter your stats type (1: Table --- 2: Graph): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		input = strings.TrimSpace(input)

		if num, err := strconv.ParseUint(input, 10, 8); err == nil {
			number = uint8(num)
			if number == 1 || number == 2 {
				return number
			}
		}

		fmt.Println("Invalid stats type. Please try again.")
	}
}

func getEmailFromUser(reader *bufio.Reader) string {
	for {
		fmt.Print("Enter your Git email address: ")
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

		fmt.Println("Invalid folder path. Please try again.")
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
