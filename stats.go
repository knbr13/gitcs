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

const outOfRange = 99999
const daysInLastSixMonths = 183
const weeksInLastSixMonths = 26

type column []int

var graphData []float64

// stats calculates and prints the stats.
func stats(email string, statsType string) {
	commits := processRepositories(email)
	fmt.Println()
	switch statsType {
	case "Table":
		printCommitsStats(commits)
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

// printCommitsStats prints the commits stats
func printCommitsStats(commits map[int]int) {
	keys := sortMapIntoSlice(commits)
	cols := buildCols(keys, commits)
	fmt.Println()
	printCells(cols)
	fmt.Println()
}

// fillCommits given a repository found in `path`, gets the commits and
// puts them in the `commits` map, returning it when completed
func fillCommits(email string, path string, commits map[int]int) (map[int]int, error) {
	// instantiate a git repo object from path
	repo, err := git.PlainOpen(path)
	if err != nil {
		return commits, fmt.Errorf("unable to open: %s", path)
	}

	// get the HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return commits, fmt.Errorf("unable to get the reference where HEAD is pointing to in: %s", path)
	}

	// get the commits history starting from HEAD
	iterator, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return commits, fmt.Errorf("failed to get the commit history in: %s", path)
	}

	// iterate the commits
	offset := calcOffset()
	err = iterator.ForEach(func(c *object.Commit) error {
		daysAgo := countDaysSinceDate(c.Author.When) + offset

		if c.Author.Email != email {
			return nil
		}

		if daysAgo != outOfRange {
			commits[daysAgo]++
		}

		return nil
	})
	if err != nil {
		return commits, fmt.Errorf("failed to process commits in: %s", path)
	}

	return commits, nil
}

// getBeginningOfDay given a time.Time calculates the start time of that day
func getBeginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
	return startOfDay
}

// countDaysSinceDate counts how many days passed since the passed `date`
func countDaysSinceDate(date time.Time) int {
	days := 0
	now := getBeginningOfDay(time.Now())
	for date.Before(now) {
		date = date.Add(time.Hour * 24)
		days++
		if days > daysInLastSixMonths {
			return outOfRange
		}
	}
	return days
}

// calcOffset determines and returns the amount of days missing to fill
// the last row of the stats graph
func calcOffset() int {
	var offset int
	weekday := time.Now().Weekday()

	switch weekday {
	case time.Sunday:
		offset = 7
	case time.Monday:
		offset = 6
	case time.Tuesday:
		offset = 5
	case time.Wednesday:
		offset = 4
	case time.Thursday:
		offset = 3
	case time.Friday:
		offset = 2
	case time.Saturday:
		offset = 1
	}

	return offset
}

// processRepositories given a user email, returns the
// commits made in the last 6 months
func processRepositories(email string) map[int]int {
	filePath := getDotFilePath()
	repos, err := parseFileLinesToSlice(filePath)
	if err != nil {
		log.Fatal(color.Red.Sprint(err))
	}
	daysInMap := daysInLastSixMonths

	commits := make(map[int]int, daysInMap)
	for i := daysInMap; i > 0; i-- {
		commits[i] = 0
	}

	for _, path := range repos {
		commits, err = fillCommits(email, path, commits)
		if err != nil {
			color.Yellow.Println(err)
		}
	}

	orders := make([]int, 0, len(commits))
	for k := range commits {
		orders = append(orders, k)
	}

	sort.Ints(orders)

	for _, order := range orders {
		if commits[order] > 0 {
			graphData = append(graphData, float64(commits[order]))
		}
	}

	// Reverse the array
	for i, j := 0, len(graphData)-1; i < j; i, j = i+1, j-1 {
		graphData[i], graphData[j] = graphData[j], graphData[i]
	}
	return commits
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
		week := int(k / 7) // 26, 25...1
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
func printCell(val int, today bool) {
	var colorFunc color.Style
	if today {
		colorFunc = color.New(color.FgBlack, color.BgRed)
		colorFunc.Printf("%2d", val)
	} else if val == 0 {
		colorFunc = color.New(color.FgWhite, color.BgBlack)
		colorFunc.Print(" - ")
	} else {
		if val < 10 {
			colorFunc = color.New(color.FgBlack, color.BgLightCyan)
		} else if val < 20 {
			colorFunc = color.New(color.FgBlack, color.BgHiCyan)
		} else if val < 30 {
			colorFunc = color.New(color.FgBlack, color.BgHiBlue)
		} else {
			colorFunc = color.New(color.FgBlack, color.BgBlue)
		}
		colorFunc.Printf("%3d", val)
	}
	colorFunc.Print(" ")
}

// printMonths prints the month names in the first line, determining when the month
// changed between switching weeks
func printMonths() {
	week := getBeginningOfDay(time.Now()).Add(-(daysInLastSixMonths * time.Hour * 24))
	month := week.Month()
	fmt.Printf("         ")
	for {
		if week.Month() != month {
			fmt.Printf("%s ", week.Month().String()[:3])
			month = week.Month()
		} else {
			fmt.Printf("    ")
		}

		week = week.Add(7 * time.Hour * 24)
		if week.After(time.Now()) {
			break
		}
	}
	fmt.Printf("\n")
}

// printDayCol given the day number (0 is Sunday) prints the day name,
// alternating the rows (prints just 2,4,6)
func printDayCol(day int) {
	out := "     "
	switch day {
	case 1:
		out = " Mon "
	case 3:
		out = " Wed "
	case 5:
		out = " Fri "
	}

	fmt.Print(out)
}

// printCells prints the cells of the graph
func printCells(cols map[int]column) {
	printMonths()
	for j := 6; j >= 0; j-- {
		for i := weeksInLastSixMonths; i >= 0; i-- {
			if i == weeksInLastSixMonths {
				printDayCol(j)
			}
			if col, ok := cols[i]; ok {
				// Special case today
				if i == 0 && j == calcOffset()-1 {
					if j < len(col) {
						printCell(col[j], true)
					} else {
						printCell(0, true)
					}
					continue
				} else {
					if j < len(col) {
						printCell(col[j], false)
					} else {
						printCell(0, false)
					}
					continue
				}
			}
			printCell(0, false)
		}
		fmt.Printf("\n")
	}
}
