package main

import (
	"bufio"
	"fmt"
	"log"
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

func getPathFromUser(reader *bufio.Reader) string {
	folder := getUserInput(reader, "Enter the folder path to scan for Git repositories: ", func(s string) bool {
		return isValidFolderPath(strings.ToLower(strings.TrimSpace(s)))
	})
	folder = strings.ToLower(strings.TrimSpace(folder))

	return folder
}
