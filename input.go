package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gookit/color"
)

func getPathFromUser(reader *bufio.Reader) string {
	for {
		fmt.Print("enter the folder path to scan for Git repositories: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "gitcs: error reading input: %s\n", err.Error())
			os.Exit(1)
		}
		input = strings.TrimSpace(input)
		if isValidFolderPath(input) {
			return input
		}
		fmt.Println(color.Yellow.Sprintf("gitcs: path %q is not found, please enter a valid folder path", input))
	}
}
