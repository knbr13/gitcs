package main

import "testing"

func TestGraphAddsValidEdge(t *testing.T) {
	graph := NewGraph()

	graph.AddNode(Node{
		ID:   "server.ts",
		Kind: NodeKindFile,
	})

	graph.AddNode(Node{
		ID:   "routes/index.ts",
		Kind: NodeKindFile,
	})

	err := graph.AddEdge(Edge{
		From: "server.ts",
		To:   "routes/index.ts",
		Kind: EdgeKindImports,
	})

	if err != nil {
		t.Fatalf("expected edge to be valid, got error: %v", err)
	}

	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestGraphRejectsEdgeWithMissingNode(t *testing.T) {
	graph := NewGraph()

	graph.AddNode(Node{
		ID:   "server.ts",
		Kind: NodeKindFile,
	})

	err := graph.AddEdge(Edge{
		From: "server.ts",
		To:   "routes/index.ts",
		Kind: EdgeKindImports,
	})

	if err == nil {
		t.Fatal("expected an error for an edge with a missing target node")
	}

	if len(graph.Edges) != 0 {
		t.Fatalf("expected no edges to be added, got %d", len(graph.Edges))
	}
}
