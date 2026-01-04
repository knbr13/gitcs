package main

import (
	"os"
	"path"
	"slices"
	"testing"
)

func TestScanGitFolders(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	test := []struct {
		Name      string
		Root      string
		Want      []string
		ExpectErr bool
	}{
		{
			Name: "5 expected repos",
			Root: path.Join(wd, "test_data"),
			Want: []string{
				path.Join(wd, "test_data", "project_1"),
				path.Join(wd, "test_data", "project_2"),
				path.Join(wd, "test_data", "project_3"),
				path.Join(wd, "test_data", "project_that_has_future_commits"),
				path.Join(wd, "test_data", "project_by_another_contributor"),
			},
		},
		{
			Name: "no expected repos in empty dir",
			Root: t.TempDir(),
			Want: []string{},
		},
		{
			Name:      "path does not exist",
			Root:      path.Join(wd, "does_not_exist"),
			Want:      []string{},
			ExpectErr: true,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := scanGitFolders(tt.Root)
			if tt.ExpectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("failed to scan git folders: %v", err)
			}

			if len(got) != len(tt.Want) {
				t.Fatalf("expected %d git folders, got %d: %v", len(tt.Want), len(got), got)
			}

			for i := range got {
				if !slices.Contains(tt.Want, got[i]) {
					t.Fatalf("expected %q to be in %q", got[i], tt.Want)
				}
			}
		})
	}
}
