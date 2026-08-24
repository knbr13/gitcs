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
	Status           changeStatus `json:"status"`
	Additions        int          `json:"additions"`
	Deletions        int          `json:"deletions"`
	FirstChangedLine int          `json:"firstChangedLine"`
	TouchedSymbols   []string     `json:"touchedSymbols"`
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

type mapResponse struct {
	Repository   string                   `json:"repository"`
	Revision     uint64                   `json:"revision"`
	GeneratedAt  time.Time                `json:"generatedAt"`
	Clean        bool                     `json:"clean"`
	Nodes        []mapNodeResponse        `json:"nodes"`
	Edges        []mapEdgeResponse        `json:"edges"`
	OtherChanges []mapOtherChangeResponse `json:"otherChanges"`
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
	changeFacts := make(map[NodeID]mapChangeResponse, len(changes))
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
		facts := mapChangeResponse{
			Status:           change.Status,
			Additions:        diff.Additions,
			Deletions:        diff.Deletions,
			FirstChangedLine: max(1, diff.FirstChangedLine),
			TouchedSymbols:   touched,
		}
		changeFacts[id] = facts
		openTargets[id] = openTarget{
			Path:     absolutePath,
			Line:     facts.FirstChangedLine,
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
	for _, id := range nodeIDs {
		node := graph.Nodes[id]
		content, _ := os.ReadFile(node.Path)
		symbols := topLevelGoSymbols(string(content))
		var change *mapChangeResponse
		changeStatus := changeStatus("")
		var additions, deletions int
		var touched []string
		if details, exists := changeFacts[id]; exists {
			copy := details
			change = &copy
			changeStatus = details.Status
			additions = details.Additions
			deletions = details.Deletions
			touched = details.TouchedSymbols
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
			GeneratedAt:  time.Now().UTC(),
			Clean:        len(changes) == 0,
			Nodes:        nodes,
			Edges:        edges,
			OtherChanges: otherChanges,
		},
		OpenTargets: openTargets,
	}, nil
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
