package main

import (
	"os"
	"strings"
	"testing"
)

func TestWriteGraphHTMLIncludesGraphData(t *testing.T) {
	graph := NewGraph()
	graph.AddNode(Node{
		ID:       "server.go",
		Label:    "server.go",
		Path:     "/project/server.go",
		Language: "Go",
		Kind:     NodeKindFile,
		IsRoot:   true,
	})
	graph.AddNode(Node{
		ID:       "routes.go",
		Label:    "routes.go",
		Path:     "/project/routes.go",
		Language: "Go",
		Kind:     NodeKindFile,
	})
	if err := graph.AddEdge(Edge{From: "server.go", To: "routes.go", Kind: EdgeKindCalls}); err != nil {
		t.Fatalf("could not prepare graph: %v", err)
	}

	path, err := writeGraphHTML(graph)
	if err != nil {
		t.Fatalf("could not create graph HTML: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read graph HTML: %v", err)
	}
	page := string(contents)

	for _, expected := range []string{
		`"id":"server.go"`,
		`"isRoot":true`,
		`"path":"/project/routes.go"`,
		`"from":"server.go"`,
		`"to":"routes.go"`,
		`vscode://file/`,
		`pointerdown`,
		`updateEdgePaths`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected generated page to contain %q", expected)
		}
	}
}
