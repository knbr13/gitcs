package main

import "fmt"

type NodeID string

type NodeKind string

const (
	NodeKindFile     NodeKind = "file"
	NodeKindFolder   NodeKind = "folder"
	NodeKindFunction NodeKind = "function"
)

type EdgeKind string

const (
	EdgeKindImports  EdgeKind = "imports"
	EdgeKindCalls    EdgeKind = "calls"
	EdgeKindContains EdgeKind = "contains"
)

type Node struct {
	ID       NodeID   `json:"id"`
	Label    string   `json:"label"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Kind     NodeKind `json:"kind"`
	IsRoot   bool     `json:"isRoot,omitempty"`
}

type Edge struct {
	From NodeID   `json:"from"`
	To   NodeID   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

type Graph struct {
	Nodes map[NodeID]Node
	Edges []Edge
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[NodeID]Node),
		Edges: make([]Edge, 0),
	}
}

func (g *Graph) AddNode(node Node) {
	g.Nodes[node.ID] = node
}

func (g *Graph) AddEdge(edge Edge) error {
	if _, exists := g.Nodes[edge.From]; !exists {
		return fmt.Errorf("source node %q does not exist", edge.From)
	}
	if _, exists := g.Nodes[edge.To]; !exists {
		return fmt.Errorf("target node %q does not exist", edge.To)
	}
	g.Edges = append(g.Edges, edge)
	return nil
}
