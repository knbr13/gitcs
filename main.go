package main

import (
	"time"

	"github.com/briandowns/spinner"
)

func main() {
	email, folder := getInputFromUser()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("red", "bold")
	s.FinalMSG = "Done!"
	s.Start()

	repos := scan(folder)

	stats(email, repos)

	s.Stop()
}
