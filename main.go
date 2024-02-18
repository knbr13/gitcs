package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

var email string
var since, until time.Time

func main() {
	var sinceflag, untilflag string
	flag.StringVar(&sinceflag, "since", "", "start date")
	flag.StringVar(&untilflag, "until", "", "end date")
	flag.StringVar(&email, "email", strings.TrimSpace(getGlobalEmailFromGit()), "you Git email")
	flag.Parse()

	var err error
	if untilflag != "" {
		until, err = time.Parse("2006-01-02", untilflag)
		if err != nil {
			fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: invalid 'until' date format. please use the format: 2006-01-02"))
			os.Exit(1)
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
			fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: invalid 'since' date format. please use the format: 2006-01-02"))
			os.Exit(1)
		}
	} else {
		since = time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, until.Location()).AddDate(0, 0, -sixMonthsInDays)
	}

	_, err = mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: invalid 'email' address"))
		os.Exit(1)
	}

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
