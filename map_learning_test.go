package main

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildRootedForestUsesExplicitRootsAndKeepsCrossLinks(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "main.go", IsRoot: true},
			{ID: "a.go"},
			{ID: "b.go"},
			{ID: "shared.go"},
		},
		[]Edge{
			{From: "main.go", To: "b.go", Kind: EdgeKindCalls},
			{From: "main.go", To: "a.go", Kind: EdgeKindCalls},
			{From: "a.go", To: "shared.go", Kind: EdgeKindCalls},
			{From: "b.go", To: "shared.go", Kind: EdgeKindCalls},
		},
	)

	forest := buildRootedForest(graph)

	// Sorting makes output deterministic even though Graph.Nodes is a Go map.
	assertNodeIDs(t, "roots", forest.Roots, []NodeID{"main.go"})
	assertNodeIDs(t, "main children", forest.Children["main.go"], []NodeID{"a.go", "b.go"})
	assertNodeIDs(t, "a children", forest.Children["a.go"], []NodeID{"shared.go"})

	if got := forest.Parent["shared.go"]; got != "a.go" {
		t.Fatalf("shared.go parent = %q, want %q", got, NodeID("a.go"))
	}
	if !forest.Reachable["shared.go"] {
		t.Fatal("shared.go should be reachable from the explicit root")
	}

	wantCrossLinks := []Edge{{From: "b.go", To: "shared.go", Kind: EdgeKindCalls}}
	if !reflect.DeepEqual(forest.CrossLinks, wantCrossLinks) {
		t.Fatalf("cross-links = %#v, want %#v", forest.CrossLinks, wantCrossLinks)
	}
}

func TestBuildRootedForestFallbackDoesNotPromoteTestFiles(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{{ID: "app.go"}, {ID: "lib.go"}, {ID: "app_test.go"}},
		[]Edge{{From: "app.go", To: "lib.go", Kind: EdgeKindCalls}},
	)

	forest := buildRootedForest(graph)

	assertNodeIDs(t, "fallback roots", forest.Roots, []NodeID{"app.go"})
	assertNodeIDs(t, "other roots", forest.OtherRoots, []NodeID{"app_test.go"})
	if forest.Reachable["app_test.go"] {
		t.Fatal("app_test.go should be hidden from the initial rooted view")
	}
}

func TestBuildRootedForestBreaksDisconnectedCyclesDeterministically(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "main.go", IsRoot: true},
			{ID: "service.go"},
			{ID: "x.go"},
			{ID: "y.go"},
		},
		[]Edge{
			{From: "main.go", To: "service.go", Kind: EdgeKindCalls},
			{From: "x.go", To: "y.go", Kind: EdgeKindCalls},
			{From: "y.go", To: "x.go", Kind: EdgeKindCalls},
		},
	)

	forest := buildRootedForest(graph)

	assertNodeIDs(t, "other roots", forest.OtherRoots, []NodeID{"x.go"})
	assertNodeIDs(t, "cycle children", forest.Children["x.go"], []NodeID{"y.go"})
	if got := forest.Parent["y.go"]; got != "x.go" {
		t.Fatalf("y.go parent = %q, want %q", got, NodeID("x.go"))
	}
	wantCrossLink := Edge{From: "y.go", To: "x.go", Kind: EdgeKindCalls}
	if !containsEdge(forest.CrossLinks, wantCrossLink) {
		t.Fatalf("cross-links %#v do not contain %#v", forest.CrossLinks, wantCrossLink)
	}
}

func TestVisibleTreeRowsRespectsExpansionAndShowAll(t *testing.T) {
	forest := learningStateForest()
	state := mapState{
		Selected: "main.go",
		Expanded: map[NodeID]bool{"main.go": true, "a.go": true, "unused.go": true},
	}

	rows := visibleTreeRows(state, forest)
	assertRows(t, rows, []treeRow{
		{NodeID: "main.go", Depth: 0, Expanded: true},
		{NodeID: "a.go", Depth: 1, Expanded: true},
		{NodeID: "nested.go", Depth: 2},
		{NodeID: "b.go", Depth: 1},
	})

	state.ShowAll = true
	rows = visibleTreeRows(state, forest)
	assertRows(t, rows, []treeRow{
		{NodeID: "main.go", Depth: 0, Expanded: true},
		{NodeID: "a.go", Depth: 1, Expanded: true},
		{NodeID: "nested.go", Depth: 2},
		{NodeID: "b.go", Depth: 1},
		{NodeID: "unused.go", Depth: 0, Expanded: true, IsOther: true},
		{NodeID: "unused_child.go", Depth: 1, IsOther: true},
	})
}

