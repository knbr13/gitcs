package main

import (
	"strings"
	"time"
)

// changeStatus is the small, UI-independent version of Git's status codes.
// The Git adapter converts go-git's staging/worktree status into one of these
// values before the map API and summary code see it.
type changeStatus string

const (
	changeAdded    changeStatus = "A"
	changeModified changeStatus = "M"
	changeDeleted  changeStatus = "D"
	changeRenamed  changeStatus = "R"
)

type reviewChange struct {
	Path   string
	Status changeStatus
}

type activityCommit struct {
	When  time.Time
	Email string
}

// aggregateRepoActivity converts commits into calendar offsets where zero is
// the boundary's final day, one is the previous day, and so on.
func aggregateRepoActivity(
	commits []activityCommit,
	email string,
	boundary Boundary,
) map[int]int {
	activity := make(map[int]int)
	location := boundary.Until.Location()
	since := dayStart(boundary.Since, location)
	until := dayStart(boundary.Until, location)

	for _, commit := range commits {
		if email != "*" && !strings.EqualFold(strings.TrimSpace(commit.Email), strings.TrimSpace(email)) {
			continue
		}

		commitDay := dayStart(commit.When, location)
		if commitDay.Before(since) || commitDay.After(until) {
			continue
		}

		daysBack := calendarDaysBetween(commitDay, until)
		activity[daysBack]++
	}

	return activity
}

func dayStart(value time.Time, location *time.Location) time.Time {
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func calendarDaysBetween(earlier, later time.Time) int {
	earlierUTC := time.Date(
		earlier.Year(), earlier.Month(), earlier.Day(), 0, 0, 0, 0, time.UTC,
	)
	laterUTC := time.Date(
		later.Year(), later.Month(), later.Day(), 0, 0, 0, 0, time.UTC,
	)
	return int(laterUTC.Sub(earlierUTC).Hours() / 24)
}
