package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type mapChangeResponse struct {
	Status           changeStatus  `json:"status"`
	Additions        int           `json:"additions"`
	Deletions        int           `json:"deletions"`
	FirstChangedLine int           `json:"firstChangedLine"`
	TouchedSymbols   []string      `json:"touchedSymbols"`
	Summary          changeSummary `json:"summary"`
}

type changeSummary struct {
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Changed  string `json:"changed"`
	Impact   string `json:"impact"`
}

type mapNodeResponse struct {
	ID          NodeID             `json:"id"`
	Label       string             `json:"label"`
	Language    string             `json:"language"`
	Kind        NodeKind           `json:"kind"`
	Description string             `json:"description"`
	IsRoot      bool               `json:"isRoot,omitempty"`
	Affected    bool               `json:"affected,omitempty"`
	Openable    bool               `json:"openable"`
	Activity    mapFileActivity    `json:"activity"`
	Change      *mapChangeResponse `json:"change,omitempty"`
}

type mapEdgeResponse struct {
	From NodeID   `json:"from"`
	To   NodeID   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

type mapOtherChangeResponse struct {
	ID       NodeID       `json:"id"`
	Label    string       `json:"label"`
	Status   changeStatus `json:"status"`
	Openable bool         `json:"openable"`
}

type mapCommitResponse struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	When    time.Time `json:"when"`
}

type mapFileActivity struct {
	Commits30     int                 `json:"commits30"`
	Commits90     int                 `json:"commits90"`
	CommitsAll    int                 `json:"commitsAll"`
	People        int                 `json:"people"`
	LastChangedAt *time.Time          `json:"lastChangedAt,omitempty"`
	RecentCommits []mapCommitResponse `json:"recentCommits"`
}

type mapActivityBucket struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Count int       `json:"count"`
}

type mapResponse struct {
	Repository   string                   `json:"repository"`
	Branch       string                   `json:"branch"`
	Revision     uint64                   `json:"revision"`
	GeneratedAt  time.Time                `json:"generatedAt"`
	Clean        bool                     `json:"clean"`
	Nodes        []mapNodeResponse        `json:"nodes"`
	Edges        []mapEdgeResponse        `json:"edges"`
	OtherChanges []mapOtherChangeResponse `json:"otherChanges"`
	Activity     []mapActivityBucket      `json:"activity"`
}

// fileDescriptionFacts is the UI-independent input to the learning exercise
// in map_description.go. Every field is derived from source or Git evidence.
type fileDescriptionFacts struct {
	Label          string
	Language       string
	IsRoot         bool
	Symbols        []string
	IncomingCount  int
	OutgoingCount  int
	ChangeStatus   changeStatus
	Additions      int
	Deletions      int
	TouchedSymbols []string
}

type openTarget struct {
	Path     string
	Line     int
	Openable bool
}

type mapSnapshot struct {
	Response    mapResponse
	OpenTargets map[NodeID]openTarget
}

type lineRange struct {
	Start int
	End   int
}

type fileDiffFacts struct {
	Additions        int
	Deletions        int
	FirstChangedLine int
	OldRanges        []lineRange
	NewRanges        []lineRange
}

type changeEvidence struct {
	Response          mapChangeResponse
	PreviousSymbols   []string
	CurrentSymbols    []string
	PreviousLineCount int
	CurrentLineCount  int
}

