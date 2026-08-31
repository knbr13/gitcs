package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
)

func newMapWebServerForDirectory(directory string, opener func(string, int) error) (*mapWebServer, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	repo, err := git.PlainOpenWithOptions(root, &git.PlainOpenOptions{DetectDotGit: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return newMapWebServer(root, nil, nil, opener)
	}
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	return newMapWebServer(worktree.Filesystem.Root(), repo, worktree, opener)
}

// A nil repository means root is a workspace containing independent repositories.
func buildDirectoryMapSnapshot(root string, repo *git.Repository, worktree *git.Worktree) (mapSnapshot, error) {
	snapshot, err := buildDirectoryFileSnapshot(root, repo, worktree)
	if err != nil {
		return mapSnapshot{}, err
	}
	if err := addMapArchitecture(root, &snapshot); err != nil {
		return mapSnapshot{}, fmt.Errorf("build architecture: %w", err)
	}
	return snapshot, nil
}

func buildDirectoryFileSnapshot(root string, repo *git.Repository, worktree *git.Worktree) (mapSnapshot, error) {
	if repo != nil {
		return buildMapSnapshot(root, repo, worktree)
	}
	roots, err := discoverMapRepositories(root)
	if err != nil {
		return mapSnapshot{}, err
	}
	if len(roots) == 0 {
		return mapSnapshot{}, fmt.Errorf("no Git repositories found inside %s", root)
	}
	merged := mapSnapshot{
		Response: mapResponse{
			Repository: filepath.Base(root), GeneratedAt: time.Now().UTC(), Clean: true,
			Nodes: []mapNodeResponse{}, Edges: []mapEdgeResponse{},
			OtherChanges: []mapOtherChangeResponse{}, Activity: []mapActivityBucket{},
		},
		OpenTargets: make(map[NodeID]openTarget),
	}
	branches := make([]string, 0, len(roots))
	for _, repoRoot := range roots {
		repository, err := git.PlainOpen(repoRoot)
		if err != nil {
			return mapSnapshot{}, fmt.Errorf("open %s: %w", repoRoot, err)
		}
		worktree, err := repository.Worktree()
		if err != nil {
			return mapSnapshot{}, err
		}
		snapshot, err := buildMapSnapshot(repoRoot, repository, worktree)
		if err != nil {
			return mapSnapshot{}, fmt.Errorf("map %s: %w", repoRoot, err)
		}
		relative, err := filepath.Rel(root, repoRoot)
		if err != nil {
			return mapSnapshot{}, err
		}
		prefix := filepath.ToSlash(relative) + "/"
		branches = append(branches, filepath.ToSlash(relative)+": "+snapshot.Response.Branch)
		merged.Response.Clean = merged.Response.Clean && snapshot.Response.Clean
		for _, node := range snapshot.Response.Nodes {
			node.ID = NodeID(prefix + string(node.ID))
			// Co-change only compares commits within the same repository.
			for i, hash := range node.Activity.CommitHashes {
				node.Activity.CommitHashes[i] = prefix + hash
			}
			merged.Response.Nodes = append(merged.Response.Nodes, node)
		}
		for _, edge := range snapshot.Response.Edges {
			edge.From = NodeID(prefix + string(edge.From))
			edge.To = NodeID(prefix + string(edge.To))
			merged.Response.Edges = append(merged.Response.Edges, edge)
		}
		for _, change := range snapshot.Response.OtherChanges {
			change.ID = NodeID(prefix + string(change.ID))
			merged.Response.OtherChanges = append(merged.Response.OtherChanges, change)
		}
		for id, target := range snapshot.OpenTargets {
			merged.OpenTargets[NodeID(prefix+string(id))] = target
		}
		for _, bucket := range snapshot.Response.Activity {
			found := false
			for i := range merged.Response.Activity {
				if merged.Response.Activity[i].Start.Equal(bucket.Start) {
					merged.Response.Activity[i].Count += bucket.Count
					found = true
					break
				}
			}
			if !found {
				merged.Response.Activity = append(merged.Response.Activity, bucket)
			}
		}
	}
	merged.Response.Branch = strings.Join(branches, " · ")
	return merged, nil
}

// Stop at repository boundaries so source files are never mapped twice.
// Both .git directories and linked-worktree .git files are supported.
func discoverMapRepositories(root string) ([]string, error) {
	var roots []string
	err := filepath.WalkDir(root, func(candidate string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if candidate != root {
			if _, excluded := excludedFolders[strings.ToLower(entry.Name())]; excluded || entry.Name() == ".git" {
				return filepath.SkipDir
			}
		}
		if _, err := os.Lstat(filepath.Join(candidate, ".git")); err == nil {
			roots = append(roots, candidate)
			return filepath.SkipDir
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	return roots, err
}
