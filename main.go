package main

import (
	"time"

	"github.com/briandowns/spinner"
)

func main() {
	email, folder := getInputFromUser()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithSuffix("  loading..."))
	s.Color("red", "bold")
	s.FinalMSG = "Done!"
	func() {
		s.Start()
		defer s.Stop()

		repos := scan(folder)
		stats(email, repos)
	}()
}
