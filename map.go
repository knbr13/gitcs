package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
)

// readWorkingTreeChanges keeps go-git's two-column status representation out
// of the pure review-map transformation. A worktree change takes precedence
// over a staged change because it describes the file currently on disk.
func readWorkingTreeChanges(worktree *git.Worktree) ([]reviewChange, error) {
	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("read Git working-tree status: %w", err)
	}

	paths := make([]string, 0, len(status))
	for path := range status {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	changes := make([]reviewChange, 0, len(paths))
	for _, path := range paths {
		fileStatus := status[path]
		code := fileStatus.Worktree
		if code == git.Unmodified {
			code = fileStatus.Staging
		}

		mapped, changed := mapGitStatus(code)
		if !changed {
			continue
		}

		changes = append(changes, reviewChange{
			Path:   filepath.ToSlash(filepath.Clean(path)),
			Status: mapped,
		})
	}

	return changes, nil
}

func mapGitStatus(code git.StatusCode) (changeStatus, bool) {
	switch code {
	case git.Untracked, git.Added:
		return changeAdded, true
	case git.Modified, git.Copied, git.UpdatedButUnmerged:
		return changeModified, true
	case git.Deleted:
		return changeDeleted, true
	case git.Renamed:
		return changeRenamed, true
	default:
		return "", false
	}
}

func analyzeRepositoryGraph(root string) (*Graph, error) {
	files, err := findSourceFiles(root)
	if err != nil {
		return nil, fmt.Errorf("could not scan the repository: %w", err)
	}
	graph, err := buildFileGraph(root, files)
	if err != nil {
		return nil, fmt.Errorf("could not build the project graph: %w", err)
	}
	goAnalyzer, err := NewGoAnalyzer(graph)
	if err != nil {
		return nil, fmt.Errorf("could not prepare the Go analyzer: %w", err)
	}
	if err := applyAnalyzers(root, files, graph, []Analyzer{goAnalyzer}); err != nil {
		return nil, fmt.Errorf("could not analyze project connections: %w", err)
	}
	return graph, nil
}
