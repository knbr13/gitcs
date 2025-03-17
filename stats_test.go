package main

import (
	"os"
	"path"
	"testing"
	"time"
)

func TestFillCommits(t *testing.T) {
	now := time.Now()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)
	tests := []struct {
		Name      string
		Path      string
		Email     string
		Expected  map[int]int
		ExpectErr bool
	}{
		{
			Name:     "test 1",
			Path:     path.Join(wd, "test_data", "project_1"),
			Email:    "tester@test.com",
			Expected: map[int]int{days: 3},
		},
		{
			Name:     "test 2",
			Path:     path.Join(wd, "test_data", "project_2"),
			Email:    "tester@test.com",
			Expected: map[int]int{days: 3},
		},
		{
			Name:     "test 3",
			Path:     path.Join(wd, "test_data", "project_3"),
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
			err = fillCommits(tt.Path, tt.Email, commits, b)
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

	commitsDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	days := daysAgo(commitsDate)

	tests := []struct {
		Name     string
		Repos    []string
		Email    string
		Expected map[int]int
	}{
		{
			Name: "test 1",
			Repos: []string{
				path.Join(wd, "test_data", "project_1"),
				path.Join(wd, "test_data", "project_2"),
				path.Join(wd, "test_data", "project_3"),
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
				t.Errorf("processRepos11() = %v, want %v", commits, tt.Expected)
			}
			for k, v := range tt.Expected {
				if commits[k] != v {
					t.Errorf("processRepos(22) = %v, want %v", commits[k], v)
				}
			}
		})
	}
}
