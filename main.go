package main

import (
	"log"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

func main() {
	email, folder := getInputFromUser()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithSuffix("  loading..."))
	s.Color("red", "bold")
	func() {
		s.Start()
		defer s.Stop()

		repos, err := scanGitFolders(folder)
		if err != nil {
			log.Fatal(color.Red.Sprintf("Error: %v\n", err))
		}
		stats(email, repos)
	}()
}
