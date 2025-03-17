package main

import (
	"math"
	"testing"
	"time"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{
			name:  "valid - test 1",
			email: "tester@test.com",
			want:  true,
		},
		{
			name:  "not valid - test 2",
			email: "tester",
			want:  false,
		},
		{
			name:  "not valid - test 3",
			email: "jane@.com",
			want:  false,
		},
		{
			name:  "not valid - test 4",
			email: "jane.com",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidFolderPath(t *testing.T) {
	tests := []struct {
		name     string
		folder   string
		expected bool
	}{
		{
			name:     "valid folder#1",
			folder:   "./test_data",
			expected: true,
		},
		{
			name:     "valid folder#2",
			folder:   "./test_data/project_1",
			expected: true,
		},
		{
			name:     "valid folder#2",
			folder:   "./test_data/project_3",
			expected: true,
		},
		{
			name:     "non-existent folder",
			folder:   "/path/to/non-existent/folder",
			expected: false,
		},
		{
			name:     "file",
			folder:   "./test_data/project_1/main.go",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := isValidFolderPath(tt.folder)
			if actual != tt.expected {
				t.Errorf("isValidFolderPath() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestDaysAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		time     time.Time
		expected int
	}{
		{
			name:     "Today",
			time:     now,
			expected: 0,
		},
		{
			name:     "Yesterday",
			time:     now.Add(-24 * time.Hour),
			expected: 1,
		},
		{
			name:     "Two Days Ago",
			time:     now.Add(-48 * time.Hour),
			expected: 2,
		},
		{
			name:     "Three Days Ago",
			time:     now.Add(-72 * time.Hour),
			expected: 3,
		},
		{
			name:     "Future Date",
			time:     now.Add(24 * time.Hour),
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := daysAgo(tt.time)
			if actual != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, actual)
			}
		})
	}
}

func TestGetGlobalEmailFromGit(t *testing.T) {
	email := getGlobalEmailFromGit()
	if email == "" {
		t.Errorf("Expected email, got empty string")
	}
}

func TestGetMaxValue(t *testing.T) {
	type args struct {
		m map[int]int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "empty map",
			args: args{
				m: map[int]int{},
			},
			want: math.MinInt,
		},
		{
			name: "one element map",
			args: args{
				m: map[int]int{
					1: 1,
				},
			},
			want: 1,
		},
		{
			name: "two elements map",
			args: args{
				m: map[int]int{
					1: 1,
					2: 2,
				},
			},
			want: 2,
		},
		{
			name: "ten elements map",
			args: args{
				m: map[int]int{
					1:  1,
					2:  2,
					10: 3245,
					23: 4653,
					29: 431509,
					32: 34,
					8:  35,
					12: 12,
					19: 86,
					43: 17,
				},
			},
			want: 431509,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMaxValue(tt.args.m); got != tt.want {
				t.Errorf("getMaxValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetTimeFlags(t *testing.T) {
	originalNow := now
	defer func() {
		now = originalNow
	}()

	now = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		sinceflag     string
		untilflag     string
		expectedSince time.Time
		expectedUntil time.Time
		expectedError string
	}{
		{
			name:          "Valid since and until flags provided",
			sinceflag:     "2022-01-01",
			untilflag:     "2022-12-31",
			expectedSince: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedUntil: time.Date(2022, time.December, 31, 0, 0, 0, 0, time.UTC),
			expectedError: "",
		},
		{
			name:          "Valid since flag provided, until flag not provided",
			sinceflag:     "2022-01-01",
			untilflag:     "",
			expectedSince: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedUntil: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedError: "",
		},
		{
			name:          "Valid until flag provided, since flag not provided",
			sinceflag:     "",
			untilflag:     "2022-12-31",
			expectedSince: time.Date(2022, time.July, 2, 0, 0, 0, 0, time.UTC),
			expectedUntil: time.Date(2022, time.December, 31, 0, 0, 0, 0, time.UTC),
			expectedError: "",
		},
		{
			name:          "Invalid since flag format",
			sinceflag:     "01-01-2022",
			untilflag:     "",
			expectedSince: time.Time{},
			expectedUntil: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedError: "invalid 'since' date format. please use the format: 2006-01-02",
		},
		{
			name:          "Invalid until flag format",
			sinceflag:     "",
			untilflag:     "2022/12/31",
			expectedSince: time.Date(2022, time.July, 4, 0, 0, 0, 0, time.UTC),
			expectedUntil: time.Time{},
			expectedError: "invalid 'until' date format. please use the format: 2006-01-02",
		},
		{
			name:          "until flag is in the future",
			untilflag:     "2099-01-01",
			sinceflag:     "2022-01-01",
			expectedSince: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedUntil: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := setTimeFlags(tt.sinceflag, tt.untilflag)

			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("Unexpected error message. Expected: %v, Got: %v", tt.expectedError, err.Error())
				}
				return
			}

			if b.Since != tt.expectedSince {
				t.Errorf("Unexpected value of 'since'. Expected: %v, Got: %v", tt.expectedSince, b.Since)
			}

			if b.Until != tt.expectedUntil {
				t.Errorf("Unexpected value of 'until'. Expected: %v, Got: %v", tt.expectedUntil, b.Until)
			}
		})
	}
}
