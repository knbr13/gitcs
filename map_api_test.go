package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestDiffFileLinesReportsCountsAndFirstNewLine(t *testing.T) {
	oldContent := "package main\n\nfunc keep() {}\nfunc change() {\n\tprintln(1)\n}\n"
	newContent := "package main\n\nfunc keep() {}\nfunc change() {\n\tprintln(2)\n\tprintln(3)\n}\n"

	got := diffFileLines(oldContent, newContent)
	if got.Additions != 2 || got.Deletions != 1 || got.FirstChangedLine != 5 {
		t.Fatalf("diff facts = %#v, want +2 -1 starting on line 5", got)
	}
}

func TestTouchedGoSymbolsUsesOldAndNewDeclarations(t *testing.T) {
	oldContent := "package main\n\nfunc removed() {\n\tprintln(1)\n}\n"
	newContent := "package main\n\nfunc added() {\n\tprintln(2)\n}\n"
	diff := diffFileLines(oldContent, newContent)

	got := touchedGoSymbols(oldContent, diff.OldRanges, newContent, diff.NewRanges)
	want := []string{"added", "removed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("touched symbols = %#v, want %#v", got, want)
	}
}

func TestValidateOpenTargetRejectsOutsideAndDeletedFiles(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	if err := validateOpenTarget(root, openTarget{Path: outside, Openable: true}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside target error = %v", err)
	}
	if err := validateOpenTarget(root, openTarget{Path: filepath.Join(root, "old.go"), Openable: false}); err == nil || !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("deleted target error = %v", err)
	}
}

