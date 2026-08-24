package main

import (
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	minimumMapWidth  = 60
	minimumMapHeight = 20
	wideMapWidth     = 110
	otherChangesID   = "__other_changes__"
)

type fileOpenedMsg struct{ err error }

type mapView int

const (
	treeView mapView = iota
	reviewView
)

type reviewRowKind int

const (
	reviewGroupRow reviewRowKind = iota
	reviewFileRow
	reviewOtherRow
)

type reviewItemID struct {
	GroupID   string
	NodeID    NodeID
	OtherPath string
}

type reviewRow struct {
	ID       reviewItemID
	Kind     reviewRowKind
	Expanded bool
	Changed  bool
	Neighbor bool
	Status   changeStatus
}

type reviewColumn struct {
	Title string
	Rows  []reviewRow
}

type mapTUIModel struct {
	root           string
	graph          *Graph
	forest         rootedForest
	review         reviewMap
	changes        map[NodeID]changeStatus
	state          mapState
	activeView     mapView
	reviewIndex    int
	expandedGroups map[string]bool
	calendarOpen   bool
	activity       map[int]int
	activityRange  Boundary
	activityEmail  string
	details        viewport.Model
	width          int
	height         int
	status         string
	openFile       func(string) error
}

func runMapTUI(
	root string,
	graph *Graph,
	review reviewMap,
	changes []reviewChange,
	activity map[int]int,
	activityRange Boundary,
	activityEmail string,
	status string,
) error {
	model := newMapTUIModelWithReview(
		root, graph, review, changes, activity, activityRange,
		activityEmail, status, openFileInVSCode,
	)
	program := tea.NewProgram(model)
	_, err := program.Run()
	return err
}

func newMapTUIModel(root string, graph *Graph, opener func(string) error) mapTUIModel {
	boundary, _ := setTimeFlags("", "")
	model := newMapTUIModelWithReview(
		root, graph, buildReviewMap(graph, nil), nil, map[int]int{},
		*boundary, "", "", opener,
	)
	model.activeView = treeView
	return model
}

func newMapTUIModelWithReview(
	root string,
	graph *Graph,
	review reviewMap,
	changes []reviewChange,
	activity map[int]int,
	activityRange Boundary,
	activityEmail string,
	status string,
	opener func(string) error,
) mapTUIModel {
	forest := buildRootedForest(graph)
	changeIndex := make(map[NodeID]changeStatus)
	for _, change := range changes {
		normalized := path.Clean(strings.ReplaceAll(change.Path, "\\", "/"))
		changeIndex[NodeID(normalized)] = change.Status
	}

	model := mapTUIModel{
		root:           root,
		graph:          graph,
		forest:         forest,
		review:         review,
		changes:        changeIndex,
		state:          initialMapState(forest),
		activeView:     reviewView,
		expandedGroups: make(map[string]bool),
		calendarOpen:   false,
		activity:       activity,
		activityRange:  activityRange,
		activityEmail:  activityEmail,
		details:        viewport.New(),
		status:         status,
		openFile:       opener,
	}
	model.syncReviewSelection()
	return model
}

func (model mapTUIModel) Init() tea.Cmd { return nil }

func (model mapTUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		model.resizeDetails()
		return model, nil
	case fileOpenedMsg:
		if message.err != nil {
			model.status = "Could not open VS Code: " + message.err.Error()
		} else {
			model.status = "Opened " + string(model.state.Selected)
		}
		return model, nil
	case tea.KeyPressMsg:
		if model.state.Searching {
			return model.updateSearch(message)
		}

		switch message.String() {
		case "q", "ctrl+c":
			return model, tea.Quit
		case "1":
			model.activeView = treeView
			model.refreshDetails()
			return model, nil
		case "2":
			model.activeView = reviewView
			model.revealReviewNode(model.state.Selected)
			model.refreshDetails()
			return model, nil
		case "c":
			model.calendarOpen = !model.calendarOpen
			model.resizeDetails()
			return model, nil
		case "tab", "/", "a":
			action, _ := actionForKey(message.String())
			model.state = reduceMapState(model.state, action, model.forest, model.graph)
			model.refreshDetails()
			return model, nil
		case "o":
			return model, model.openSelectedFile()
		}

		if model.state.Focus == detailsPane {
			updated, command := model.details.Update(message)
			model.details = updated
			return model, command
		}

		if model.activeView == reviewView {
			command := model.updateReviewKey(message.String())
			model.refreshDetails()
			return model, command
		}

		action, handled := actionForKey(message.String())
		if handled {
			model.state = reduceMapState(model.state, action, model.forest, model.graph)
			model.refreshDetails()
		}
	}

	return model, nil
}

