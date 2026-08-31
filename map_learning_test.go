package main

import (
	"testing"
	"time"
)

func TestAggregateRepoActivityFiltersEmailAndBoundary(t *testing.T) {
	boundary := Boundary{
		Since: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, time.January, 7, 23, 59, 0, 0, time.UTC),
	}
	commits := []activityCommit{
		{When: time.Date(2026, time.January, 7, 12, 0, 0, 0, time.UTC), Email: "dev@example.com"},
		{When: time.Date(2026, time.January, 6, 12, 0, 0, 0, time.UTC), Email: "DEV@example.com"},
		{When: time.Date(2026, time.January, 6, 12, 0, 0, 0, time.UTC), Email: "other@example.com"},
		{When: time.Date(2025, time.December, 31, 12, 0, 0, 0, time.UTC), Email: "dev@example.com"},
	}

	got := aggregateRepoActivity(commits, "dev@example.com", boundary)
	want := map[int]int{0: 1, 1: 1}
	if len(got) != len(want) {
		t.Fatalf("aggregateRepoActivity() = %#v, want %#v", got, want)
	}
	for day, count := range want {
		if got[day] != count {
			t.Fatalf("aggregateRepoActivity()[%d] = %d, want %d; full result %#v", day, got[day], count, got)
		}
	}
}