func TestIsMeaningfulOtherChangeFiltersNoise(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: ".gitignore", want: false},
		{path: "frontend/dist/app.js", want: false},
		{path: "README.md", want: true},
		{path: "go.mod", want: true},
		{path: "scripts/release.sh", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isMeaningfulOtherChange(tt.path); got != tt.want {
				t.Fatalf("isMeaningfulOtherChange(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSummarizeChangeModifiedGoFile(t *testing.T) {
	summary := summarizeChange(changeSummaryFacts{
		Label:             "service.go",
		Language:          "Go",
		Status:            changeModified,
		Additions:         3,
		Deletions:         1,
		FirstChangedLine:  12,
		TouchedSymbols:    []string{"Run"},
		PreviousSymbols:   []string{"Run", "Stop"},
		CurrentSymbols:    []string{"Run", "Start"},
		PreviousLineCount: 30,
		CurrentLineCount:  32,
		IncomingCount:     2,
		OutgoingCount:     1,
		Neighbors:         []string{"main.go"},
	})

	assertContainsAll(t, strings.ToLower(summary.Previous), []string{"previously", "go", "run", "stop"})
	assertContainsAll(t, strings.ToLower(summary.Current), []string{"now", "go", "run", "start"})
	assertContainsAll(t, strings.ToLower(summary.Changed), []string{"modified", "line 12", "+3/-1", "run", "adding start", "removing stop"})
	assertContainsAll(t, strings.ToLower(summary.Impact), []string{"2 incoming", "1 outgoing", "main.go"})
}

func TestSummarizeChangeAddedGoFile(t *testing.T) {
	summary := summarizeChange(changeSummaryFacts{
		Label:            "new.go",
		Language:         "Go",
		Status:           changeAdded,
		Additions:        8,
		CurrentSymbols:   []string{"Build"},
		CurrentLineCount: 8,
	})

	assertContainsAll(t, strings.ToLower(summary.Previous), []string{"did not exist"})
	assertContainsAll(t, strings.ToLower(summary.Current), []string{"now", "go", "8 lines", "build"})
	assertContainsAll(t, strings.ToLower(summary.Changed), []string{"added", "new.go", "8 lines", "build"})
}

func TestSummarizeChangeDeletedFile(t *testing.T) {
	summary := summarizeChange(changeSummaryFacts{
		Label:             "old.go",
		Language:          "Go",
		Status:            changeDeleted,
		PreviousSymbols:   []string{"Legacy"},
		PreviousLineCount: 10,
	})

	assertContainsAll(t, strings.ToLower(summary.Previous), []string{"previously", "legacy"})
	assertContainsAll(t, strings.ToLower(summary.Current), []string{"removed"})
	assertContainsAll(t, strings.ToLower(summary.Changed), []string{"deleted", "old.go", "legacy"})
}

func TestSummarizeChangeNonGoFallback(t *testing.T) {
	summary := summarizeChange(changeSummaryFacts{
		Label:             "app.svelte",
		Language:          "Svelte",
		Status:            changeModified,
		Additions:         5,
		Deletions:         2,
		FirstChangedLine:  4,
		PreviousLineCount: 20,
		CurrentLineCount:  23,
	})

	assertContainsAll(t, strings.ToLower(summary.Previous), []string{"svelte", "20 lines"})
	assertContainsAll(t, strings.ToLower(summary.Current), []string{"svelte", "23 lines"})
	assertContainsAll(t, strings.ToLower(summary.Changed), []string{"modified", "line 4", "+5/-2"})
}

func TestSummarizeChangeEmptyFactsDoNotInventIntent(t *testing.T) {
	summary := summarizeChange(changeSummaryFacts{
		Label:            "unknown.go",
		Language:         "Go",
		Status:           changeModified,
		FirstChangedLine: 1,
	})

	combined := strings.ToLower(summary.Previous + " " + summary.Current + " " + summary.Changed)
	assertContainsAll(t, combined, []string{"no detected top-level go symbols", "modified"})
	for _, forbidden := range []string{"intended", "probably", "feature", "business"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("summary invented intent with %q in %q", forbidden, combined)
		}
	}
}

func TestBuildMapSnapshotIncludesChangeSummary(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("package main\n\nfunc Run() {\n\tprintln(1)\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("app.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "tester", Email: "tester@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("package main\n\nfunc Run() {\n\tprintln(2)\n\tprintln(3)\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := buildMapSnapshot(root, repo, worktree)
	if err != nil {
		t.Fatal(err)
	}

	var changed *mapNodeResponse
	for _, node := range snapshot.Response.Nodes {
		if node.ID == "app.go" {
			copy := node
			changed = &copy
			break
		}
	}
	if changed == nil || changed.Change == nil {
		t.Fatalf("app.go changed node was not returned: %#v", snapshot.Response.Nodes)
	}
	assertContainsAll(t, strings.ToLower(changed.Change.Summary.Previous), []string{"previously", "run"})
	assertContainsAll(t, strings.ToLower(changed.Change.Summary.Current), []string{"now", "run"})
	assertContainsAll(t, strings.ToLower(changed.Change.Summary.Changed), []string{"modified", "+2/-1", "run"})
	if changed.Activity.CommitsAll != 1 || len(changed.Activity.RecentCommits) != 1 {
		t.Fatalf("changed activity = %#v, want one historical commit", changed.Activity)
	}
	if len(snapshot.Response.Activity) != 24 {
		t.Fatalf("repository activity buckets = %d, want 24", len(snapshot.Response.Activity))
	}
}

func TestDescribeFileLearningExercise(t *testing.T) {
	description := strings.ToLower(describeFile(fileDescriptionFacts{
		Label:          "map.go",
		Language:       "Go",
		IsRoot:         false,
		Symbols:        []string{"runMap", "readWorkingTreeChanges"},
		IncomingCount:  1,
		OutgoingCount:  3,
		ChangeStatus:   changeModified,
		Additions:      8,
		Deletions:      2,
		TouchedSymbols: []string{"runMap"},
	}))

	for _, evidence := range []string{"go", "runmap", "modified"} {
		if !strings.Contains(description, evidence) {
			t.Fatalf("description %q does not include evidence %q", description, evidence)
		}
	}
}

func assertContainsAll(t *testing.T, text string, values []string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(text, value) {
			t.Fatalf("%q does not contain %q", text, value)
		}
	}
}