func TestReduceMapStateNavigatesVisibleRows(t *testing.T) {
	forest := learningStateForest()
	graph := graphForLearningTest(nil, nil)
	state := mapState{
		Selected: "main.go",
		Expanded: map[NodeID]bool{"main.go": true, "a.go": true},
	}

	state = reduceMapState(state, mapAction{Kind: actionMoveNext}, forest, graph)
	if state.Selected != "a.go" {
		t.Fatalf("after moving next, selected = %q, want a.go", state.Selected)
	}

	state = reduceMapState(state, mapAction{Kind: actionMoveFirstChild}, forest, graph)
	if state.Selected != "nested.go" {
		t.Fatalf("after moving to first child, selected = %q, want nested.go", state.Selected)
	}

	state = reduceMapState(state, mapAction{Kind: actionMoveParent}, forest, graph)
	if state.Selected != "a.go" {
		t.Fatalf("after moving to parent, selected = %q, want a.go", state.Selected)
	}
}

func TestReduceMapStateRevealsAcceptedSearchResult(t *testing.T) {
	forest := learningStateForest()
	graph := graphForLearningTest(
		[]Node{
			{ID: "main.go", Label: "main.go", Language: "Go"},
			{ID: "unused.go", Label: "unused.go", Language: "Go"},
			{ID: "unused_child.go", Label: "unused_child.go", Language: "Go"},
		},
		nil,
	)
	state := mapState{
		Selected:      "main.go",
		Expanded:      map[NodeID]bool{"main.go": true},
		Searching:     true,
		SearchQuery:   "CHILD",
		SearchMatches: []NodeID{"unused_child.go"},
	}

	state = reduceMapState(state, mapAction{Kind: actionAcceptSearch}, forest, graph)

	if state.Selected != "unused_child.go" {
		t.Fatalf("selected = %q, want unused_child.go", state.Selected)
	}
	if !state.ShowAll {
		t.Fatal("accepting a hidden search result should enable ShowAll")
	}
	if !state.Expanded["unused.go"] {
		t.Fatal("accepting a result should expand its ancestors")
	}
	if state.Searching {
		t.Fatal("accepting a result should close search mode")
	}
}

func TestReduceMapStateHandlesVisibilityFocusAndSearchEditing(t *testing.T) {
	forest := learningStateForest()
	graph := graphForLearningTest(
		[]Node{
			{ID: "main.go", Label: "main.go", Language: "Go"},
			{ID: "café.go", Label: "café.go", Language: "Go"},
		},
		nil,
	)
	state := mapState{
		Selected: "unused.go",
		Expanded: map[NodeID]bool{"main.go": true},
		ShowAll:  true,
	}

	state = reduceMapState(state, mapAction{Kind: actionToggleAll}, forest, graph)
	if state.ShowAll || state.Selected != "main.go" {
		t.Fatalf("hiding other files produced ShowAll=%t Selected=%q", state.ShowAll, state.Selected)
	}

	state = reduceMapState(state, mapAction{Kind: actionToggleFocus}, forest, graph)
	if state.Focus != detailsPane {
		t.Fatalf("focus = %v, want detailsPane", state.Focus)
	}

	state = reduceMapState(state, mapAction{Kind: actionBeginSearch}, forest, graph)
	state = reduceMapState(state, mapAction{Kind: actionAppendSearch, Text: "é"}, forest, graph)
	state = reduceMapState(state, mapAction{Kind: actionAppendSearch, Text: "x"}, forest, graph)
	state = reduceMapState(state, mapAction{Kind: actionBackspaceSearch}, forest, graph)
	if state.SearchQuery != "é" {
		t.Fatalf("query after Unicode backspace = %q, want %q", state.SearchQuery, "é")
	}
	assertNodeIDs(t, "search matches", state.SearchMatches, []NodeID{"café.go"})

	state = reduceMapState(state, mapAction{Kind: actionCancelSearch}, forest, graph)
	if state.Searching || state.SearchQuery != "" || state.SearchMatches != nil {
		t.Fatalf("cancelled search was not cleared: %#v", state)
	}
}

