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

func printTable(commits map[int]int) {
	fmt.Printf("%s     %s\n", sixEmptySpaces, buildHeader(since, until))
	s := strings.Builder{}
	days := int(until.Sub(since).Hours() / 24)
	max := getMaxValue(commits)
	for i := 0; i < 7; i++ {
		s.WriteString(fmt.Sprintf("%-5s", getDay(i)))
		for j := days + calcOffset(); j >= 0; j -= 7 {
			s.WriteString(printCell(commits[j-i], max))
		}
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
