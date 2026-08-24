package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestActionForKey(t *testing.T) {
	tests := []struct {
		key  string
		kind mapActionKind
	}{
		{key: "up", kind: actionMovePrevious},
		{key: "j", kind: actionMoveNext},
		{key: "left", kind: actionMoveParent},
		{key: "l", kind: actionMoveFirstChild},
		{key: "enter", kind: actionToggleExpanded},
		{key: "a", kind: actionToggleAll},
		{key: "tab", kind: actionToggleFocus},
		{key: "/", kind: actionBeginSearch},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			action, handled := actionForKey(test.key)
			if !handled || action.Kind != test.kind {
				t.Fatalf("actionForKey(%q) = %#v, %t; want kind %v", test.key, action, handled, test.kind)
			}
		})
	}

	if _, handled := actionForKey("unknown"); handled {
		t.Fatal("unknown key should not be handled")
	}
}

func TestWindowAroundSelectionKeepsSelectedLineVisible(t *testing.T) {
	lines := []string{"0", "1", "2", "3", "4", "5"}

	got := windowAroundSelection(lines, 4, 3)
	want := []string{"3", "4", "5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("window = %#v, want %#v", got, want)
	}

	got = windowAroundSelection(lines, 0, 3)
	want = []string{"0", "1", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("first window = %#v, want %#v", got, want)
	}
}

func TestSearchResultsContentShowsActiveMatch(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "a.go", Label: "a.go", Language: "Go"},
			{ID: "b.go", Label: "b.go", Language: "Go"},
		},
		nil,
	)
	model := newMapTUIModel("repo", graph, func(string) error { return nil })
	model.state.Searching = true
	model.state.SearchQuery = ".go"
	model.state.SearchMatches = []NodeID{"a.go", "b.go"}
	model.state.SearchIndex = 1

	content := model.searchResultsContent(40, 5)
	if !strings.Contains(content, "a.go") || !strings.Contains(content, "b.go") {
		t.Fatalf("search content does not include both matches: %q", content)
	}
	if !strings.Contains(content, "\x1b[") {
		t.Fatalf("active search result is not styled: %q", content)
	}
}

func TestOpenSelectedFileUsesInjectedOpener(t *testing.T) {
	wantPath := `C:\repo\main.go`
	graph := graphForLearningTest(
		[]Node{{ID: "main.go", Path: wantPath, IsRoot: true}},
		nil,
	)
	var openedPath string
	model := newMapTUIModel("repo", graph, func(path string) error {
		openedPath = path
		return nil
	})

	message := model.openSelectedFile()()
	result, ok := message.(fileOpenedMsg)
	if !ok || result.err != nil {
		t.Fatalf("open command result = %#v", message)
	}
	if openedPath != wantPath {
		t.Fatalf("opened path = %q, want %q", openedPath, wantPath)
	}
}

func TestOpenSelectedFileReportsOpenerError(t *testing.T) {
	wantError := errors.New("VS Code unavailable")
	graph := graphForLearningTest(
		[]Node{{ID: "main.go", Path: `C:\repo\main.go`, IsRoot: true}},
		nil,
	)
	model := newMapTUIModel("repo", graph, func(string) error { return wantError })

	message := model.openSelectedFile()()
	result := message.(fileOpenedMsg)
	if !errors.Is(result.err, wantError) {
		t.Fatalf("open error = %v, want %v", result.err, wantError)
	}
}

func TestReviewViewRendersMinimalMapAndActivity(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "cmd/main.go", Label: "main.go", Language: "Go", IsRoot: true},
			{ID: "internal/service.go", Label: "service.go", Language: "Go"},
		},
		[]Edge{{From: "cmd/main.go", To: "internal/service.go", Kind: EdgeKindCalls}},
	)
	changes := []reviewChange{{Path: "internal/service.go", Status: changeModified}}
	boundary := Boundary{
		Since: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, time.August, 1, 23, 59, 0, 0, time.UTC),
	}
	model := newMapTUIModelWithReview(
		"repo",
		graph,
		buildReviewMap(graph, changes),
		changes,
		map[int]int{0: 1},
		boundary,
		"dev@example.com",
		"",
		func(string) error { return nil },
	)
	model.width = 140
	model.height = 42
	model.resizeDetails()

	view := model.render()
	for _, text := range []string{
		"codemap",
		"[1] tree",
		"[2] map",
		"reviewing working tree",
		"internal",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("review view does not contain %q:\n%s", text, view)
		}
	}
	if strings.Contains(view, "Commits over time") {
		t.Fatal("activity panel should be closed by default")
	}

	model.calendarOpen = true
	model.resizeDetails()
	if openedView := model.render(); !strings.Contains(openedView, "Commits over time") {
		t.Fatal("activity panel should render after it is opened")
	}

	model.calendarOpen = false
	model.width = 80
	model.height = 30
	model.resizeDetails()
	narrowView := model.render()
	if lines := strings.Count(narrowView, "\n") + 1; lines > model.height {
		t.Fatalf("narrow responsive view uses %d lines, terminal height is %d", lines, model.height)
	}
}