func TestReduceMapStateKeepsSearchIndexWithinMatches(t *testing.T) {
	forest := learningStateForest()
	state := mapState{
		Searching:     true,
		SearchMatches: []NodeID{"a.go", "b.go"},
	}

	state = reduceMapState(state, mapAction{Kind: actionMovePrevious}, forest, NewGraph())
	if state.SearchIndex != 0 {
		t.Fatalf("search index moved above first match: %d", state.SearchIndex)
	}
	state = reduceMapState(state, mapAction{Kind: actionMoveNext}, forest, NewGraph())
	state = reduceMapState(state, mapAction{Kind: actionMoveNext}, forest, NewGraph())
	if state.SearchIndex != 1 {
		t.Fatalf("search index = %d, want last match index 1", state.SearchIndex)
	}
}

func TestSearchGraphMatchesPathLabelAndLanguageCaseInsensitively(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "cmd/server.go", Label: "server.go", Path: `C:\repo\cmd\server.go`, Language: "Go"},
			{ID: "web/App.tsx", Label: "App.tsx", Path: `C:\repo\web\App.tsx`, Language: "TypeScript"},
		},
		nil,
	)

	assertNodeIDs(t, "path search", searchGraph(graph, "CMD/"), []NodeID{"cmd/server.go"})
	assertNodeIDs(t, "label search", searchGraph(graph, "app.TSX"), []NodeID{"web/App.tsx"})
	assertNodeIDs(t, "language search", searchGraph(graph, "typescript"), []NodeID{"web/App.tsx"})
	assertNodeIDs(t, "empty search", searchGraph(graph, "   "), nil)
}

func TestBuildReviewMapGroupsCleanRepositoryDeterministically(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "internal/z.go"},
			{ID: "main.go"},
			{ID: "cmd/tool/main.go"},
			{ID: "internal/a.go"},
		},
		nil,
	)

	got := buildReviewMap(graph, nil)
	want := reviewMap{
		Clean: true,
		Groups: []reviewGroup{
			{ID: ".", Label: ".", Files: []NodeID{"main.go"}},
			{ID: "cmd/tool", Label: "cmd/tool", Files: []NodeID{"cmd/tool/main.go"}},
			{ID: "internal", Label: "internal", Files: []NodeID{"internal/a.go", "internal/z.go"}},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildReviewMap() = %#v\nwant %#v", got, want)
	}
}

