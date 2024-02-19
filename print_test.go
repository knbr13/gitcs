package main

import (
	"fmt"
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
			want: "Jan             Feb             ",
		},
		{
			name: "test 2",
			args: args{
				start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
			},
			want: "Jan             Feb             Mar             ",
		},
		{
			name: "test 2",
			args: args{
				start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				end:   time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
			},
			want: "Jan             Feb             Mar             Apr             May             ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildHeader(tt.args.start, tt.args.end); got != tt.want {
				t.Errorf("buildHeader() = %v, want %v", got, tt.want)
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
			description:     "Zero value (/8)",
			val:             0,
			maxValue:        10,
			expectedMessage: color.New(color.FgWhite, color.BgBlack).Sprintf("  - "),
		},
		{
			description:     "Lower bound value (/4)",
			val:             2,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgLightCyan).Sprintf("  2 "),
		},
		{
			description:     "Middle value (/2)",
			val:             4,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgHiBlue).Sprintf("  4 "),
		},
		{
			description:     "Upper bound value 1",
			val:             8,
			maxValue:        10,
			expectedMessage: color.New(color.FgBlack, color.BgBlue).Sprintf("  8 "),
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
				t.Errorf("Expected message: %s, got: %s", tc.expectedMessage, actualMessage)
			}
		})
	}
}

func TestPrintTable(t *testing.T) {
	commits := map[int]int{
		0:  5,
		1:  8,
		2:  12,
		3:  3,
		4:  0,
		5:  10,
		6:  7,
		7:  4,
		8:  6,
		9:  9,
		10: 2,
		11: 15,
		12: 1,
		13: 0,
	}

	since = time.Date(2024, 2, 7, 0, 0, 0, 0, time.UTC)
	until = time.Date(2024, 2, 19, 0, 0, 0, 0, time.UTC)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTable(commits)
	w.Close()

	dat, err := io.ReadAll(r)
	if err != nil {
		t.Errorf("Error reading from pipe: %s", err.Error())
	}

	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = fmt.Fprint(&buf, string(dat))

	s := strings.Builder{}
	s1 := since

	s.WriteString(fmt.Sprintf("%s     %s\n", sixEmptySpaces, buildHeader(since, until)))

	max := getMaxValue(commits)
	for i := 0; i < 7; i++ {
		s.WriteString(fmt.Sprintf("%-5s", getDay(i)))
		sn2 := s1
		for !sn2.After(until) {
			d := daysAgo(sn2)
			s.WriteString(printCell(commits[d], max))
			sn2 = sn2.AddDate(0, 0, 7)
		}
		s1 = s1.AddDate(0, 0, 1)
		s.WriteRune('\n')
	}

	expectedOutput := s.String()
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(expectedOutput) {
		t.Errorf("Expected output: %q\n\n, got: %q", expectedOutput, buf.String())
	}

}