func buildMapSnapshot(root string, repo *git.Repository, worktree *git.Worktree) (mapSnapshot, error) {
	graph, err := analyzeRepositoryGraph(root)
	if err != nil {
		return mapSnapshot{}, err
	}

	changes, err := readWorkingTreeChanges(worktree)
	if err != nil {
		return mapSnapshot{}, err
	}

	headTree := readHeadTree(repo)
	changeByID := make(map[NodeID]reviewChange, len(changes))
	changeFacts := make(map[NodeID]changeEvidence, len(changes))
	openTargets := make(map[NodeID]openTarget, len(graph.Nodes)+len(changes))

	for _, change := range changes {
		id := NodeID(filepath.ToSlash(filepath.Clean(change.Path)))
		changeByID[id] = change
		absolutePath := filepath.Join(root, filepath.FromSlash(string(id)))
		openable := change.Status != changeDeleted
		openTargets[id] = openTarget{Path: absolutePath, Line: 1, Openable: openable}

		if _, analyzed := graph.Nodes[id]; !analyzed {
			continue
		}
		oldContent := readTreeFile(headTree, string(id))
		newContent := readWorktreeFile(absolutePath, change.Status)
		diff := diffFileLines(oldContent, newContent)
		touched := touchedGoSymbols(oldContent, diff.OldRanges, newContent, diff.NewRanges)
		facts := changeEvidence{
			Response: mapChangeResponse{
				Status:           change.Status,
				Additions:        diff.Additions,
				Deletions:        diff.Deletions,
				FirstChangedLine: max(1, diff.FirstChangedLine),
				TouchedSymbols:   touched,
			},
			PreviousSymbols:   topLevelGoSymbols(oldContent),
			CurrentSymbols:    topLevelGoSymbols(newContent),
			PreviousLineCount: contentLineCount(oldContent),
			CurrentLineCount:  contentLineCount(newContent),
		}
		changeFacts[id] = facts
		openTargets[id] = openTarget{
			Path:     absolutePath,
			Line:     facts.Response.FirstChangedLine,
			Openable: openable,
		}
	}

	changedIDs := make(map[NodeID]bool, len(changeByID))
	for id := range changeByID {
		if _, exists := graph.Nodes[id]; exists {
			changedIDs[id] = true
		}
	}
	affectedIDs := make(map[NodeID]bool)
	for _, edge := range graph.Edges {
		if changedIDs[edge.From] && !changedIDs[edge.To] {
			affectedIDs[edge.To] = true
		}
		if changedIDs[edge.To] && !changedIDs[edge.From] {
			affectedIDs[edge.From] = true
		}
	}

	incoming := make(map[NodeID]int)
	outgoing := make(map[NodeID]int)
	edges := make([]mapEdgeResponse, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		incoming[edge.To]++
		outgoing[edge.From]++
		edges = append(edges, mapEdgeResponse(edge))
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	nodeIDs := make([]NodeID, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })

	nodes := make([]mapNodeResponse, 0, len(nodeIDs))
	generatedAt := time.Now().UTC()
	for _, id := range nodeIDs {
		node := graph.Nodes[id]
		content, _ := os.ReadFile(node.Path)
		symbols := topLevelGoSymbols(string(content))
		var change *mapChangeResponse
		changeStatus := changeStatus("")
		var additions, deletions int
		var touched []string
		if details, exists := changeFacts[id]; exists {
			copy := details.Response
			copy.Summary = summarizeChange(changeSummaryFacts{
				Label:             node.Label,
				Language:          node.Language,
				Status:            details.Response.Status,
				Additions:         details.Response.Additions,
				Deletions:         details.Response.Deletions,
				FirstChangedLine:  details.Response.FirstChangedLine,
				TouchedSymbols:    details.Response.TouchedSymbols,
				PreviousSymbols:   details.PreviousSymbols,
				CurrentSymbols:    details.CurrentSymbols,
				PreviousLineCount: details.PreviousLineCount,
				CurrentLineCount:  details.CurrentLineCount,
				IncomingCount:     incoming[id],
				OutgoingCount:     outgoing[id],
				Neighbors:         connectedFileLabels(id, graph),
			})
			change = &copy
			changeStatus = details.Response.Status
			additions = details.Response.Additions
			deletions = details.Response.Deletions
			touched = details.Response.TouchedSymbols
		}
		description := describeFile(fileDescriptionFacts{
			Label:          node.Label,
			Language:       node.Language,
			IsRoot:         node.IsRoot,
			Symbols:        symbols,
			IncomingCount:  incoming[id],
			OutgoingCount:  outgoing[id],
			ChangeStatus:   changeStatus,
			Additions:      additions,
			Deletions:      deletions,
			TouchedSymbols: touched,
		})
		nodes = append(nodes, mapNodeResponse{
			ID:          id,
			Label:       node.Label,
			Language:    node.Language,
			Kind:        node.Kind,
			Description: description,
			IsRoot:      node.IsRoot,
			Affected:    affectedIDs[id],
			Openable:    true,
			Activity:    readMapFileActivity(repo, string(id), generatedAt),
			Change:      change,
		})
		if _, exists := openTargets[id]; !exists {
			openTargets[id] = openTarget{Path: node.Path, Line: 1, Openable: true}
		}
	}

	otherChanges := make([]mapOtherChangeResponse, 0)
	for _, change := range changes {
		id := NodeID(filepath.ToSlash(filepath.Clean(change.Path)))
		if _, analyzed := graph.Nodes[id]; analyzed {
			continue
		}
		if !isMeaningfulOtherChange(string(id)) {
			continue
		}
		otherChanges = append(otherChanges, mapOtherChangeResponse{
			ID:       id,
			Label:    filepath.Base(string(id)),
			Status:   change.Status,
			Openable: change.Status != changeDeleted,
		})
	}

	return mapSnapshot{
		Response: mapResponse{
			Repository:   filepath.Base(root),
			Branch:       readBranchName(repo),
			GeneratedAt:  generatedAt,
			Clean:        len(changes) == 0,
			Nodes:        nodes,
			Edges:        edges,
			OtherChanges: otherChanges,
			Activity:     readMapActivityBuckets(repo, generatedAt, 24),
		},
		OpenTargets: openTargets,
	}, nil
}

