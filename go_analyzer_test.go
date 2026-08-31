package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGoFunctions(t *testing.T) {
	tempDirectory := t.TempDir()
	filePath := filepath.Join(tempDirectory, "sample.go")

	source := `package sample

func StartServer() {}

type User struct{}

func (User) Save() {}

var Port = 3000
`

	err := os.WriteFile(filePath, []byte(source), 0600)
	if err != nil {
		t.Fatalf("could not create test file: %v", err)
	}

	functions, err := findGoFunctions(filePath)
	if err != nil {
		t.Fatalf("could not analyze Go file: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}

	if functions[0] != "StartServer" {
		t.Fatalf("expected StartServer, got %q", functions[0])
	}
}

func TestFindGoCalls(t *testing.T) {
	tempDirectory := t.TempDir()
	filePath := filepath.Join(tempDirectory, "sample.go")

	source := `package sample

import "fmt"

func StartServer() {
	loadRoutes()
	fmt.Println("ready")
}
`

	err := os.WriteFile(filePath, []byte(source), 0600)
	if err != nil {
		t.Fatalf("could not create test file: %v", err)
	}

	calls, err := findGoCalls(filePath)
	if err != nil {
		t.Fatalf("could not analyze Go calls: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 simple function call, got %d: %v", len(calls), calls)
	}

	if calls[0] != "loadRoutes" {
		t.Fatalf("expected loadRoutes, got %q", calls[0])
	}
}

func TestBuildGoFunctionIndex(t *testing.T) {
	tempDirectory := t.TempDir()
	serverPath := filepath.Join(tempDirectory, "server.go")
	routesPath := filepath.Join(tempDirectory, "routes.go")

	if err := os.WriteFile(serverPath, []byte("package sample\n\nfunc StartServer() {}\n"), 0600); err != nil {
		t.Fatalf("could not create server file: %v", err)
	}
	if err := os.WriteFile(routesPath, []byte("package sample\n\nfunc LoadRoutes() {}\n"), 0600); err != nil {
		t.Fatalf("could not create routes file: %v", err)
	}

	graph := NewGraph()
	graph.AddNode(Node{ID: "server.go", Path: serverPath, Language: "Go", Kind: NodeKindFile})
	graph.AddNode(Node{ID: "routes.go", Path: routesPath, Language: "Go", Kind: NodeKindFile})
	graph.AddNode(Node{ID: "frontend.ts", Path: "frontend.ts", Language: "TypeScript", Kind: NodeKindFile})

	index, err := buildGoFunctionIndex(graph)
	if err != nil {
		t.Fatalf("could not build Go function index: %v", err)
	}

	if len(index["StartServer"]) != 1 || index["StartServer"][0] != "server.go" {
		t.Fatalf("expected StartServer to belong to server.go, got %v", index["StartServer"])
	}
	if len(index["LoadRoutes"]) != 1 || index["LoadRoutes"][0] != "routes.go" {
		t.Fatalf("expected LoadRoutes to belong to routes.go, got %v", index["LoadRoutes"])
	}
}

func TestGoAnalyzerFindsUniqueCrossFileConnection(t *testing.T) {
	root := t.TempDir()
	serverPath := filepath.Join(root, "server.go")
	routesPath := filepath.Join(root, "routes.go")

	serverSource := `package sample

func StartServer() {
	LoadRoutes()
	LoadRoutes()
	localHelper()
}

func localHelper() {}
`
	routesSource := "package sample\n\nfunc LoadRoutes() {}\n"

	if err := os.WriteFile(serverPath, []byte(serverSource), 0600); err != nil {
		t.Fatalf("could not create server file: %v", err)
	}
	if err := os.WriteFile(routesPath, []byte(routesSource), 0600); err != nil {
		t.Fatalf("could not create routes file: %v", err)
	}

	files := []SourceFile{
		{Path: serverPath, Language: "Go"},
		{Path: routesPath, Language: "Go"},
	}
	graph, err := buildFileGraph(root, files)
	if err != nil {
		t.Fatalf("could not build file graph: %v", err)
	}
	analyzer, err := NewGoAnalyzer(graph)
	if err != nil {
		t.Fatalf("could not create Go analyzer: %v", err)
	}

	edges, err := analyzer.FindConnections(root, files[0], graph)
	if err != nil {
		t.Fatalf("could not find Go connections: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 deduplicated cross-file edge, got %d: %v", len(edges), edges)
	}
	if edges[0] != (Edge{From: "server.go", To: "routes.go", Kind: EdgeKindCalls}) {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}
}

func TestGoAnalyzerSkipsAmbiguousConnection(t *testing.T) {
	root := t.TempDir()
	callerPath := filepath.Join(root, "caller.go")
	firstPath := filepath.Join(root, "first.go")
	secondPath := filepath.Join(root, "second.go")

	if err := os.WriteFile(callerPath, []byte("package sample\n\nfunc Caller() { Start() }\n"), 0600); err != nil {
		t.Fatalf("could not create caller file: %v", err)
	}
	if err := os.WriteFile(firstPath, []byte("package sample\n\nfunc Start() {}\n"), 0600); err != nil {
		t.Fatalf("could not create first target: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("package sample\n\nfunc Start() {}\n"), 0600); err != nil {
		t.Fatalf("could not create second target: %v", err)
	}

	files := []SourceFile{
		{Path: callerPath, Language: "Go"},
		{Path: firstPath, Language: "Go"},
		{Path: secondPath, Language: "Go"},
	}
	graph, err := buildFileGraph(root, files)
	if err != nil {
		t.Fatalf("could not build file graph: %v", err)
	}
	analyzer, err := NewGoAnalyzer(graph)
	if err != nil {
		t.Fatalf("could not create Go analyzer: %v", err)
	}

	edges, err := analyzer.FindConnections(root, files[0], graph)
	if err != nil {
		t.Fatalf("could not find Go connections: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("expected ambiguous call to create no edge, got %v", edges)
	}
}

func TestNewGoAnalyzerMarksMainFunctionAsRoot(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.go")
	if err := os.WriteFile(mainPath, []byte("package main\n\nfunc main() {}\n"), 0600); err != nil {
		t.Fatalf("could not create main file: %v", err)
	}

	graph, err := buildFileGraph(root, []SourceFile{{Path: mainPath, Language: "Go"}})
	if err != nil {
		t.Fatalf("could not build file graph: %v", err)
	}
	if _, err := NewGoAnalyzer(graph); err != nil {
		t.Fatalf("could not create Go analyzer: %v", err)
	}

	if !graph.Nodes["main.go"].IsRoot {
		t.Fatal("expected a file declaring main() to be marked as a root")
	}
}
