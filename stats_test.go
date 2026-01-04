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
	tempDir := t.TempDir()
	now := time.Now()
	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)

	project1 := path.Join(tempDir, "project_1")
	createTestRepo(t, project1, "tester@test.com", 3, commitsDate)

	tests := []struct {
		Name     string
		Path     string
		Email    string
		Expected map[int]int
	}{
		{
			Name:     "test 1",
			Path:     project1,
			Email:    "tester@test.com",
			Expected: map[int]int{days: 3},
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
	tempDir := t.TempDir()
	now := time.Now()
	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)

	project1 := path.Join(tempDir, "project_1")
	project2 := path.Join(tempDir, "project_2")
	createTestRepo(t, project1, "tester@test.com", 3, commitsDate)
	createTestRepo(t, project2, "tester@test.com", 6, commitsDate)

	tests := []struct {
		Name     string
		Repos    []string
		Email    string
		Expected map[int]int
	}{
		{
			Name: "test 1",
			Repos: []string{
				project1,
				project2,
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
	}

	b := Boundary{
		Since: time.Now().AddDate(0, 0, -1),
		Until: time.Now().AddDate(0, 0, 1),
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			commits := processRepos(tt.Repos, tt.Email, b)
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
