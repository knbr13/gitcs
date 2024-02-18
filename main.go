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

	s := spinner.New(spinner.CharSets[6], 100*time.Millisecond, spinner.WithSuffix(" loading..."))

	s.Color("green")
	s.Start()
	defer s.Stop()

	repos, err := scanGitFolders(folder)
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("\ngitcs: error: %s\n", err.Error()))
		s.Stop()
		os.Exit(1)
	}

	commits := processRepos(repos, email)
	fmt.Print("\n\n")
	printTable(commits)
	fmt.Print("\n\n")
}
