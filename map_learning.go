package main

import (
	"path"
	"sort"
	"strings"
	"time"
)

// rootedForest is the terminal-friendly shape derived from the directed graph.
// A node appears once in Children; every remaining edge is kept in CrossLinks.
type rootedForest struct {
	Roots      []NodeID
	OtherRoots []NodeID
	Children   map[NodeID][]NodeID
	Parent     map[NodeID]NodeID
	CrossLinks []Edge
	Reachable  map[NodeID]bool
}

// changeStatus is the small, UI-independent version of Git's status codes.
// The Git adapter converts go-git's staging/worktree status into one of these
// values before the review-map logic sees it.
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

// reviewGroup summarizes one repository-relative directory. Files contains
// every analyzed source file in that directory; the other slices describe the
// smaller review-focused subset shown when the group is expanded.
type reviewGroup struct {
	ID            string
	Label         string
	Files         []NodeID
	ChangedFiles  []NodeID
	NeighborFiles []NodeID
}

type reviewMap struct {
	Groups       []reviewGroup
	Connections  []Edge
	OtherChanges []reviewChange
	Clean        bool
}

// buildReviewMap is the first exercise for the review-first map. Start with
// the two focused tests for deterministic directory grouping and changed-file
// placement. Neighbor discovery and connections will get their own tests next.
func buildReviewMap(graph *Graph, changes []reviewChange) reviewMap {
	result := reviewMap{
		Clean: len(changes) == 0,
	}

	normalizedChanges := make([]reviewChange, 0, len(changes))
	for _, change := range changes {
		normalizedPath := strings.ReplaceAll(change.Path, "\\", "/")
		normalizedPath = path.Clean(normalizedPath)
		normalizedChanges = append(normalizedChanges, reviewChange{
			Path:   normalizedPath,
			Status: change.Status,
		})
	}

	if graph == nil {
		result.OtherChanges = normalizedChanges
		sortReviewChanges(result.OtherChanges)
		return result
	}

	changedByID := make(map[NodeID]changeStatus, len(changes))
	for _, change := range normalizedChanges {
		nodeID := NodeID(change.Path)
		if _, exists := graph.Nodes[nodeID]; !exists {
			result.OtherChanges = append(result.OtherChanges, change)
			continue
		}
		changedByID[nodeID] = change.Status
	}

	groupsByID := make(map[string]*reviewGroup)
	relevantGroups := make(map[string]bool)

	for nodeID := range graph.Nodes {
		groupID := path.Dir(string(nodeID))
		group, exists := groupsByID[groupID]
		if !exists {
			group = &reviewGroup{
				ID:    groupID,
				Label: groupID,
			}
			groupsByID[groupID] = group
		}

		group.Files = append(group.Files, nodeID)

		if _, changed := changedByID[nodeID]; changed {
			group.ChangedFiles = append(group.ChangedFiles, nodeID)
			relevantGroups[groupID] = true
		}
		if result.Clean {
			relevantGroups[groupID] = true
		}
	}

	neighborIDs := make(map[NodeID]bool)
	seenConnections := make(map[Edge]bool)
	for _, edge := range graph.Edges {
		if _, exists := graph.Nodes[edge.From]; !exists {
			continue
		}
		if _, exists := graph.Nodes[edge.To]; !exists {
			continue
		}

		_, fromChanged := changedByID[edge.From]
		_, toChanged := changedByID[edge.To]
		if !result.Clean && !fromChanged && !toChanged {
			continue
		}

		if !seenConnections[edge] {
			seenConnections[edge] = true
			result.Connections = append(result.Connections, edge)
		}
		if result.Clean {
			continue
		}
		if fromChanged && !toChanged {
			neighborIDs[edge.To] = true
		}
		if toChanged && !fromChanged {
			neighborIDs[edge.From] = true
		}
	}

	for nodeID := range neighborIDs {
		groupID := path.Dir(string(nodeID))
		groupsByID[groupID].NeighborFiles = append(
			groupsByID[groupID].NeighborFiles,
			nodeID,
		)
		relevantGroups[groupID] = true
	}

	for groupID, group := range groupsByID {
		if !relevantGroups[groupID] {
			continue
		}

		sort.Slice(group.Files, func(i, j int) bool {
			return group.Files[i] < group.Files[j]
		})

		sort.Slice(group.ChangedFiles, func(i, j int) bool {
			return group.ChangedFiles[i] < group.ChangedFiles[j]
		})
		sort.Slice(group.NeighborFiles, func(i, j int) bool {
			return group.NeighborFiles[i] < group.NeighborFiles[j]
		})

		result.Groups = append(result.Groups, *group)
	}

	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].ID < result.Groups[j].ID
	})
	sort.Slice(result.Connections, func(i, j int) bool {
		left := result.Connections[i]
		right := result.Connections[j]
		if left.From != right.From {
			return left.From < right.From
		}
		if left.To != right.To {
			return left.To < right.To
		}
		return left.Kind < right.Kind
	})
	sortReviewChanges(result.OtherChanges)

	return result
}

