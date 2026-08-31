package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type mapProjectResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Role string `json:"role"`
}

// Presentation hints only: these never decide project membership or filtering.
func mapProjectRole(directory string) string {
	if info, err := os.Stat(filepath.Join(directory, "Cargo.toml")); err == nil && !info.IsDir() {
		return "rust"
	}
	var manifest struct {
		Dependencies    map[string]json.RawMessage `json:"dependencies"`
		DevDependencies map[string]json.RawMessage `json:"devDependencies"`
	}
	if raw, err := os.ReadFile(filepath.Join(directory, "package.json")); err == nil {
		_ = json.Unmarshal(raw, &manifest)
	}
	has := func(names ...string) bool {
		for _, name := range names {
			if _, ok := manifest.Dependencies[name]; ok {
				return true
			}
			if _, ok := manifest.DevDependencies[name]; ok {
				return true
			}
		}
		return false
	}
	if has("@nestjs/core", "express", "fastify", "koa", "hono") {
		return "backend"
	}
	if has("react", "react-dom", "svelte", "vue", "@angular/core", "next") {
		return "frontend"
	}
	switch strings.ToLower(filepath.Base(directory)) {
	case "frontend", "client", "web", "ui":
		return "frontend"
	case "backend", "server", "api":
		return "backend"
	}
	return "unknown"
}