func actionForKey(key string) (mapAction, bool) {
	switch key {
	case "up", "k":
		return mapAction{Kind: actionMovePrevious}, true
	case "down", "j":
		return mapAction{Kind: actionMoveNext}, true
	case "left", "h":
		return mapAction{Kind: actionMoveParent}, true
	case "right", "l":
		return mapAction{Kind: actionMoveFirstChild}, true
	case "enter":
		return mapAction{Kind: actionToggleExpanded}, true
	case "a":
		return mapAction{Kind: actionToggleAll}, true
	case "tab":
		return mapAction{Kind: actionToggleFocus}, true
	case "/":
		return mapAction{Kind: actionBeginSearch}, true
	default:
		return mapAction{}, false
	}
}

func (model mapTUIModel) updateSearch(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var action mapAction
	switch key.String() {
	case "esc":
		action = mapAction{Kind: actionCancelSearch}
	case "enter":
		action = mapAction{Kind: actionAcceptSearch}
	case "backspace":
		action = mapAction{Kind: actionBackspaceSearch}
	case "up":
		action = mapAction{Kind: actionMovePrevious}
	case "down":
		action = mapAction{Kind: actionMoveNext}
	default:
		if key.Key().Text == "" {
			return model, nil
		}
		action = mapAction{Kind: actionAppendSearch, Text: key.Key().Text}
	}

	model.state = reduceMapState(model.state, action, model.forest, model.graph)
	if action.Kind == actionAcceptSearch {
		model.revealReviewNode(model.state.Selected)
	}
	model.refreshDetails()
	return model, nil
}

func (model *mapTUIModel) updateReviewKey(key string) tea.Cmd {
	rows := model.visibleReviewRows()
	if len(rows) == 0 {
		return nil
	}
	if model.reviewIndex < 0 || model.reviewIndex >= len(rows) {
		model.reviewIndex = 0
	}

	current := rows[model.reviewIndex]
	switch key {
	case "up", "k":
		if model.reviewIndex > 0 {
			model.reviewIndex--
		}
	case "down", "j":
		if model.reviewIndex+1 < len(rows) {
			model.reviewIndex++
		}
	case "left", "h":
		if current.Kind == reviewFileRow || current.Kind == reviewOtherRow {
			model.selectReviewItem(reviewItemID{GroupID: current.ID.GroupID})
		} else if current.Expanded {
			model.expandedGroups[current.ID.GroupID] = false
		}
	case "right", "l":
		if current.Kind == reviewGroupRow {
			if !current.Expanded {
				model.expandedGroups[current.ID.GroupID] = true
			} else if model.reviewIndex+1 < len(rows) && rows[model.reviewIndex+1].ID.GroupID == current.ID.GroupID {
				model.reviewIndex++
			}
		}
	case "enter":
		if current.Kind == reviewGroupRow {
			model.expandedGroups[current.ID.GroupID] = !current.Expanded
		} else if current.Kind == reviewFileRow {
			return model.openSelectedFile()
		}
	}

	rows = model.visibleReviewRows()
	if model.reviewIndex >= len(rows) {
		model.reviewIndex = max(0, len(rows)-1)
	}
	model.syncReviewSelection()
	return nil
}

func (model *mapTUIModel) syncReviewSelection() {
	row, exists := model.currentReviewRow()
	if exists && row.ID.NodeID != "" {
		model.state.Selected = row.ID.NodeID
	}
}

func (model *mapTUIModel) selectReviewItem(id reviewItemID) {
	for index, row := range model.visibleReviewRows() {
		if row.ID == id {
			model.reviewIndex = index
			model.syncReviewSelection()
			return
		}
	}
}

func (model *mapTUIModel) revealReviewNode(nodeID NodeID) {
	if nodeID == "" {
		return
	}
	groupID := path.Dir(string(nodeID))
	model.expandedGroups[groupID] = true
	model.selectReviewItem(reviewItemID{GroupID: groupID, NodeID: nodeID})
}

