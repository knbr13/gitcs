package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gookit/color"
)

func TestGetDay(t *testing.T) {
	tests := []struct {
		name string
		day  int
		want string
	}{
		{
			name: "Monday",
			day:  1,
			want: "Mon",
		},
		{
			name: "Wednesday",
			day:  3,
			want: "Wed",
		},
		{
			name: "Friday",
			day:  5,
			want: "Fri",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDay(tt.day); got != tt.want {
				t.Errorf("getDay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildHeader(t *testing.T) {
	type args struct {
		start time.Time
		end   time.Time
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test 1",
			args: args{
				start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			},
			want: "     Jan                 ",
		},
		{
			name: "test 2",
			args: args{
				start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
			},
			want: "     Jan                 Feb             ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildHeader(tt.args.start, tt.args.end); got != tt.want {
				t.Errorf("buildHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintCell(t *testing.T) {
	testCases := []struct {
		description     string
		val             int
		maxValue        int
		expectedMessage string
	}{
		{
			description:     "Zero value",
			val:             0,
			maxValue:        10,
			expectedMessage: color.New(color.FgLightWhite, color.BgBlack).Sprintf("  - "),
		},
		{
			description:     "Lower bound value (/8)",
			val:             1,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgLightCyan).Sprintf("  1 "),
		},
		{
			description:     "Lower bound value (/4)",
			val:             2,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgLightCyan).Sprintf("  2 "),
		},
		{
			description:     "Middle value",
			val:             5,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgHiCyan).Sprintf("  5 "),
		},
		{
			description:     "Upper bound value 1",
			val:             7,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgHiBlue).Sprintf("  7 "),
		},
		{
			description:     "Upper bound value 2",
			val:             10,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgBlue).Sprintf(" 10 "),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualMessage := printCell(tc.val, tc.maxValue)
			if actualMessage != tc.expectedMessage {
				t.Errorf("Expected message: %q, got: %q", tc.expectedMessage, actualMessage)
			}
		})
	}
}

func TestPrintTable(t *testing.T) {
	commits := map[int]int{
		0: 5,
		1: 8,
	}

	b := Boundary{
		Since: time.Date(2024, 2, 7, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2024, 2, 19, 0, 0, 0, 0, time.UTC),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTable(commits, b)
	w.Close()

	dat, err := io.ReadAll(r)
	if err != nil {
		t.Errorf("Error reading from pipe: %s", err.Error())
	}

	os.Stdout = oldStdout

	if len(dat) == 0 {
		t.Error("Expected some output from printTable, got nothing")
	}
}

func TestBuildProgression(t *testing.T) {
	end := startOfDay(today)
	start := end.AddDate(0, 0, -13)
	commits := map[int]int{
		daysAgo(start):                  2,
		daysAgo(start.AddDate(0, 0, 6)): 1,
		daysAgo(start.AddDate(0, 0, 8)): 3,
	}
	b := Boundary{
		Since: start,
		Until: end,
	}

	points := buildProgression(commits, b, 7)
	if len(points) != 2 {
		t.Fatalf("buildProgression() returned %d points, want 2", len(points))
	}

	if points[0].Commits != 3 || points[0].Cumulative != 3 || points[0].Percent != 50 {
		t.Fatalf("first point = %+v, want commits=3 cumulative=3 percent=50", points[0])
	}
	if points[1].Commits != 3 || points[1].Cumulative != 6 || points[1].Percent != 100 {
		t.Fatalf("second point = %+v, want commits=3 cumulative=6 percent=100", points[1])
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name    string
		percent int
		width   int
		want    string
	}{
		{name: "empty", percent: 0, width: 10, want: "[----------]"},
		{name: "half", percent: 50, width: 10, want: "[#####-----]"},
		{name: "full", percent: 100, width: 10, want: "[##########]"},
		{name: "clamps high", percent: 150, width: 10, want: "[##########]"},
		{name: "zero width", percent: 50, width: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := progressBar(tt.percent, tt.width); got != tt.want {
				t.Fatalf("progressBar(%d, %d) = %q, want %q", tt.percent, tt.width, got, tt.want)
			}
		})
	}
}

func TestCountBar(t *testing.T) {
	tests := []struct {
		name  string
		count int
		max   int
		width int
		want  string
	}{
		{name: "zero count", count: 0, max: 10, width: 10, want: "          "},
		{name: "half", count: 5, max: 10, width: 10, want: "#####     "},
		{name: "rounds up", count: 1, max: 10, width: 10, want: "#         "},
		{name: "full", count: 10, max: 10, width: 10, want: "##########"},
		{name: "zero width", count: 5, max: 10, width: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countBar(tt.count, tt.max, tt.width); got != tt.want {
				t.Fatalf("countBar(%d, %d, %d) = %q, want %q", tt.count, tt.max, tt.width, got, tt.want)
			}
		})
	}
}

func TestPrintCommitBars(t *testing.T) {
	end := startOfDay(today)
	start := end.AddDate(0, 0, -6)
	commits := map[int]int{
		daysAgo(start): 2,
		daysAgo(end):   1,
	}
	b := Boundary{
		Since: start,
		Until: end,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printCommitBars(commits, b)
	w.Close()

	dat, err := io.ReadAll(r)
	if err != nil {
		t.Errorf("Error reading from pipe: %s", err.Error())
	}

	os.Stdout = oldStdout

	output := string(dat)
	if !strings.Contains(output, "Commit history") {
		t.Fatalf("printCommitBars() output = %q, want title", output)
	}
	if !strings.Contains(output, "3") {
		t.Fatalf("printCommitBars() output = %q, want commit count", output)
	}
}
