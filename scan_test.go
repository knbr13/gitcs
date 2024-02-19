package main

import (
	"os"
	"path"
	"testing"
)

func TestScanGitFolders(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	test := []struct {
		Name string
		Root string
		Want []string
	}{
		{
			Name: "3 expected repos",
			Root: path.Join(wd, "test_data"),
			Want: []string{path.Join(wd, "test_data", "project_1"), path.Join(wd, "test_data", "project_2"), path.Join(wd, "test_data", "project_3")},
		},
		{
			Name: "no expected repos",
			Root: path.Join(wd, ".github"),
			Want: []string{},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := scanGitFolders(tt.Root)
			if err != nil {
				t.Fatalf("failed to scan git folders: %v", err)
			}

			if len(got) != len(tt.Want) {
				t.Fatalf("expected %d git folders, got %d", len(tt.Want), len(got))
			}

			for i := range got {
				if got[i] != tt.Want[i] {
					t.Fatalf("expected %s, got %s", tt.Want[i], got[i])
				}
			}
		})
	}
}
