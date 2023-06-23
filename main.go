package main

import (
	"bufio"
	"os"
	"fmt"
	"strings"
)

func main() {
	email, folder := getInputFromUser()

	if folder != "" {
		scan(folder)
	}

	stats(email)
}

func getInputFromUser() (string, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter your email address: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Enter the folder path to scan for Git repositories: ")
	folder, _ := reader.ReadString('\n')
	folder = strings.TrimSpace(folder)

	return email, folder
}