func TestBuildReviewMapPlacesChangedFileInItsGroup(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "cmd/tool/main.go"},
			{ID: "internal/z.go"},
			{ID: "internal/a.go"},
		},
		nil,
	)
	changes := []reviewChange{
		{Path: `internal\z.go`, Status: changeModified},
	}

	got := buildReviewMap(graph, changes)
	want := reviewMap{
		Clean: false,
		Groups: []reviewGroup{
			{
				ID:           "internal",
				Label:        "internal",
				Files:        []NodeID{"internal/a.go", "internal/z.go"},
				ChangedFiles: []NodeID{"internal/z.go"},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildReviewMap() = %#v\nwant %#v", got, want)
	}
}

func TestBuildReviewMapIncludesDirectNeighborsAndDeduplicatesConnections(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "cmd/main.go"},
			{ID: "internal/helper.go"},
			{ID: "internal/service.go"},
			{ID: "pkg/store/store.go"},
			{ID: "unrelated/log.go"},
		},
		[]Edge{
			{From: "cmd/main.go", To: "internal/service.go", Kind: EdgeKindCalls},
			{From: "cmd/main.go", To: "internal/service.go", Kind: EdgeKindCalls},
			{From: "internal/service.go", To: "internal/helper.go", Kind: EdgeKindCalls},
			{From: "internal/service.go", To: "pkg/store/store.go", Kind: EdgeKindCalls},
			{From: "unrelated/log.go", To: "pkg/store/store.go", Kind: EdgeKindCalls},
		},
	)
	changes := []reviewChange{
		{Path: "internal/service.go", Status: changeModified},
	}

	got := buildReviewMap(graph, changes)
	want := reviewMap{
		Groups: []reviewGroup{
			{
				ID:            "cmd",
				Label:         "cmd",
				Files:         []NodeID{"cmd/main.go"},
				NeighborFiles: []NodeID{"cmd/main.go"},
			},
			{
				ID:            "internal",
				Label:         "internal",
				Files:         []NodeID{"internal/helper.go", "internal/service.go"},
				ChangedFiles:  []NodeID{"internal/service.go"},
				NeighborFiles: []NodeID{"internal/helper.go"},
			},
			{
				ID:            "pkg/store",
				Label:         "pkg/store",
				Files:         []NodeID{"pkg/store/store.go"},
				NeighborFiles: []NodeID{"pkg/store/store.go"},
			},
		},
		Connections: []Edge{
			{From: "cmd/main.go", To: "internal/service.go", Kind: EdgeKindCalls},
			{From: "internal/service.go", To: "internal/helper.go", Kind: EdgeKindCalls},
			{From: "internal/service.go", To: "pkg/store/store.go", Kind: EdgeKindCalls},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildReviewMap() = %#v\nwant %#v", got, want)
	}
}

func TestBuildReviewMapPreservesChangesOutsideAnalyzedGraph(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{{ID: "main.go"}},
		nil,
	)
	changes := []reviewChange{
		{Path: `legacy\old.go`, Status: changeDeleted},
		{Path: "docs/README.md", Status: changeModified},
	}

	got := buildReviewMap(graph, changes)
	want := reviewMap{
		OtherChanges: []reviewChange{
			{Path: "docs/README.md", Status: changeModified},
			{Path: "legacy/old.go", Status: changeDeleted},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildReviewMap() = %#v\nwant %#v", got, want)
	}
}

func TestAggregateRepoActivityFiltersEmailAndBoundary(t *testing.T) {
	boundary := Boundary{
		Since: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, time.August, 3, 23, 59, 0, 0, time.UTC),
	}
	commits := []activityCommit{
		{When: time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC), Email: "Dev@example.com"},
		{When: time.Date(2026, time.August, 2, 18, 0, 0, 0, time.UTC), Email: "dev@example.com"},
		{When: time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC), Email: "other@example.com"},
		{When: time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC), Email: "dev@example.com"},
	}

	got := aggregateRepoActivity(commits, "DEV@example.com", boundary)
	want := map[int]int{1: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aggregateRepoActivity() = %#v, want %#v", got, want)
	}
}

func graphForLearningTest(nodes []Node, edges []Edge) *Graph {
	graph := NewGraph()
	for _, node := range nodes {
		graph.AddNode(node)
	}
	graph.Edges = append(graph.Edges, edges...)
	return graph
}

func learningStateForest() rootedForest {
	return rootedForest{
		Roots:      []NodeID{"main.go"},
		OtherRoots: []NodeID{"unused.go"},
		Children: map[NodeID][]NodeID{
			"main.go":   {"a.go", "b.go"},
			"a.go":      {"nested.go"},
			"unused.go": {"unused_child.go"},
		},
		Parent: map[NodeID]NodeID{
			"a.go":            "main.go",
			"b.go":            "main.go",
			"nested.go":       "a.go",
			"unused_child.go": "unused.go",
		},
		Reachable: map[NodeID]bool{
			"main.go":   true,
			"a.go":      true,
			"b.go":      true,
			"nested.go": true,
		},
	}
}

func assertNodeIDs(t *testing.T, name string, got, want []NodeID) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func assertRows(t *testing.T, got, want []treeRow) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %#v, want %#v", got, want)
	}
}

func containsEdge(edges []Edge, want Edge) bool {
	for _, edge := range edges {
		if edge == want {
			return true
		}
	}
	return false
}