func (model mapTUIModel) currentReviewRow() (reviewRow, bool) {
	rows := model.visibleReviewRows()
	if model.reviewIndex < 0 || model.reviewIndex >= len(rows) {
		return reviewRow{}, false
	}
	return rows[model.reviewIndex], true
}

func (model mapTUIModel) visibleReviewRows() []reviewRow {
	var rows []reviewRow
	for _, group := range model.review.Groups {
		expanded := model.expandedGroups[group.ID]
		rows = append(rows, reviewRow{
			ID:       reviewItemID{GroupID: group.ID},
			Kind:     reviewGroupRow,
			Expanded: expanded,
		})
		if !expanded {
			continue
		}

		visible := group.Files
		if !model.review.Clean {
			visible = append([]NodeID(nil), group.ChangedFiles...)
			seen := make(map[NodeID]bool, len(visible))
			for _, nodeID := range visible {
				seen[nodeID] = true
			}
			for _, nodeID := range group.NeighborFiles {
				if !seen[nodeID] {
					visible = append(visible, nodeID)
					seen[nodeID] = true
				}
			}
		}

		for _, nodeID := range visible {
			status, changed := model.changes[nodeID]
			rows = append(rows, reviewRow{
				ID:       reviewItemID{GroupID: group.ID, NodeID: nodeID},
				Kind:     reviewFileRow,
				Changed:  changed,
				Neighbor: !changed && !model.review.Clean,
				Status:   status,
			})
		}
	}

	if len(model.review.OtherChanges) > 0 {
		expanded := model.expandedGroups[otherChangesID]
		rows = append(rows, reviewRow{
			ID:       reviewItemID{GroupID: otherChangesID},
			Kind:     reviewGroupRow,
			Expanded: expanded,
		})
		if expanded {
			for _, change := range model.review.OtherChanges {
				rows = append(rows, reviewRow{
					ID:      reviewItemID{GroupID: otherChangesID, OtherPath: change.Path},
					Kind:    reviewOtherRow,
					Changed: true,
					Status:  change.Status,
				})
			}
		}
	}
	return rows
}

func (model mapTUIModel) openSelectedFile() tea.Cmd {
	if model.activeView == reviewView {
		row, exists := model.currentReviewRow()
		if !exists || row.Kind != reviewFileRow {
			return func() tea.Msg { return fileOpenedMsg{err: fmt.Errorf("select a source file first")} }
		}
	}

	node, exists := model.graph.Nodes[model.state.Selected]
	if !exists || node.Path == "" {
		return func() tea.Msg { return fileOpenedMsg{err: fmt.Errorf("no file is selected")} }
	}
	opener := model.openFile
	return func() tea.Msg { return fileOpenedMsg{err: opener(node.Path)} }
}

func openFileInVSCode(path string) error {
	return exec.Command("code", "--goto", path).Start()
}

func (model *mapTUIModel) resizeDetails() {
	contentHeight := model.mainContentHeight()
	if model.width < wideMapWidth {
		contentHeight = max(1, contentHeight/2-1)
	}
	model.details.SetHeight(contentHeight)
	model.details.SetWidth(max(1, model.detailsPaneWidth()-4))
	model.refreshDetails()
}

func (model mapTUIModel) mainContentHeight() int {
	height := model.height - 6
	if model.calendarOpen {
		height -= 10
	}
	return max(4, height)
}

func (model *mapTUIModel) refreshDetails() {
	model.details.SetContent(model.detailsContent())
}

func (model mapTUIModel) detailsPaneWidth() int {
	if model.width < wideMapWidth {
		return max(1, model.width-2)
	}
	leftWidth := model.width * 62 / 100
	return max(1, model.width-leftWidth-1)
}

func (model mapTUIModel) View() tea.View {
	view := tea.NewView(model.render())
	view.AltScreen = true
	view.WindowTitle = "gitcs review map"
	return view
}

