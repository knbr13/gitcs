package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
)

func getInputFromUser() (string, string, string) {
	reader := bufio.NewReader(os.Stdin)

	email := ""

	autoEmail := askForEmail()

	if autoEmail == "y" {
		email = getAutoEmailFromGit()
	} else {
		email = getEmailFromUser(reader)
	}

	folder := getFolderFromUser(reader)
	statsType := getStatsType(reader)

	return email, folder, statsType
}

func getStatsType(reader *bufio.Reader) string {

	prompt := promptui.Select{
		Label: "Select Stats type",
		Items: []string{"Table", "Graph"},
	}

	_, result, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return ""
	}

	return result
}

func askForEmail() string {
	prompt := promptui.Prompt{
		Label:     "Do you want to get your local git email?",
		IsConfirm: true,
	}

	result, err := prompt.Run()

	if err != nil {
		return ""
	}

	return result

}

func getAutoEmailFromGit() string {
	localEmail, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err != nil {
		log.Fatal(err)
		return ""
	}

	email := strings.TrimSpace(bytes.NewBuffer(localEmail).String())
	fmt.Println("Your git email is:", email)
	return email
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
