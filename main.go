package main

import (
	"bufio"
	"flag"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	folder := getPathFromUser(reader)

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
var email string

func init() {
	var sinceflag, untilflag string
	flag.StringVar(&sinceflag, "since", "", "start date")
	flag.StringVar(&untilflag, "until", "", "end date")
	flag.StringVar(&email, "email", strings.TrimSpace(getGlobalEmailFromGit()), "you Git email")
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
		since = time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, until.Location()).AddDate(0, 0, -sixMonthsInDays)
	}

	_, err = mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		log.Fatal(color.Red.Sprintf("Invalid 'email' address"))
	}
}
