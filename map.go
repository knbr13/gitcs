package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"os"
)

func runMap() {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println("gitcs map: could not find the current folder")
		return
	}
	// roshan: this means it will work even in the nested folders
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		fmt.Println("gitcs map: the current folder is not inside a Git repository")
		return
	}

	worktree, err := repo.Worktree()
	if err != nil {
		fmt.Println("gitcs map: could not read the repository worktree")
		return
	}

	root := worktree.Filesystem.Root()
	files, err := findSourceFiles(root)
	if err != nil {
		fmt.Println("gitcs map: could not scan the repository")
		return
	}
	graph, err := buildFileGraph(root, files)
	if err != nil {
		fmt.Println("gitcs map: could not build the project graph")
		return
	}
	goAnalyzer, err := NewGoAnalyzer(graph)
	if err != nil {
		fmt.Println("gitcs map: could not prepare the Go analyzer")
		return
	}
	if err := applyAnalyzers(root, files, graph, []Analyzer{goAnalyzer}); err != nil {
		fmt.Println("gitcs map: could not analyze project connections")
		return
	}

	fmt.Printf("Map mode: scanning %s\n", root)
	fmt.Printf("Built graph with %d cards and %d connections\n", len(graph.Nodes), len(graph.Edges))

	reportPath, err := writeGraphHTML(graph)
	if err != nil {
		fmt.Println("gitcs map: could not create the browser view")
		return
	}

	if err := openGraphInBrowser(reportPath); err != nil {
		fmt.Printf("Browser view created at %s\n", reportPath)
		return
	}

	fmt.Printf("Opened browser view: %s\n", reportPath)
}
