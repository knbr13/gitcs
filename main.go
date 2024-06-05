package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("gitcs: error: %s\n", err.Error()))
		os.Exit(1)
	}

	var email, path string
	var sinceflag, untilflag string
	flag.StringVar(&sinceflag, "since", "", "start date")
	flag.StringVar(&untilflag, "until", "", "end date")
	flag.StringVar(&email, "email", strings.TrimSpace(getGlobalEmailFromGit()), "you Git email")
	flag.StringVar(&path, "path", wd, "folder path to scan")
	flag.Parse()

	b, err := setTimeFlags(sinceflag, untilflag)
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("gitcs: %s\n", err.Error()))
		os.Exit(1)
	}

	if valid := isValidEmail(email); !valid {
		fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: invalid 'email' address"))
		os.Exit(1)
	}

	s := spinner.New(spinner.CharSets[6], 100*time.Millisecond, spinner.WithSuffix(" loading..."))

	s.Color("green")
	s.Start()
	defer s.Stop()

	repos, err := scanGitFolders(path)
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("\ngitcs: error: %s\n", err.Error()))
		s.Stop()
		os.Exit(1)
	}

	commits := processRepos(repos, email, *b)
	fmt.Print("\n\n")
	printTable(commits, *b)
	fmt.Print("\n\n")
}

type Boundary struct {
	Since time.Time
	Until time.Time
}
