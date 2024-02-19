package main

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/gookit/color"
)

func getPathFromUser(reader *bufio.Reader) (string, error) {
	for {
		fmt.Print("enter the folder path to scan for Git repositories: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		if isValidFolderPath(input) {
			return input, nil
		}
		fmt.Println(color.Yellow.Sprintf("gitcs: path %q is not found, please enter a valid folder path", input))
	}
}