func sortReviewChanges(changes []reviewChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Status < changes[j].Status
	})
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

// treeRow is one currently visible line in the left-hand tree pane.
type treeRow struct {
	NodeID   NodeID
	Depth    int
	Expanded bool
	IsOther  bool
}

type mapPane int

const (
	treePane mapPane = iota
	detailsPane
)

type mapState struct {
	Selected      NodeID
	Expanded      map[NodeID]bool
	ShowAll       bool
	Focus         mapPane
	Searching     bool
	SearchQuery   string
	SearchMatches []NodeID
	SearchIndex   int
}

type mapActionKind int

const (
	actionMovePrevious mapActionKind = iota
	actionMoveNext
	actionMoveParent
	actionMoveFirstChild
	actionToggleExpanded
	actionToggleAll
	actionToggleFocus
	actionBeginSearch
	actionAppendSearch
	actionBackspaceSearch
	actionAcceptSearch
	actionCancelSearch
)

type mapAction struct {
	Kind mapActionKind
	Text string
}

// buildRootedForest is your first learning exercise. The tests define root
// selection, deterministic breadth-first traversal, cycles, and cross-links.
func buildRootedForest(graph *Graph) rootedForest {
	forest := rootedForest{
		Children:  make(map[NodeID][]NodeID),
		Parent:    make(map[NodeID]NodeID),
		Reachable: make(map[NodeID]bool),
	}

	if graph == nil || len(graph.Nodes) == 0 {
		return forest
	}

	// maps dod not gurannate iteration order
	nodeIDs := make([]NodeID, 0, len(graph.Nodes))
	for nodeID := range graph.Nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Slice(nodeIDs, func(i, j int) bool {
		return nodeIDs[i] < nodeIDs[j]
	})

	// pointers
	outgoing := make(map[NodeID][]Edge)
	incomingCount := make(map[NodeID]int)

	for _, edge := range graph.Edges {
		if _, exists := graph.Nodes[edge.From]; !exists {
			continue
		}
		if _, exists := graph.Nodes[edge.To]; !exists {
			continue
		}
		outgoing[edge.From] = append(outgoing[edge.From], edge)
		incomingCount[edge.To]++
	}

	for nodeID := range outgoing {
		sort.Slice(outgoing[nodeID], func(i, j int) bool {
			left := outgoing[nodeID][i]
			right := outgoing[nodeID][j]

			if left.To != right.To {
				return left.To < right.To
			}

			return left.Kind < right.Kind
		})
	}

	var roots []NodeID

	for _, nodeID := range nodeIDs {
		if graph.Nodes[nodeID].IsRoot {
			roots = append(roots, nodeID)
		}
	}

	if len(roots) == 0 {
		for _, nodeID := range nodeIDs {
			isTestFile := strings.HasSuffix(
				strings.ToLower(string(nodeID)),
				"_test.go",
			)

			if incomingCount[nodeID] == 0 && !isTestFile {
				roots = append(roots, nodeID)
			}
		}
	}

	if len(roots) == 0 {
		roots = append(roots, nodeIDs[0])
	}

	forest.Roots = append(forest.Roots, roots...)

	visited := make(map[NodeID]bool)

	walk := func(startingNodes []NodeID, markReachable bool) {
		queue := make([]NodeID, 0, len(startingNodes))

		for _, nodeID := range startingNodes {
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true
			queue = append(queue, nodeID)

			if markReachable {
				forest.Reachable[nodeID] = true
			}
		}

		for index := 0; index < len(queue); index++ {
			from := queue[index]

			for _, edge := range outgoing[from] {
				to := edge.To
				if visited[to] {
					forest.CrossLinks = append(forest.CrossLinks, edge)
					continue
				}
				visited[to] = true
				forest.Parent[to] = from
				forest.Children[from] = append(forest.Children[from], to)
				queue = append(queue, to)

				if markReachable {
					forest.Reachable[to] = true
				}
			}
		}
	}

	walk(forest.Roots, true)
	// Anything still unvisited belongs under "Other files".
	for len(visited) < len(nodeIDs) {
		var otherRoots []NodeID

		// Find zero-incoming nodes within the remaining subgraph.
		for _, candidate := range nodeIDs {
			if visited[candidate] {
				continue
			}

			remainingIncoming := 0

			for _, edge := range graph.Edges {
				if edge.To != candidate {
					continue
				}

				if visited[edge.From] {
					continue
				}

				if _, exists := graph.Nodes[edge.From]; !exists {
					continue
				}

				remainingIncoming++
			}

			if remainingIncoming == 0 {
				otherRoots = append(otherRoots, candidate)
			}
		}

		// A disconnected cycle has no zero-incoming node.
		if len(otherRoots) == 0 {
			for _, nodeID := range nodeIDs {
				if !visited[nodeID] {
					otherRoots = append(otherRoots, nodeID)
					break
				}
			}
		}

		forest.OtherRoots = append(
			forest.OtherRoots,
			otherRoots...,
		)

		walk(otherRoots, false)
	}

	// Keep cross-links stable for tests and terminal output.
	sort.Slice(forest.CrossLinks, func(i, j int) bool {
		left := forest.CrossLinks[i]
		right := forest.CrossLinks[j]

		if left.From != right.From {
			return left.From < right.From
		}

		if left.To != right.To {
			return left.To < right.To
		}

		return left.Kind < right.Kind
	})

	return forest
}

