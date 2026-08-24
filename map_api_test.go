package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
