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

	"github.com/gookit/color"
	"github.com/manifoldco/promptui"
)

func getInputFromUser() (string, string, string) {
	reader := bufio.NewReader(os.Stdin)

	email := ""
	autoEmail := askForEmail(reader)

	if autoEmail {
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
		color.Red.Printf("Prompt failed %v\n", err)
		return ""
	}

	return result
}

func askForEmail(reader *bufio.Reader) bool {
	for {
		fmt.Print("Do you want to retrieve your global Git email address automatically? (y/n): ")
		result, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		result = strings.TrimSpace(result)
		if strings.ToLower(result) == "y" {
			return true
		}
		if strings.ToLower(result) == "n" {
			return false
		}
		fmt.Println(color.Yellow.Sprint("Invalid input. Please try again."))
	}
}

func getAutoEmailFromGit() string {
	localEmail, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err != nil {
		log.Fatal(err)
		return ""
	}

	email := strings.TrimSpace(bytes.NewBuffer(localEmail).String())
	fmt.Println("Your git email is:", color.Cyan.Sprint(email))
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

		fmt.Println(color.Yellow.Sprint("Invalid email address. Please try again."))
	}
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return emailRegex.MatchString(email)
}