func isMeaningfulOtherChange(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	base := strings.ToLower(filepath.Base(clean))
	if base == "." || strings.HasPrefix(base, ".") {
		return false
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" {
			continue
		}
		if _, excluded := excludedFolders[strings.ToLower(segment)]; excluded {
			return false
		}
	}
	if _, source := languagesByExtension[strings.ToLower(filepath.Ext(clean))]; source {
		return true
	}
	switch base {
	case "readme.md", "architecture.md", "contributing.md", "makefile", "go.mod", "package.json", "pyproject.toml", "cargo.toml", "pom.xml", "build.gradle":
		return true
	}
	return false
}

func connectedFileLabels(id NodeID, graph *Graph) []string {
	if graph == nil {
		return nil
	}
	var labels []string
	for _, edge := range graph.Edges {
		switch {
		case edge.From == id:
			if node, exists := graph.Nodes[edge.To]; exists {
				labels = append(labels, node.Label)
			}
		case edge.To == id:
			if node, exists := graph.Nodes[edge.From]; exists {
				labels = append(labels, node.Label)
			}
		}
	}
	return uniqueSortedStrings(labels)
}

func readMapFileActivity(repo *git.Repository, path string, now time.Time) mapFileActivity {
	if repo == nil || path == "" {
		return mapFileActivity{}
	}
	path = filepath.ToSlash(filepath.Clean(path))
	iterator, err := repo.Log(&git.LogOptions{
		FileName: &path,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return mapFileActivity{}
	}
	defer iterator.Close()

	since30 := now.AddDate(0, 0, -30)
	since90 := now.AddDate(0, 0, -90)
	people := make(map[string]bool)
	activity := mapFileActivity{}
	err = iterator.ForEach(func(commit *object.Commit) error {
		activity.CommitsAll++
		if !commit.Author.When.Before(since90) {
			activity.Commits90++
		}
		if !commit.Author.When.Before(since30) {
			activity.Commits30++
		}
		if commit.Author.Email != "" {
			people[commit.Author.Email] = true
		} else if commit.Author.Name != "" {
			people[commit.Author.Name] = true
		}
		if activity.LastChangedAt == nil || commit.Author.When.After(*activity.LastChangedAt) {
			when := commit.Author.When
			activity.LastChangedAt = &when
		}
		if len(activity.RecentCommits) < 3 {
			activity.RecentCommits = append(activity.RecentCommits, mapCommitResponse{
				Hash:    shortHash(commit.Hash.String()),
				Message: firstCommitLine(commit.Message),
				Author:  commit.Author.Name,
				When:    commit.Author.When,
			})
		}
		return nil
	})
	if err != nil {
		return mapFileActivity{}
	}
	activity.People = len(people)
	return activity
}

func readMapActivityBuckets(repo *git.Repository, now time.Time, weeks int) []mapActivityBucket {
	if repo == nil || weeks <= 0 {
		return nil
	}
	end := dayStart(now, now.Location())
	start := end.AddDate(0, 0, -(weeks*7 - 1))
	buckets := make([]mapActivityBucket, 0, weeks)
	for index := 0; index < weeks; index++ {
		bucketStart := start.AddDate(0, 0, index*7)
		buckets = append(buckets, mapActivityBucket{
			Start: bucketStart,
			End:   bucketStart.AddDate(0, 0, 6),
		})
	}

	iterator, err := repo.Log(&git.LogOptions{
		Since: &start,
		Until: &now,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return buckets
	}
	defer iterator.Close()

	_ = iterator.ForEach(func(commit *object.Commit) error {
		if commit.Author.When.Before(start) || commit.Author.When.After(now) {
			return nil
		}
		index := int(commit.Author.When.Sub(start).Hours() / 24 / 7)
		if index >= 0 && index < len(buckets) {
			buckets[index].Count++
		}
		return nil
	})
	return buckets
}

func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}

func firstCommitLine(message string) string {
	line := strings.TrimSpace(strings.Split(message, "\n")[0])
	if line == "" {
		return "Commit"
	}
	return line
}

func readBranchName(repo *git.Repository) string {
	if repo == nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	if head.Name().IsBranch() {
		return head.Name().Short()
	}
	return shortHash(head.Hash().String())
}

func readHeadTree(repo *git.Repository) *object.Tree {
	reference, err := repo.Head()
	if err != nil {
		return nil
	}
	commit, err := repo.CommitObject(reference.Hash())
	if err != nil {
		return nil
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil
	}
	return tree
}

func readTreeFile(tree *object.Tree, path string) string {
	if tree == nil {
		return ""
	}
	file, err := tree.File(filepath.ToSlash(path))
	if err != nil {
		return ""
	}
	content, err := file.Contents()
	if err != nil {
		return ""
	}
	return content
}

func readWorktreeFile(path string, status changeStatus) string {
	if status == changeDeleted {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func diffFileLines(oldContent, newContent string) fileDiffFacts {
	matcher := diffmatchpatch.New()
	oldRunes, newRunes, lines := matcher.DiffLinesToRunes(oldContent, newContent)
	diffs := matcher.DiffMainRunes(oldRunes, newRunes, false)
	diffs = matcher.DiffCharsToLines(diffs, lines)

	result := fileDiffFacts{}
	oldLine, newLine := 1, 1
	for _, diff := range diffs {
		count := countDiffLines(diff.Text)
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			oldLine += count
			newLine += count
		case diffmatchpatch.DiffDelete:
			result.Deletions += count
			result.OldRanges = appendRange(result.OldRanges, oldLine, count)
			if result.FirstChangedLine == 0 {
				result.FirstChangedLine = newLine
			}
			oldLine += count
		case diffmatchpatch.DiffInsert:
			result.Additions += count
			result.NewRanges = appendRange(result.NewRanges, newLine, count)
			if result.FirstChangedLine == 0 {
				result.FirstChangedLine = newLine
			}
			newLine += count
		}
	}
	return result
}

func countDiffLines(text string) int {
	if text == "" {
		return 0
	}
	count := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		count++
	}
	return count
}

func appendRange(ranges []lineRange, start, count int) []lineRange {
	if count == 0 {
		return ranges
	}
	return append(ranges, lineRange{Start: start, End: start + count - 1})
}

type sourceSymbol struct {
	Name       string
	Start, End int
}

func parseGoSymbols(content string) []sourceSymbol {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "source.go", content, 0)
	if err != nil {
		return nil
	}
	var symbols []sourceSymbol
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			symbols = append(symbols, sourceSymbol{
				Name:  declaration.Name.Name,
				Start: fileSet.Position(declaration.Pos()).Line,
				End:   fileSet.Position(declaration.End()).Line,
			})
		case *ast.GenDecl:
			for _, specification := range declaration.Specs {
				var name string
				switch specification := specification.(type) {
				case *ast.TypeSpec:
					name = specification.Name.Name
				case *ast.ValueSpec:
					if len(specification.Names) > 0 {
						name = specification.Names[0].Name
					}
				}
				if name != "" {
					symbols = append(symbols, sourceSymbol{
						Name:  name,
						Start: fileSet.Position(specification.Pos()).Line,
						End:   fileSet.Position(specification.End()).Line,
					})
				}
			}
		}
	}
	return symbols
}

