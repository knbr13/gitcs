package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gookit/color"
	"github.com/guptarohit/asciigraph"
)

var sixMonthsAgo time.Time = time.Now().AddDate(0, -6, 0)
var daysAgoFromSixMonths int = daysAgo(sixMonthsAgo)

const daysInLastSixMonths = 183
const weeksInLastSixMonths = 26

type column []int

var graphData []float64

// stats calculates and prints the stats.
func stats(email string, statsType string, repos []string) {
	commits, err := processRepos(repos, email)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()
	switch statsType {
	case "Table":
		printTable(commits)
	case "Graph":
		printGraphCommits(graphData)
	}
}

// printGraphCommits prints the commits graph
func printGraphCommits(graphData []float64) {
	data := graphData

	options := []asciigraph.Option{
		asciigraph.Width(60),
		asciigraph.Height(20),
		asciigraph.Precision(0),
		asciigraph.SeriesColors(asciigraph.Blue),
	}

	graph := asciigraph.Plot(data, options...)

	fmt.Println(graph)
}

// fillCommits given a repository found in `path`, gets the commits and
// puts them in the `commits` map, returning it when completed
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

// getBeginningOfDay given a time.Time calculates the start time of that day
func getBeginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
	return startOfDay
}

// calcOffset determines and returns the amount of days missing to fill
// the last row of the stats graph
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

// processRepositories given a user email, returns the
// commits made in the last 6 months
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

// sortMapIntoSlice returns a slice of indexes of a map, ordered
func sortMapIntoSlice(m map[int]int) []int {
	// order map
	// To store the keys in slice in sorted order
	var keys []int
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	return keys
}

// buildCols generates a map with rows and columns ready to be printed to screen
func buildCols(keys []int, commits map[int]int) map[int]column {
	cols := make(map[int]column)
	col := column{}

	for _, k := range keys {
		week := k / 7      // 26, 25...1
		dayinweek := k % 7 // 0, 1, 2, 3, 4, 5, 6

		if dayinweek == 0 { // reset
			col = column{}
		}

		col = append(col, commits[k])

		if dayinweek == 6 {
			cols[week] = col
		}
	}

	return cols
}

// printCell given a cell value prints it with a different format
// based on the value amount, and on the `today` flag.
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
