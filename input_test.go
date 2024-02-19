package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestGetPathFromUser(t *testing.T) {
	testCases := []struct {
		description   string
		input         string
		expectedPath  string
		expectedError bool
	}{
		{
			description:   "Valid folder path",
			input:         "./test_data",
			expectedPath:  "./test_data",
			expectedError: false,
		},
		{
			description:   "Invalid folder path",
			input:         "./path/to/invalid/folder",
			expectedPath:  "./",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.input + "\n"))
			actualPath, err := getPathFromUser(reader)
			if err != nil && !tc.expectedError {
				t.Errorf("getPathFromUser() error = %v, want %v", err, tc.expectedError)
			}

			if !tc.expectedError && actualPath != tc.expectedPath {
				t.Errorf("Expected path: %s, got: %s", tc.expectedPath, actualPath)
			}
		})
	}
}
