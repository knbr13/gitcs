package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestWorkspaceMapCombinesRepositoriesAndRebuilds(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"frontend", "backend"} {
		directory := filepath.Join(root, name)
		repo, err := git.PlainInit(directory, false)
		if err != nil {
			t.Fatal(err)
		}
		for file, content := range map[string]string{
			"main.js":      "import './helper.js';\n",
			"helper.js":    "export const value = 1;\n",
			"package.json": "{}\n",
		} {
			if err := os.WriteFile(filepath.Join(directory, file), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		worktree, err := repo.Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := worktree.Commit("initial", &git.CommitOptions{Author: &object.Signature{
			Name: "Test", Email: "test@example.com", When: time.Now(),
		}}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "helper.js"), []byte("export const value = 2;\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(directory, "package.json")); err != nil {
			t.Fatal(err)
		}
	}
	server, err := newMapWebServerForDirectory(root, func(string, int) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	response := server.snapshot.Response
	if len(response.Nodes) != 4 || len(response.Edges) != 2 || response.Clean || len(response.OtherChanges) != 2 {
		t.Fatalf("unexpected combined graph: %#v", response)
	}
	for _, name := range []string{"frontend", "backend"} {
		id := NodeID(name + "/main.js")
		target, exists := server.snapshot.OpenTargets[id]
		if !exists || target.Path != filepath.Join(root, name, "main.js") {
			t.Fatalf("incorrect open target for %s: %#v", id, target)
		}
	}
	for _, edge := range response.Edges {
		if strings.Split(string(edge.From), "/")[0] != strings.Split(string(edge.To), "/")[0] {
			t.Fatalf("cross-repository edge: %#v", edge)
		}
	}
	for _, node := range response.Nodes {
		if node.Activity.CommitsAll != 1 || len(node.Activity.CommitHashes) != 1 || !strings.HasPrefix(node.Activity.CommitHashes[0], strings.Split(string(node.ID), "/")[0]+"/") {
			t.Fatalf("history not isolated for %s: %#v", node.ID, node.Activity)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "new.js"), []byte("export const added = true;"), 0600); err != nil {
		t.Fatal(err)
	}
	server.rebuild()
	if server.snapshot.Response.Revision != 2 || len(server.snapshot.Response.Nodes) != 5 {
		t.Fatalf("rebuild did not update workspace: %#v", server.snapshot.Response)
	}
	if len(server.snapshot.Response.Projects) != 2 {
		t.Fatal("workspace repositories lost their project boundaries")
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "new.spec.ts"), []byte("import './new.js';"), 0600); err != nil {
		t.Fatal(err)
	}
	server.rebuild()
	foundTests := false
	for _, group := range server.snapshot.Response.Architecture.Modules {
		if group.ProjectID == "project:backend" && group.IsTest {
			foundTests = group.FileCount == 1
		}
	}
	if !foundTests {
		t.Fatal("new Tests group missing after rebuild")
	}
	if err := os.Remove(filepath.Join(root, "backend", "new.spec.ts")); err != nil {
		t.Fatal(err)
	}
	server.rebuild()
	for _, group := range server.snapshot.Response.Architecture.Modules {
		if group.IsTest {
			t.Fatal("deleted Tests group survived rebuild")
		}
	}
	// Starting inside an individual repository preserves its existing IDs.
	single, err := newMapWebServerForDirectory(filepath.Join(root, "frontend"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := single.snapshot.OpenTargets["main.js"]; !exists {
		t.Fatal("single repository IDs changed")
	}
}

func TestWorkspaceDiscoverySkipsDependenciesAndRejectsEmptyFolder(t *testing.T) {
	root := t.TempDir()
	if _, err := git.PlainInit(filepath.Join(root, "node_modules", "dependency"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := newMapWebServerForDirectory(root, nil); err == nil || !strings.Contains(err.Error(), "no Git repositories") {
		t.Fatalf("expected no repositories, got %v", err)
	}
	// Discovery recognizes a linked worktree's .git file as a boundary.
	directory := filepath.Join(root, "linked")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".git"), []byte("gitdir: elsewhere"), 0600); err != nil {
		t.Fatal(err)
	}
	roots, err := discoverMapRepositories(root)
	if err != nil || len(roots) != 1 || roots[0] != directory {
		t.Fatalf("discovery = %v, %v", roots, err)
	}
}
