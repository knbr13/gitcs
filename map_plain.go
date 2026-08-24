package main

import (
	"fmt"
	"io"
	"strings"
)

func printPlainMap(writer io.Writer, root string, graph *Graph) error {
	forest := buildRootedForest(graph)
	state := initialMapState(forest)
	state.ShowAll = true
	for nodeID := range forest.Children {
		state.Expanded[nodeID] = true
	}

	fmt.Fprintf(writer, "gitcs project map: %s\n", root)
	fmt.Fprintf(writer, "%d files, %d connections\n\n", len(graph.Nodes), len(graph.Edges))

	rows := visibleTreeRows(state, forest)
	if len(rows) == 0 {
		fmt.Fprintln(writer, "No supported source files found.")
		return nil
	}

	insideOtherFiles := false
	for _, row := range rows {
		if row.IsOther && !insideOtherFiles {
			fmt.Fprintln(writer, "\nOther files:")
			insideOtherFiles = true
		}
		fmt.Fprintf(writer, "%s- %s\n", strings.Repeat("  ", row.Depth), row.NodeID)
	}
	return nil
}
