package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"golang.org/x/term"
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

func runTerminalMap() error {
	path, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not find the current folder: %w", err)
	}
	// roshan: this means it will work even in the nested folders
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("the current folder is not inside a Git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("could not read the repository worktree: %w", err)
	}

	root := worktree.Filesystem.Root()
	graph, err := analyzeRepositoryGraph(root)
	if err != nil {
		return err
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return printPlainMap(os.Stdout, root, graph)
	}

	var notices []string
	changes, statusErr := readWorkingTreeChanges(worktree)
	if statusErr != nil {
		notices = append(notices, statusErr.Error())
	}
	review := buildReviewMap(graph, changes)

	boundary, _ := setTimeFlags("", "")
	email := getRepoEmailFromGit(root)
	activity := make(map[int]int)
	if email == "" {
		notices = append(notices, "Git user.email is not configured; activity is unavailable")
	} else {
		activity, err = readRepoActivity(repo, email, *boundary)
		if err != nil {
			notices = append(notices, "Could not read repository activity: "+err.Error())
			activity = make(map[int]int)
		}
	}

	return runMapTUI(
		root,
		graph,
		review,
		changes,
		activity,
		*boundary,
		email,
		strings.Join(notices, " · "),
	)
}
