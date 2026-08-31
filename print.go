package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gookit/color"
)

func buildHeader(start, end time.Time) string {
	s := strings.Builder{}
	s.WriteString("     ") // Offset for day names (Mon, Wed, Fri) which are 5 chars total with padding
	lastMonth := ""
	for current := start; !current.After(end); current = current.AddDate(0, 0, 7) {
		month := current.Month().String()[:3]
		if month != lastMonth {
			s.WriteString(fmt.Sprintf("%-4s", month))
			lastMonth = month
		} else {
			s.WriteString("    ")
		}
	}
	return s.String()
}

func getDay(i int) string {
	switch i {
	case 1:
		return "Mon"
	case 3:
		return "Wed"
	case 5:
		return "Fri"
	}
	return "   "
}

func printTable(commits map[int]int, b Boundary) {
	since := b.Since
	until := b.Until

	for since.Weekday() != time.Sunday {
		since = since.AddDate(0, 0, -1)
	}
	for until.Weekday() != time.Saturday {
		until = until.AddDate(0, 0, 1)
	}

	fmt.Println(buildHeader(since, until))
	max := getMaxValue(commits)

	for i := 0; i < 7; i++ {
		fmt.Printf("%-5s", getDay(i))
		curr := since.AddDate(0, 0, i)
		for !curr.After(until) {
			d := daysAgo(curr)
			fmt.Print(printCell(commits[d], max))
			curr = curr.AddDate(0, 0, 7)
		}
		fmt.Println()
	}

	printLegend(max)
	printSummary(commits, b)
}

func printCell(val, maxValue int) string {
	digitWidth := 1
	if maxValue > 0 {
		digitWidth = len(fmt.Sprintf("%d", maxValue))
	}

	var colorFunc color.Style
	if val == 0 {
		colorFunc = color.New(color.FgLightWhite, color.BgBlack)
		return colorFunc.Sprintf(" %*s ", digitWidth, "-")
	}

	if maxValue <= 0 {
		maxValue = 1
	}

	if val <= maxValue/4 {
		colorFunc = color.New(color.FgBlack, color.BgLightCyan)
	} else if val <= maxValue/2 {
		colorFunc = color.New(color.FgBlack, color.BgHiCyan)
	} else if val <= (maxValue*3)/4 {
		colorFunc = color.New(color.FgBlack, color.BgHiBlue)
	} else {
		colorFunc = color.New(color.FgBlack, color.BgBlue)
	}
	return colorFunc.Sprintf(" %*d ", digitWidth, val)
}

func printLegend(max int) {
	fmt.Printf("\nLegend: Less ")
	fmt.Print(printCell(0, max), " ")
	if max > 0 {
		fmt.Print(printCell(1, max), " ")
	}
	if max > 1 {
		fmt.Print(printCell(max/2, max), " ")
	}
	if max > 2 {
		fmt.Print(printCell(max*3/4, max), " ")
	}
	if max > 3 {
		fmt.Print(printCell(max, max), " ")
	}
	fmt.Println("More")
}

func printSummary(commits map[int]int, b Boundary) {
	total := 0
	for _, v := range commits {
		total += v
	}
	fmt.Printf("Total commits: %s between %s and %s\n",
		color.Green.Sprintf("%d", total),
		color.Cyan.Render(b.Since.Format("2006-01-02")),
		color.Cyan.Render(b.Until.Format("2006-01-02")),
	)
}

type progressPoint struct {
	Start      time.Time
	End        time.Time
	Commits    int
	Cumulative int
	Percent    int
}

func printProgression(commits map[int]int, b Boundary) {
	points := buildProgression(commits, b, 7)
	if len(points) == 0 {
		fmt.Println("Overall progression")
		fmt.Println("No commits found for this date range.")
		return
	}

	fmt.Println("Overall progression")
	fmt.Printf("%-23s %7s %7s %s\n", "Period", "Commits", "Total", "Progress")
	for _, point := range points {
		period := fmt.Sprintf("%s..%s", point.Start.Format("2006-01-02"), point.End.Format("2006-01-02"))
		fmt.Printf(
			"%-23s %7d %7d %s %3d%%\n",
			period,
			point.Commits,
			point.Cumulative,
			progressBar(point.Percent, 20),
			point.Percent,
		)
	}
}

func printCommitBars(commits map[int]int, b Boundary) {
	points := buildProgression(commits, b, 7)
	if len(points) == 0 {
		fmt.Println("Commit history")
		fmt.Println("No commits found for this date range.")
		return
	}

	maxCommits := 0
	for _, point := range points {
		if point.Commits > maxCommits {
			maxCommits = point.Commits
		}
	}

	fmt.Println("Commit history")
	for _, point := range points {
		period := fmt.Sprintf("%s..%s", point.Start.Format("2006-01-02"), point.End.Format("2006-01-02"))
		fmt.Printf("%-23s |%s| %d\n", period, countBar(point.Commits, maxCommits, 30), point.Commits)
	}
}

func buildProgression(commits map[int]int, b Boundary, intervalDays int) []progressPoint {
	if intervalDays <= 0 {
		intervalDays = 7
	}

	start := startOfDay(b.Since)
	end := startOfDay(b.Until)
	if start.After(end) {
		return nil
	}

	total := commitsBetween(commits, start, end)
	if total == 0 {
		return nil
	}

	var points []progressPoint
	cumulative := 0
	for current := start; !current.After(end); current = current.AddDate(0, 0, intervalDays) {
		periodEnd := current.AddDate(0, 0, intervalDays-1)
		if periodEnd.After(end) {
			periodEnd = end
		}

		periodCommits := commitsBetween(commits, current, periodEnd)
		cumulative += periodCommits
		points = append(points, progressPoint{
			Start:      current,
			End:        periodEnd,
			Commits:    periodCommits,
			Cumulative: cumulative,
			Percent:    (cumulative * 100) / total,
		})
	}

	return points
}

func commitsBetween(commits map[int]int, start, end time.Time) int {
	total := 0
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		total += commits[daysAgo(current)]
	}
	return total
}

func progressBar(percent, width int) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := (percent*width + 50) / 100
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func countBar(count, max, width int) string {
	if width <= 0 {
		return ""
	}
	if count <= 0 || max <= 0 {
		return strings.Repeat(" ", width)
	}

	filled := (count*width + max - 1) / max
	if filled > width {
		filled = width
	}
	return strings.Repeat("#", filled) + strings.Repeat(" ", width-filled)
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
