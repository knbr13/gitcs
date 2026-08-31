package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

type GoAnalyzer struct {
	functionIndex GoFunctionIndex
}

func NewGoAnalyzer(graph *Graph) (*GoAnalyzer, error) {
	index, err := buildGoFunctionIndex(graph)
	if err != nil {
		return nil, err
	}
	for _, nodeID := range index["main"] {
		node := graph.Nodes[nodeID]
		node.IsRoot = true
		graph.Nodes[nodeID] = node
	}

	return &GoAnalyzer{functionIndex: index}, nil
}

func (analyzer *GoAnalyzer) CanAnalyze(language string) bool {
	return language == "Go"
}

func (analyzer *GoAnalyzer) FindConnections(
	root string,
	file SourceFile,
	graph *Graph,
) ([]Edge, error) {
	relativePath, err := filepath.Rel(root, file.Path)
	if err != nil {
		return nil, fmt.Errorf("make source path relative: %w", err)
	}

	from := NodeID(filepath.ToSlash(relativePath))
	if _, exists := graph.Nodes[from]; !exists {
		return nil, fmt.Errorf("source node %q does not exist", from)
	}

	calls, err := findGoCalls(file.Path)
	if err != nil {
		return nil, err
	}

	seenTargets := make(map[NodeID]struct{})
	var edges []Edge

	for _, call := range calls {
		targets := analyzer.functionIndex[call]
		if len(targets) != 1 {
			continue
		}

		to := targets[0]
		if to == from {
			continue
		}
		if _, seen := seenTargets[to]; seen {
			continue
		}

		seenTargets[to] = struct{}{}
		edges = append(edges, Edge{From: from, To: to, Kind: EdgeKindCalls})
	}

	return edges, nil
}

func findGoFunctions(path string) ([]string, error) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var functions []string

	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)

		if !isFunction {
			continue
		}

		if function.Recv != nil {
			continue
		}
		functions = append(functions, function.Name.Name)

	}
	return functions, nil

}

func findGoCalls(path string) ([]string, error) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var calls []string

	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		function, isSimpleFunction := call.Fun.(*ast.Ident)
		if !isSimpleFunction {
			return true
		}

		calls = append(calls, function.Name)
		return true
	})

	return calls, nil
}

type GoFunctionIndex map[string][]NodeID

func buildGoFunctionIndex(graph *Graph) (GoFunctionIndex, error) {
	index := make(GoFunctionIndex)

	for nodeID, node := range graph.Nodes {
		if node.Kind != NodeKindFile || node.Language != "Go" {
			continue
		}

		functions, err := findGoFunctions(node.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"find functions in %q: %w",
				node.Path,
				err,
			)
		}

		for _, function := range functions {
			index[function] = append(index[function], nodeID)
		}
	}

	return index, nil
}