func topLevelGoSymbols(content string) []string {
	symbols := parseGoSymbols(content)
	names := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		names = append(names, symbol.Name)
	}
	return uniqueSortedStrings(names)
}

func touchedGoSymbols(oldContent string, oldRanges []lineRange, newContent string, newRanges []lineRange) []string {
	var names []string
	for _, candidate := range []struct {
		content string
		ranges  []lineRange
	}{{oldContent, oldRanges}, {newContent, newRanges}} {
		for _, symbol := range parseGoSymbols(candidate.content) {
			for _, changed := range candidate.ranges {
				if symbol.Start <= changed.End && changed.Start <= symbol.End {
					names = append(names, symbol.Name)
					break
				}
			}
		}
	}
	return uniqueSortedStrings(names)
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validateOpenTarget(root string, target openTarget) error {
	if !target.Openable {
		return fmt.Errorf("file is deleted and cannot be opened")
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetPath, err := filepath.Abs(target.Path)
	if err != nil {
		return err
	}
	if resolvedRoot, resolveErr := filepath.EvalSymlinks(rootPath); resolveErr == nil {
		rootPath = resolvedRoot
	}
	resolvedTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return fmt.Errorf("file is unavailable: %w", err)
	}
	targetPath = resolvedTarget
	relative, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("file is outside the repository")
	}
	return nil
}
