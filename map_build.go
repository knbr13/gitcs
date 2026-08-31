package main

import (
	"fmt"
	"path/filepath"
)

func buildFileGraph(root string, files []SourceFile) (*Graph, error) {
	graph := NewGraph()

	for _, file := range files {
		relativePath, err := filepath.Rel(root, file.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"make %q relative to repo root: %w",
				file.Path,
				err,
			)
		}
		id := NodeID(filepath.ToSlash(relativePath))
		graph.AddNode(Node{
			ID:       id,
			Label:    filepath.Base(file.Path),
			Path:     file.Path,
			Language: file.Language,
			Kind:     NodeKindFile,
		})
	}
	return graph, nil
}
