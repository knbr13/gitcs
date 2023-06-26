package main

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
)

func main() {
	email, folder, statsType := getInputFromUser()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("red", "bold")
	s.FinalMSG = "Done!"
	s.Start()

	if folder != "" {
		scan(folder)
	}

	stats(email, statsType)

	s.Stop()
	// There is no need to handle the errors, it is okay to keep the file in the home dir
	homeDir, err := os.UserHomeDir()
	if err == nil {
		deleteFile(".gogitlocalstats", homeDir)
	}
}
