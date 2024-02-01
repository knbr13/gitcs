package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	folder := getPathFromUser(reader)

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithSuffix("  loading..."))

	s.Color("red", "bold")
	s.Start()
	defer s.Stop()

	repos, err := scanGitFolders(folder)
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("Error: %v\n", err))
		os.Exit(1)
	}
	stats(email, repos)
}
