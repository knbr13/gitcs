package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gookit/color"
)

var sixMonthsAgo time.Time = time.Now().AddDate(0, -6, 0)
var daysAgoFromSixMonths int = daysAgo(sixMonthsAgo)

func stats(email string, repos []string) {
	commits, err := processRepos(repos, email)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()
	printTable(commits)
}

func fillCommits(path, email string, commits map[int]int) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	commitIterator, err := repo.Log(&git.LogOptions{Since: &sixMonthsAgo})
	if err != nil {
		return err
	}

	err = commitIterator.ForEach(func(c *object.Commit) error {
		if c.Author.Email != email {
			return nil
		}

		days := daysAgo(c.Author.When)
		commits[days]++
		return nil
	})
	if err != nil {
		return err
	}
	for i := time.Now(); i.After(sixMonthsAgo); i = i.AddDate(0, 0, -1) {
		days := daysAgo(i)
		if _, ok := commits[days]; !ok {
			commits[days] = 0
		}
	}
	for i := daysAgoFromSixMonths; i < daysAgoFromSixMonths+calcOffset()-1; i++ {
		commits[i] = 0
	}
	return nil
}

func calcOffset() int {
	now := time.Now()
	switch now.Weekday() {
	case time.Sunday:
		return 0
	case time.Monday:
		return 1
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	case time.Saturday:
		return 6
	}
	panic("unhandled time")
}

func processRepos(repos []string, email string) (map[int]int, error) {
	m := map[int]int{}
	var err error
	for _, repo := range repos {
		err = fillCommits(repo, email, m)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func printCell(val, maxValue int) string {
	var colorFunc color.Style
	if val == 0 {
		colorFunc = color.New(color.FgWhite, color.BgBlack)
		return colorFunc.Sprintf("  - ")
	}
	if val <= maxValue/8 {
		colorFunc = color.New(color.FgBlack, color.BgLightCyan)
	} else if val <= maxValue/4 {
		colorFunc = color.New(color.FgBlack, color.BgHiCyan)
	} else if val < maxValue/2 {
		colorFunc = color.New(color.FgBlack, color.BgHiBlue)
	} else {
		colorFunc = color.New(color.FgBlack, color.BgBlue)
	}
	return colorFunc.Sprintf(" %2d ", val)
}
