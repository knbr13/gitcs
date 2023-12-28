package main

import (
	"bufio"
	"fmt"
	"log"
	"net/mail"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gookit/color"
)

type validator func(string) bool

func getUserInput(reader *bufio.Reader, prompt string, fn validator) string {
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		if fn(input) {
			return input
		}
		fmt.Println(color.Yellow.Sprint("Invalid input. Please try again!"))
	}
}

func getInputFromUser() (string, string) {
	reader := bufio.NewReader(os.Stdin)

	var email string
	autoEmail := getUserInput(
		reader,
		"Do you want to retrieve your global Git email address automatically? (y/n): ",
		func(s string) bool {
			if s := strings.ToLower(strings.TrimSpace(s)); s == "y" || s == "n" {
				return true
			}
			return false
		},
	)

	if strings.ToLower(strings.TrimSpace(autoEmail)) == "y" {
		email = getAutoEmailFromGit()
	} else {
		email = getUserInput(reader, "Enter your Git email address: ", func(s string) bool {
			_, err := mail.ParseAddress(strings.ToLower(strings.TrimSpace(s)))
			return err == nil
		})
	}

	folder := getUserInput(reader, "Enter the folder path to scan for Git repositories: ", func(s string) bool {
		return isValidFolderPath(strings.ToLower(strings.TrimSpace(s)))
	})
	folder = strings.ToLower(strings.TrimSpace(folder))

	return email, folder
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

	email := strings.TrimSpace(string(localEmail))
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
