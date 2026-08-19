package main

import "fmt"

type Analyzer interface {
	CanAnalyze(language string) bool

	FindConnections(
		root string,
		file SourceFile,
		graph *Graph,
	) ([]Edge, error)
}

func applyAnalyzers(root string, files []SourceFile, graph *Graph, analyzers []Analyzer) error {
	for _, file := range files {
		for _, analyzer := range analyzers {
			if !analyzer.CanAnalyze(file.Language) {
				continue
			}

			edges, err := analyzer.FindConnections(root, file, graph)
			if err != nil {
				return fmt.Errorf("analyze %q: %w", file.Path, err)
			}

			for _, edge := range edges {
				if err := graph.AddEdge(edge); err != nil {
					return fmt.Errorf("add connection from %q to %q: %w", edge.From, edge.To, err)
				}
			}
		}
	}

	return nil
}