type mapModuleResponse struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"projectId"`
	Label        string   `json:"label"`
	Path         string   `json:"path"`
	IsTest       bool     `json:"isTest"`
	MemberIDs    []NodeID `json:"memberIds"`
	FileCount    int      `json:"fileCount"`
	ChangedCount int      `json:"changedCount"`
	EntryPoints  []NodeID `json:"entryPoints"`
}

type mapModuleEdgeResponse struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Kind     EdgeKind          `json:"kind"`
	Count    int               `json:"count"`
	Evidence []mapEdgeResponse `json:"evidence"`
}

type mapArchitectureResponse struct {
	Modules []mapModuleResponse     `json:"modules"`
	Edges   []mapModuleEdgeResponse `json:"edges"`
}

func isTestSource(relative string) bool {
	clean := strings.ToLower(filepath.ToSlash(relative))
	parts := strings.Split(clean, "/")
	for _, directory := range parts[:len(parts)-1] {
		if directory == "test" || directory == "tests" || directory == "__tests__" {
			return true
		}
	}
	base := path.Base(clean)
	ext := path.Ext(base)
	if ext == ".go" {
		return strings.HasSuffix(base, "_test.go")
	}
	if ext == ".py" {
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	}
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		stem := strings.TrimSuffix(base, ext)
		return strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec")
	}
	return false
}

// Assign every source to the nearest manifest or repository boundary. Walk
// ancestors, not dependency trees, and cache results shared by sibling files.
func addMapArchitecture(root string, snapshot *mapSnapshot) error {
	root = filepath.Clean(root)
	cache := make(map[string]string)
	var projectRoot func(string) (string, error)
	projectRoot = func(directory string) (string, error) {
		if found, ok := cache[directory]; ok {
			return found, nil
		}
		for _, marker := range []string{"package.json", "go.mod", "Cargo.toml", ".git"} {
			if _, err := os.Lstat(filepath.Join(directory, marker)); err == nil {
				cache[directory] = directory
				return directory, nil
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
		if directory == root {
			cache[directory] = directory
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("file outside map root")
		}
		found, err := projectRoot(parent)
		if err == nil {
			cache[directory] = found
		}
		return found, err
	}
	projects := make(map[string]mapProjectResponse)
	modules := make(map[string]*mapModuleResponse)
	membership := make(map[NodeID]string)
	for i := range snapshot.Response.Nodes {
		node := &snapshot.Response.Nodes[i]
		file := filepath.Join(root, filepath.FromSlash(string(node.ID)))
		projectDir, err := projectRoot(filepath.Dir(file))
		if err != nil {
			return err
		}
		relProject, err := filepath.Rel(root, projectDir)
		if err != nil {
			return err
		}
		relFile, err := filepath.Rel(projectDir, file)
		if err != nil {
			return err
		}
		projectPath := filepath.ToSlash(relProject)
		projectID := "project:" + projectPath
		if _, exists := projects[projectID]; !exists {
			projects[projectID] = mapProjectResponse{ID: projectID, Name: filepath.Base(projectDir), Path: projectPath, Role: mapProjectRole(projectDir)}
		}
		node.ProjectID = projectID
		node.IsTest = isTestSource(relFile)
		if node.IsTest {
			node.IsRoot = false
		}
		modulePath, label := ".", "Root files"
		moduleKey := "root"
		if node.IsTest {
			moduleKey, label = "tests", "Tests"
		} else {
			rel := filepath.ToSlash(relFile)
			trimmed := strings.TrimPrefix(rel, "src/")
			if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
				label = trimmed[:slash]
				modulePath = label
				if strings.HasPrefix(rel, "src/") {
					modulePath = "src/" + label
				}
				moduleKey = "folder:" + modulePath
			}
		}
		moduleID := "module:" + projectPath + ":" + moduleKey
		group := modules[moduleID]
		if group == nil {
			group = &mapModuleResponse{ID: moduleID, ProjectID: projectID, Label: label,
				Path: path.Join(projectPath, modulePath), IsTest: node.IsTest,
				MemberIDs: []NodeID{}, EntryPoints: []NodeID{}}
			modules[moduleID] = group
		}
		group.MemberIDs = append(group.MemberIDs, node.ID)
		group.FileCount++
		if node.Change != nil {
			group.ChangedCount++
		}
		if node.IsRoot {
			group.EntryPoints = append(group.EntryPoints, node.ID)
		}
		membership[node.ID] = moduleID
	}
	response := &snapshot.Response
	response.Projects = []mapProjectResponse{}
	response.Architecture = mapArchitectureResponse{Modules: []mapModuleResponse{}, Edges: []mapModuleEdgeResponse{}}
	for _, project := range projects {
		response.Projects = append(response.Projects, project)
	}
	sort.Slice(response.Projects, func(i, j int) bool { return response.Projects[i].ID < response.Projects[j].ID })
	for _, group := range modules {
		sort.Slice(group.MemberIDs, func(i, j int) bool { return group.MemberIDs[i] < group.MemberIDs[j] })
		sort.Slice(group.EntryPoints, func(i, j int) bool { return group.EntryPoints[i] < group.EntryPoints[j] })
		response.Architecture.Modules = append(response.Architecture.Modules, *group)
	}
	sort.Slice(response.Architecture.Modules, func(i, j int) bool { return response.Architecture.Modules[i].ID < response.Architecture.Modules[j].ID })
	type edgeKey struct {
		from, to string
		kind     EdgeKind
	}
	aggregated := make(map[edgeKey]*mapModuleEdgeResponse)
	seen := make(map[mapEdgeResponse]bool)
	for _, edge := range response.Edges {
		if edge.Kind != EdgeKindImports && edge.Kind != EdgeKindCalls {
			continue
		}
		from, to := membership[edge.From], membership[edge.To]
		if from == "" || to == "" || from == to || seen[edge] {
			continue
		}
		seen[edge] = true
		key := edgeKey{from, to, edge.Kind}
		if aggregated[key] == nil {
			aggregated[key] = &mapModuleEdgeResponse{From: from, To: to, Kind: edge.Kind, Evidence: []mapEdgeResponse{}}
		}
		aggregated[key].Evidence = append(aggregated[key].Evidence, edge)
		aggregated[key].Count++
	}
	for _, edge := range aggregated {
		sort.Slice(edge.Evidence, func(i, j int) bool {
			if edge.Evidence[i].From != edge.Evidence[j].From {
				return edge.Evidence[i].From < edge.Evidence[j].From
			}
			return edge.Evidence[i].To < edge.Evidence[j].To
		})
		response.Architecture.Edges = append(response.Architecture.Edges, *edge)
	}
	sort.Slice(response.Architecture.Edges, func(i, j int) bool {
		a, b := response.Architecture.Edges[i], response.Architecture.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	return nil
}
