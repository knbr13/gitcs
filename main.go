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

	if len(os.Args) > 1 && os.Args[1] == "map" {
		if err := runMapCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs map: %s", err))
			os.Exit(1)
		}
		return
	}

	var email, path string
	var sinceflag, untilflag string
	var showBars bool
	var showProgress bool
	flag.StringVar(&sinceflag, "since", "", "start date")
	flag.StringVar(&untilflag, "until", "", "end date")
	flag.StringVar(&email, "email", strings.TrimSpace(getGlobalEmailFromGit()), "your Git email")
	flag.StringVar(&path, "path", "", "folder path to scan")
	flag.BoolVar(&showBars, "bars", false, "show weekly commit history bar chart")
	flag.BoolVar(&showProgress, "progress", false, "show cumulative commit progression")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\nRequired flags:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -path string\n\tfolder path to scan\n")

		fmt.Fprintf(flag.CommandLine.Output(), "\nOptional flags:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -bars\n\tshow weekly commit history bar chart\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -email string\n\tyour Git email (default %q)\n", strings.TrimSpace(getGlobalEmailFromGit()))
		fmt.Fprintf(flag.CommandLine.Output(), "  -progress\n\tshow cumulative commit progression\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -since string\n\tstart date (default 6 months ago)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -until string\n\tend date (default today)\n")
	}
	flag.Parse()

	if path == "" {
		fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: please provide a folder path to scan"))
		flag.Usage()
		os.Exit(1)
	}

	if !isValidFolderPath(path) {
		fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: path %q is not a valid directory", path))
		os.Exit(1)
	}

	if email == "" {
		fmt.Fprintln(os.Stderr, color.Red.Sprintf("gitcs: could not find global Git email, please provide it using the -email flag"))
		os.Exit(1)
	}

	b, err := setTimeFlags(sinceflag, untilflag)
	if err != nil {
		fmt.Fprint(os.Stderr, color.Red.Sprintf("gitcs: %s\n", err.Error()))
		os.Exit(1)
	}

	if valid := email == "*" || isValidEmail(email); !valid {
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
	if showBars {
		fmt.Print("\n")
		printCommitBars(commits, *b)
	}
	if showProgress {
		fmt.Print("\n")
		printProgression(commits, *b)
	}
	fmt.Print("\n\n")
}

func runMapCommand(arguments []string) error {
	if len(arguments) == 0 {
		return runWebMap()
	}
	return fmt.Errorf("unknown option %q", strings.Join(arguments, " "))
}

type Boundary struct {
	Since time.Time
	Until time.Time
}
