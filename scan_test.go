package main

import (
	"os"
	"path"
	"testing"
)

func TestScanGitFolders(t *testing.T) {
	tempDir := t.TempDir()

	project1 := path.Join(tempDir, "project_1")
	project2 := path.Join(tempDir, "project_2")
	project3 := path.Join(tempDir, "project_3")
	ignoredDir := path.Join(tempDir, "node_modules")

	dirs := []string{
		path.Join(project1, ".git"),
		path.Join(project2, ".git"),
		path.Join(project3, ".git"),
		path.Join(ignoredDir, ".git"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create temp dir %s: %v", d, err)
		}
	}

	test := []struct {
		Name string
		Root string
		Want []string
	}{
		{
			Name: "3 expected repos",
			Root: tempDir,
			Want: []string{project1, project2, project3},
		},
		{
			Name: "no expected repos in empty dir",
			Root: t.TempDir(),
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
				t.Fatalf("expected %d git folders, got %d: %v", len(tt.Want), len(got), got)
			}

			for _, w := range tt.Want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %s in %v", w, got)
				}
			}
		})
	}
}
