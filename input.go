package main

import (
	"bufio"
	"fmt"
	"log"
	"net/mail"
	"os"
	"os/exec"
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
