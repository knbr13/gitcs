package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gookit/color"
)

var sixEmptySpaces = strings.Repeat(" ", 6)

func buildHeader(start, end time.Time) string {
	s := strings.Builder{}
	for current := start; current.Before(end) || current.Equal(end); current = current.AddDate(0, 1, 0) {
		s.WriteString(fmt.Sprintf("%-16s", current.Month().String()[:3]))
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
	return strings.Repeat(" ", 3)
}

func printTable(commits map[int]int, since, until time.Time) {
	for since.Weekday() != time.Sunday {
		since = since.AddDate(0, 0, -1)
	}
	for until.Weekday() != time.Saturday {
		until = until.AddDate(0, 0, 1)
	}

	fmt.Printf("%s     %s\n", sixEmptySpaces, buildHeader(since, until))
	max := getMaxValue(commits)

	s := strings.Builder{}
	s1 := since

	for i := 0; i < 7; i++ {
		s.WriteString(fmt.Sprintf("%-5s", getDay(i)))
		sn2 := s1
		for !sn2.After(until) {
			d := daysAgo(sn2)
			s.WriteString(printCell(commits[d], max))
			sn2 = sn2.AddDate(0, 0, 7)
		}
		s1 = s1.AddDate(0, 0, 1)
		fmt.Println(s.String())
		s.Reset()
	}
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