// visibleTreeRows is your bridge between domain state and terminal output.
// It should flatten only expanded branches and optionally append OtherRoots.
func visibleTreeRows(state mapState, forest rootedForest) []treeRow {
	rows := make([]treeRow, 0)

	var appendNode func(nodeID NodeID, depth int, isOther bool)

	appendNode = func(nodeID NodeID, depth int, isOther bool) {
		expanded := state.Expanded[nodeID]

		rows = append(rows, treeRow{
			NodeID:   nodeID,
			Depth:    depth,
			Expanded: expanded,
			IsOther:  isOther,
		})

		if !expanded {
			return
		}
		for _, childID := range forest.Children[nodeID] {
			appendNode(childID, depth+1, isOther)
		}
	}

	for _, rootID := range forest.Roots {
		appendNode(rootID, 0, false)
	}

	if state.ShowAll {
		for _, rootID := range forest.OtherRoots {
			appendNode(rootID, 0, true)
		}
	}

	return rows
}

// searchGraph is intentionally independent of the TUI framework.
func searchGraph(graph *Graph, query string) []NodeID {
	// Remove surrounding spaces and make matching case-insensitive.
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))

	if graph == nil || normalizedQuery == "" {
		return nil
	}

	var matches []NodeID

	for nodeID, node := range graph.Nodes {
		searchableFields := []string{
			string(nodeID),
			node.Label,
			node.Path,
			node.Language,
		}

		for _, field := range searchableFields {
			normalizedField := strings.ToLower(field)

			if strings.Contains(normalizedField, normalizedQuery) {
				matches = append(matches, nodeID)

				// One node should appear only once, even if several
				// of its fields match.
				break
			}
		}
	}

	// Graph.Nodes is a map, so sort the results for stable output.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] < matches[j]
	})

	return matches
}

