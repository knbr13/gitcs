package main

import (
	"os"
	"github.com/briandowns/spinner"
	"time"
)

func main() {
	email, folder := getInputFromUser()

	if folder != "" {
		scan(folder)
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond) 
	s.Color("red", "bold")
	s.FinalMSG = "Done!"
	s.Start()
	
	stats(email)

	s.Stop()
	// There is no need to handle the errors, it is okay to keep the file in the home dir
	homeDir, err := os.UserHomeDir()
	if err == nil {
		deleteFile(".gogitlocalstats", homeDir)
	}
}
