package main

import (
	"flag"
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

var since, until time.Time

func init() {
	var sinceflag, untilflag string
	flag.StringVar(&sinceflag, "since", "", "start date")
	flag.StringVar(&untilflag, "until", "", "end date")
	flag.Parse()

	var err error
	if untilflag != "" {
		until, err = time.Parse("2006-01-02", untilflag)
		if err != nil {
			log.Fatal(color.Red.Sprintf("Invalid 'until' date format. Please use the format: 2006-01-02"))
		}
		if until.After(now) {
			until = now
		}
	} else {
		until = now
	}
	if sinceflag != "" {
		since, err = time.Parse("2006-01-02", sinceflag)
		if err != nil {
			log.Fatal(color.Red.Sprintf("Invalid 'since' date format. Please use the format: 2006-01-02"))
		}
	} else {
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -sixMonthsInDays)
	}
}
