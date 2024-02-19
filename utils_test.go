package main

import (
	"testing"
)

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
