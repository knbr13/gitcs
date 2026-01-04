package main

import (
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func createTestRepo(t *testing.T, dir, email string, commitCount int, when time.Time) {
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	for i := 0; i < commitCount; i++ {
		filename := path.Join(dir, fmt.Sprintf("file_%d", i))
		err = os.WriteFile(filename, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = w.Add(fmt.Sprintf("file_%d", i))
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = w.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "tester",
				Email: email,
				When:  when,
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}
}

func TestFillCommits(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	tempDir := t.TempDir()
	now := time.Now()
	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)

	project1 := path.Join(tempDir, "project_1")
	createTestRepo(t, project1, "tester@test.com", 3, commitsDate)

	tests := []struct {
		Name      string
		Path      string
		Email     string
		Expected  map[int]int
		ExpectErr bool
	}{
		{
			Name:     "test 1",
			Path:     project1,
			Email:    "tester@test.com",
			Expected: map[int]int{days: 3},
		},
		{
			Name:     "test 4",
			Path:     path.Join(wd, "test_data", "project_that_has_future_commits"),
			Email:    "tester@test.com",
			Expected: map[int]int{},
		},
		{
			Name:     "test 5",
			Path:     path.Join(wd, "test_data", "project_by_another_contributor"),
			Email:    "tester@test.com",
			Expected: map[int]int{},
		},
		{
			Name:      "test 6",
			ExpectErr: true,
			Path:      path.Join(wd, "test_data", "project_4"),
		},
	}

	b := Boundary{
		Since: time.Now().AddDate(0, 0, -1),
		Until: time.Now().AddDate(0, 0, 1),
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			commits := map[int]int{}
			var mu sync.Mutex
			err := fillCommits(tt.Path, tt.Email, commits, b, &mu)
			if tt.ExpectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("failed to fill commits in %q: %v", tt.Path, err)
			}
			if len(commits) != len(tt.Expected) {
				t.Errorf("fillCommits() = %v, want %v", commits, tt.Expected)
			}
			for k, v := range tt.Expected {
				if commits[k] != v {
					t.Errorf("fillCommits() = %v, want %v", commits[k], v)
				}
			}
		})
	}
}

func TestProcessRepos(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	tempDir := t.TempDir()
	now := time.Now()
	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)

	project1 := path.Join(tempDir, "project_1")
	project2 := path.Join(tempDir, "project_2")
	createTestRepo(t, project1, "tester@test.com", 3, commitsDate)
	createTestRepo(t, project2, "tester@test.com", 6, commitsDate)

	tests := []struct {
		Name      string
		Repos     []string
		Email     string
		Expected  map[int]int
		ExpectErr bool
	}{
		{
			Name: "test 1",
			Repos: []string{
				path.Join(wd, "test_data", "project_1"),
				path.Join(wd, "test_data", "project_2"),
				path.Join(wd, "test_data", "project_3"),
				path.Join(wd, "test_data", "project_that_has_future_commits"),
				path.Join(wd, "test_data", "project_by_another_contributor"),
			},
			Email:    "tester@test.com",
			Expected: map[int]int{days: 9},
		},
		{
			Name:     "test 2",
			Repos:    []string{},
			Email:    "tester@test.com",
			Expected: map[int]int{},
		},
		{
			Name:      "test 3",
			Repos:     []string{path.Join(wd, "test_data", "repo_that_does_not_exist")},
			Email:     "tester@test.com",
			Expected:  map[int]int{},
			ExpectErr: true,
		},
	}

	b := Boundary{
		Since: time.Now().AddDate(0, 0, -1),
		Until: time.Now().AddDate(0, 0, 1),
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			commits := processRepos(tt.Repos, tt.Email, b)
			if tt.ExpectErr {
				if len(commits) != 0 {
					t.Fatalf("expected error, got commits results: %v", commits)
				}
				return
			}
			if len(commits) != len(tt.Expected) {
				t.Errorf("processRepos() = %v, want %v", commits, tt.Expected)
			}
			for k, v := range tt.Expected {
				if commits[k] != v {
					t.Errorf("processRepos() = %v, want %v", commits[k], v)
				}
			}
		})
	}
}