// reduceMapState applies one user intent without knowing which key produced it.
func reduceMapState(
	state mapState,
	action mapAction,
	forest rootedForest,
	graph *Graph,
) mapState {
	rows := visibleTreeRows(state, forest)

	selectedIndex := -1

	for index, row := range rows {
		if row.NodeID == state.Selected {
			selectedIndex = index
			break
		}
	}

	// Copy the map before changing it. Maps are reference-like values in Go;
	// changing the original map would also change the caller's previous state.
	cloneExpanded := func() {
		expandedCopy := make(map[NodeID]bool, len(state.Expanded))

		for nodeID, expanded := range state.Expanded {
			expandedCopy[nodeID] = expanded
		}

		state.Expanded = expandedCopy
	}

	switch action.Kind {
	case actionMovePrevious:
		if state.Searching {
			if state.SearchIndex > 0 {
				state.SearchIndex--
			}
			break
		}

		if selectedIndex > 0 {
			state.Selected = rows[selectedIndex-1].NodeID
		}

	case actionMoveNext:
		if state.Searching {
			if state.SearchIndex+1 < len(state.SearchMatches) {
				state.SearchIndex++
			}
			break
		}

		if selectedIndex >= 0 && selectedIndex+1 < len(rows) {
			state.Selected = rows[selectedIndex+1].NodeID
		}

	case actionMoveParent:
		children := forest.Children[state.Selected]

		// Left collapses an expanded branch first.
		if state.Expanded[state.Selected] && len(children) > 0 {
			cloneExpanded()
			state.Expanded[state.Selected] = false
			break
		}

		// If already collapsed, left moves to the parent.
		if parentID, exists := forest.Parent[state.Selected]; exists {
			state.Selected = parentID
		}

	case actionMoveFirstChild:
		children := forest.Children[state.Selected]

		if len(children) == 0 {
			break
		}

		// Right expands a collapsed branch first.
		if !state.Expanded[state.Selected] {
			cloneExpanded()
			state.Expanded[state.Selected] = true
			break
		}

		// If already expanded, right moves to its first child.
		state.Selected = children[0]

	case actionToggleExpanded:
		if len(forest.Children[state.Selected]) == 0 {
			break
		}

		cloneExpanded()
		state.Expanded[state.Selected] = !state.Expanded[state.Selected]

	case actionToggleAll:
		state.ShowAll = !state.ShowAll

		// If "Other files" is hidden while one of those files is selected,
		// move selection back to the first main root.
		if !state.ShowAll && !forest.Reachable[state.Selected] {
			if len(forest.Roots) > 0 {
				state.Selected = forest.Roots[0]
			} else {
				state.Selected = ""
			}
		}

	case actionToggleFocus:
		if state.Focus == treePane {
			state.Focus = detailsPane
		} else {
			state.Focus = treePane
		}

	case actionBeginSearch:
		state.Searching = true
		state.SearchQuery = ""
		state.SearchMatches = nil
		state.SearchIndex = 0

	case actionAppendSearch:
		state.SearchQuery += action.Text
		state.SearchMatches = searchGraph(graph, state.SearchQuery)
		state.SearchIndex = 0

	case actionBackspaceSearch:
		queryRunes := []rune(state.SearchQuery)

		if len(queryRunes) > 0 {
			queryRunes = queryRunes[:len(queryRunes)-1]
			state.SearchQuery = string(queryRunes)
		}

		state.SearchMatches = searchGraph(graph, state.SearchQuery)
		state.SearchIndex = 0

	case actionAcceptSearch:
		if state.SearchIndex >= 0 && state.SearchIndex < len(state.SearchMatches) {
			selectedNode := state.SearchMatches[state.SearchIndex]
			state.Selected = selectedNode

			if !forest.Reachable[selectedNode] {
				state.ShowAll = true
			}

			// Expand every ancestor so the selected result is visible.
			cloneExpanded()
			currentNode := selectedNode

			for {
				parentID, exists := forest.Parent[currentNode]
				if !exists {
					break
				}

				state.Expanded[parentID] = true
				currentNode = parentID
			}
		}

		state.Searching = false
		state.SearchQuery = ""
		state.SearchMatches = nil
		state.SearchIndex = 0

	case actionCancelSearch:
		state.Searching = false
		state.SearchQuery = ""
		state.SearchMatches = nil
		state.SearchIndex = 0
	}

	return state
}

func initialMapState(forest rootedForest) mapState {
	state := mapState{
		Expanded: make(map[NodeID]bool),
		Focus:    treePane,
	}
	if len(forest.Roots) > 0 {
		state.Selected = forest.Roots[0]
		state.Expanded[state.Selected] = true
	}
	return state
}
