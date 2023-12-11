package main

import (
	"fmt"
	"strings"
	"time"
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
	fmt.Printf("%s     %s\n", sixEmptySpaces, buildHeader(sixMonthsAgo, time.Now()))
	s := strings.Builder{}
	max := getMaxValue(commits)
	for i := 0; i < 7; i++ {
		s.WriteString(fmt.Sprintf("%-5s", getDay(i)))
		for j := daysAgoFromSixMonths + calcOffset() - 1; j >= 0; j -= 7 {
			s.WriteString(printCell(commits[j-i], max))
		}
		fmt.Println(s.String())
		s.Reset()
	}
}