func (model mapTUIModel) render() string {
	if model.width < minimumMapWidth || model.height < minimumMapHeight {
		return lipgloss.NewStyle().Padding(1, 2).Render(fmt.Sprintf(
			"Terminal is too small.\n\nCurrent: %dx%d\nRequired: at least %dx%d",
			model.width, model.height, minimumMapWidth, minimumMapHeight,
		))
	}

	header := titleStyle.Render("codemap") + "  " + mutedStyle.Render(fmt.Sprintf(
		"%d files · %d connections · %s",
		len(model.graph.Nodes), len(model.graph.Edges), model.root,
	))
	tabs := model.tabs()

	leftWidth := model.width - 2
	rightWidth := leftWidth
	contentHeight := model.mainContentHeight()
	if model.width >= wideMapWidth {
		leftWidth = model.width * 62 / 100
		rightWidth = model.width - leftWidth - 1
	}

	paneHeight := contentHeight
	if model.width < wideMapWidth {
		paneHeight = max(3, contentHeight/2-1)
	}

	leftContent := model.reviewContent(max(1, leftWidth-4), paneHeight)
	if model.activeView == treeView {
		leftContent = model.treeContent(max(1, leftWidth-4), paneHeight)
	}
	left := panelStyle(model.state.Focus == treePane).
		Width(max(1, leftWidth-2)).Height(paneHeight).Render(leftContent)
	details := panelStyle(model.state.Focus == detailsPane).
		Width(max(1, rightWidth-2)).Height(paneHeight).Render(model.details.View())

	content := lipgloss.JoinVertical(lipgloss.Left, left, details)
	if model.width >= wideMapWidth {
		content = lipgloss.JoinHorizontal(lipgloss.Top, left, details)
	}

	parts := []string{header, tabs, content}
	if model.calendarOpen {
		activity := panelStyle(false).
			Width(max(1, model.width-4)).
			Render(model.activityContent(max(1, model.width-8)))
		parts = append(parts, activity)
	}
	parts = append(parts, model.footer())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (model mapTUIModel) tabs() string {
	tree := mutedStyle.Render("[1] tree")
	review := mutedStyle.Render("[2] map")
	if model.activeView == treeView {
		tree = tabStyle.Render("[1] tree")
	} else {
		review = tabStyle.Render("[2] map")
	}
	mode := "reviewing working tree"
	if model.review.Clean {
		mode = "clean · architecture overview"
	}
	return tree + "  " + review + "    " + mutedStyle.Render(mode)
}

func (model mapTUIModel) reviewContent(width, height int) string {
	if model.state.Searching {
		return model.searchResultsContent(width, height)
	}
	rows := model.visibleReviewRows()
	if len(rows) == 0 {
		return mutedStyle.Render("No changed or connected source files to show.")
	}

	lines := make([]string, 0, len(rows))
	for index, row := range rows {
		line := model.reviewRowLine(row)
		if index == model.reviewIndex {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	return strings.Join(windowAroundSelection(lines, model.reviewIndex, height), "\n")
}

func (model mapTUIModel) reviewRowLine(row reviewRow) string {
	if row.Kind == reviewGroupRow {
		marker := "▸"
		if row.Expanded {
			marker = "▾"
		}
		if row.ID.GroupID == otherChangesID {
			return fmt.Sprintf("%s Other changes  %d files", marker, len(model.review.OtherChanges))
		}
		group, _ := model.reviewGroup(row.ID.GroupID)
		summary := fmt.Sprintf("%d files", len(group.Files))
		if !model.review.Clean {
			summary = fmt.Sprintf("%d changed · %d affected", len(group.ChangedFiles), len(group.NeighborFiles))
		}
		return fmt.Sprintf("%s %s  %s", marker, groupStyle.Render(group.Label), mutedStyle.Render(summary))
	}

	name := row.ID.OtherPath
	if row.ID.NodeID != "" {
		name = path.Base(string(row.ID.NodeID))
	}
	marker := "↔"
	if row.Changed {
		marker = string(row.Status)
	}
	line := fmt.Sprintf("    %s  %s", marker, name)
	if row.Changed {
		return changeStyle(row.Status).Render(line)
	}
	return mutedStyle.Render(line)
}

func (model mapTUIModel) reviewGroup(id string) (reviewGroup, bool) {
	for _, group := range model.review.Groups {
		if group.ID == id {
			return group, true
		}
	}
	return reviewGroup{}, false
}

func (model mapTUIModel) treeContent(width, height int) string {
	if model.state.Searching {
		return model.searchResultsContent(width, height)
	}
	rows := visibleTreeRows(model.state, model.forest)
	if len(rows) == 0 {
		return mutedStyle.Render("No visible source files.")
	}

	lines := make([]string, 0, len(rows)+1)
	selectedLine := -1
	insideOtherFiles := false
	for _, row := range rows {
		if row.IsOther && !insideOtherFiles {
			lines = append(lines, mutedStyle.Render("Other files"))
			insideOtherFiles = true
		}

		marker := "  "
		if len(model.forest.Children[row.NodeID]) > 0 {
			if row.Expanded {
				marker = "▾ "
			} else {
				marker = "▸ "
			}
		}
		line := strings.Repeat("  ", row.Depth) + marker + string(row.NodeID)
		if row.NodeID == model.state.Selected {
			selectedLine = len(lines)
			line = selectedStyle.Render(line)
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	return strings.Join(windowAroundSelection(lines, selectedLine, height), "\n")
}

func (model mapTUIModel) searchResultsContent(width, height int) string {
	if strings.TrimSpace(model.state.SearchQuery) == "" {
		return mutedStyle.Render("Type to search by file, path, or language.")
	}
	if len(model.state.SearchMatches) == 0 {
		return mutedStyle.Render("No matching files.")
	}

	lines := make([]string, 0, len(model.state.SearchMatches))
	for index, nodeID := range model.state.SearchMatches {
		node := model.graph.Nodes[nodeID]
		line := fmt.Sprintf("%s  %s", nodeID, mutedStyle.Render(node.Language))
		if index == model.state.SearchIndex {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	return strings.Join(windowAroundSelection(lines, model.state.SearchIndex, height), "\n")
}

func windowAroundSelection(lines []string, selectedIndex, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	if selectedIndex < 0 || selectedIndex >= len(lines) {
		selectedIndex = 0
	}
	start := selectedIndex - height/2
	if start < 0 {
		start = 0
	}
	if maximumStart := len(lines) - height; start > maximumStart {
		start = maximumStart
	}
	return lines[start : start+height]
}

func (model mapTUIModel) detailsContent() string {
	if model.activeView == reviewView {
		if row, exists := model.currentReviewRow(); exists {
			if row.Kind == reviewGroupRow {
				return model.groupDetails(row.ID.GroupID)
			}
			if row.Kind == reviewOtherRow {
				return strings.Join([]string{
					titleStyle.Render(path.Base(row.ID.OtherPath)), "",
					labelStyle.Render("Path"), row.ID.OtherPath, "",
					labelStyle.Render("Change") + "  " + string(row.Status), "",
					mutedStyle.Render("This file is not available in the analyzed source graph."),
				}, "\n")
			}
		}
	}

	node, exists := model.graph.Nodes[model.state.Selected]
	if !exists {
		return mutedStyle.Render("Select a file or group to inspect it.")
	}
	lines := []string{
		titleStyle.Render(node.Label), "",
		labelStyle.Render("Path"), string(node.ID), "",
		labelStyle.Render("Language") + "  " + node.Language,
		labelStyle.Render("Group") + "     " + path.Dir(string(node.ID)),
	}
	if status, changed := model.changes[node.ID]; changed {
		lines = append(lines, labelStyle.Render("Change")+"    "+changeStyle(status).Render(string(status)))
	}
	lines = append(lines, "", labelStyle.Render("Incoming"))
	lines = append(lines, model.connectionLines(node.ID, true)...)
	lines = append(lines, "", labelStyle.Render("Outgoing"))
	lines = append(lines, model.connectionLines(node.ID, false)...)
	return strings.Join(lines, "\n")
}

func (model mapTUIModel) groupDetails(groupID string) string {
	if groupID == otherChangesID {
		return strings.Join([]string{
			titleStyle.Render("Other changes"), "",
			fmt.Sprintf("%d deleted or unsupported files", len(model.review.OtherChanges)), "",
			mutedStyle.Render("Expand this group to inspect their paths and Git status."),
		}, "\n")
	}
	group, exists := model.reviewGroup(groupID)
	if !exists {
		return mutedStyle.Render("Select a file or group to inspect it.")
	}
	mode := "Architecture group"
	if !model.review.Clean {
		mode = "Working-tree review group"
	}
	return strings.Join([]string{
		titleStyle.Render(group.Label), "", mutedStyle.Render(mode), "",
		labelStyle.Render("Files") + fmt.Sprintf("       %d", len(group.Files)),
		labelStyle.Render("Changed") + fmt.Sprintf("     %d", len(group.ChangedFiles)),
		labelStyle.Render("Affected") + fmt.Sprintf("    %d", len(group.NeighborFiles)),
	}, "\n")
}

func (model mapTUIModel) connectionLines(nodeID NodeID, incoming bool) []string {
	var lines []string
	for _, edge := range model.graph.Edges {
		var other NodeID
		if incoming && edge.To == nodeID {
			other = edge.From
		} else if !incoming && edge.From == nodeID {
			other = edge.To
		} else {
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s  %s", other, edge.Kind))
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return []string{mutedStyle.Render("None")}
	}
	return lines
}

func (model mapTUIModel) activityContent(width int) string {
	if model.activityEmail == "" {
		return mutedStyle.Render("Contribution activity unavailable: configure Git user.email")
	}

	since := dayStart(model.activityRange.Since, model.activityRange.Until.Location())
	until := dayStart(model.activityRange.Until, model.activityRange.Until.Location())
	for since.Weekday() != 0 {
		since = since.AddDate(0, 0, -1)
	}
	for until.Weekday() != 6 {
		until = until.AddDate(0, 0, 1)
	}

	weekCount := int(until.Sub(since).Hours()/24)/7 + 1
	maxWeeks := max(1, (width-5)/2)
	if weekCount > maxWeeks {
		since = since.AddDate(0, 0, (weekCount-maxWeeks)*7)
		weekCount = maxWeeks
	}

	maximum := 0
	total := 0
	for _, count := range model.activity {
		total += count
		if count > maximum {
			maximum = count
		}
	}

	lines := []string{fmt.Sprintf("Commits over time · %s · %d commits", model.activityEmail, total)}
	for weekday := 0; weekday < 7; weekday++ {
		label := "   "
		switch weekday {
		case 1:
			label = "Mon"
		case 3:
			label = "Wed"
		case 5:
			label = "Fri"
		}
		var row strings.Builder
		row.WriteString(label + " ")
		for week := 0; week < weekCount; week++ {
			day := since.AddDate(0, 0, week*7+weekday)
			count := 0
			if !day.After(model.activityRange.Until) {
				activityUntil := dayStart(model.activityRange.Until, day.Location())
				daysBack := calendarDaysBetween(dayStart(day, day.Location()), activityUntil)
				count = model.activity[daysBack]
			}
			row.WriteString(activityCell(count, maximum))
		}
		lines = append(lines, row.String())
	}
	return strings.Join(lines, "\n")
}

func activityCell(value, maximum int) string {
	if value == 0 || maximum == 0 {
		return mutedStyle.Render("· ")
	}
	if value*4 <= maximum {
		return activityLowStyle.Render("■ ")
	}
	if value*2 <= maximum {
		return activityMediumStyle.Render("■ ")
	}
	return activityHighStyle.Render("■ ")
}

func (model mapTUIModel) footer() string {
	if model.state.Searching {
		return searchStyle.Render(fmt.Sprintf(
			"/%s  %d matches · Enter select · Esc cancel",
			model.state.SearchQuery, len(model.state.SearchMatches),
		))
	}
	help := "1/2 view · ↑/↓ navigate · ←/→ open group · Enter select · Tab pane · / search · c activity · o open · q quit"
	if model.activeView == treeView {
		help = "1/2 view · ↑/↓ navigate · ←/→ branch · Enter toggle · Tab pane · / search · a all · c activity · o open · q quit"
	}
	if model.status != "" {
		help = model.status + "  |  " + help
	}
	return mutedStyle.Render(help)
}

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#67E8F9"))
	labelStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA"))
	groupStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0"))
	mutedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	selectedStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#67E8F9"))
	searchStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#A5F3FC")).Padding(0, 1)
	tabStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#1E293B")).Padding(0, 1)
	addedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	modifiedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	deletedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185"))
	renamedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
	activityLowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#155E75"))
	activityMediumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0891B2"))
	activityHighStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE"))
)

func changeStyle(status changeStatus) lipgloss.Style {
	switch status {
	case changeAdded:
		return addedStyle
	case changeDeleted:
		return deletedStyle
	case changeRenamed:
		return renamedStyle
	default:
		return modifiedStyle
	}
}

func panelStyle(focused bool) lipgloss.Style {
	borderColor := lipgloss.Color("#263547")
	if focused {
		borderColor = lipgloss.Color("#22D3EE")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
}
