package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestArchitectureClassifiesTestConventions(t *testing.T) {
	for _, name := range []string{"main_test.go", "users.service.spec.ts", "thing.test.tsx", "unit.spec.mjs", "test_app.py", "app_test.py", "tests/helper.rs", "src/__tests__/helper.ts", "test/fixture.go"} {
		if !isTestSource(name) {
			t.Errorf("missed test %q", name)
		}
	}
	for _, name := range []string{"LatestShotPanel.tsx", "inspect_models.rs", "vitest.config.ts", "contest/main.go", "src/main.rs", "testing.ts", "speculation.ts", "main.go"} {
		if isTestSource(name) {
			t.Errorf("misclassified application file %q", name)
		}
	}
}

func TestArchitectureProjectsModulesAndEvidence(t *testing.T) {
	root := t.TempDir()
	write := func(name string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, manifest := range []string{"frontend/package.json", "backend/package.json", "frontend/native/Cargo.toml", "tools/go.mod", "legacy/.git"} {
		write(manifest)
	}
	ids := []NodeID{"frontend/src/main.ts", "frontend/src/LatestShotPanel.tsx", "frontend/native/src/main.rs", "backend/src/main.ts", "backend/src/app.module.ts", "backend/src/auth/auth.module.ts", "backend/src/auth/auth.service.ts", "backend/src/auth/auth.service.spec.ts", "backend/tests/helper.ts", "backend/prisma/migrations/001/migration.sql", "tools/main.go", "tools/main_test.go", "legacy/lib.py", "loose.py"}
	snapshot := mapSnapshot{}
	for _, id := range ids {
		write(string(id))
		snapshot.Response.Nodes = append(snapshot.Response.Nodes, mapNodeResponse{ID: id, IsRoot: id == "backend/src/main.ts" || id == "tools/main_test.go"})
	}
	snapshot.Response.Nodes[7].Change = &mapChangeResponse{Status: changeModified}
	snapshot.Response.Edges = []mapEdgeResponse{
		{From: ids[3], To: ids[4], Kind: EdgeKindImports}, // internal root connection
		{From: ids[4], To: ids[5], Kind: EdgeKindImports},
		{From: ids[3], To: ids[6], Kind: EdgeKindImports}, // aggregated root -> auth
		{From: ids[3], To: ids[6], Kind: EdgeKindImports}, // deduplicated evidence
		{From: ids[3], To: ids[6], Kind: EdgeKindCalls},   // distinct kind
		{From: ids[7], To: ids[6], Kind: EdgeKindImports}, // tests -> auth
	}
	if err := addMapArchitecture(root, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Response.Projects) != 6 {
		t.Fatalf("projects: %#v", snapshot.Response.Projects)
	}
	if snapshot.Response.Nodes[3].ProjectID != "project:backend" || snapshot.Response.Nodes[2].ProjectID != "project:frontend/native" || snapshot.Response.Nodes[12].ProjectID != "project:legacy" || snapshot.Response.Nodes[13].ProjectID != "project:." {
		t.Fatal("incorrect project boundaries")
	}
	groups := map[string]mapModuleResponse{}
	membership := map[NodeID]int{}
	for _, group := range snapshot.Response.Architecture.Modules {
		groups[group.ID] = group
		for _, id := range group.MemberIDs {
			membership[id]++
		}
	}
	for _, id := range ids {
		if membership[id] != 1 {
			t.Errorf("%s belongs to %d groups", id, membership[id])
		}
	}
	tests := groups["module:backend:tests"]
	if tests.FileCount != 2 || tests.ChangedCount != 1 || len(tests.EntryPoints) != 0 {
		t.Fatalf("Tests = %#v", tests)
	}
	if _, exists := groups["module:frontend:tests"]; exists {
		t.Fatal("empty tests group")
	}
	if snapshot.Response.Nodes[11].IsRoot {
		t.Fatal("test became application entry point")
	}
	if groups["module:backend:folder:prisma"].FileCount != 1 {
		t.Fatal("missing migrations group")
	}
	if !reflect.DeepEqual(groups["module:backend:root"].EntryPoints, []NodeID{ids[3]}) {
		t.Fatal("missing app entry point")
	}
	edges := snapshot.Response.Architecture.Edges
	if len(edges) != 3 {
		t.Fatalf("edges = %#v", edges)
	}
	found := false
	for _, edge := range edges {
		if edge.From == "module:backend:root" && edge.Kind == EdgeKindImports {
			found = edge.Count == 2 && len(edge.Evidence) == 2 && edge.To == "module:backend:folder:src/auth"
		}
	}
	if !found {
		t.Fatal("lost aggregate dependency evidence")
	}
	before := snapshot.Response.Architecture
	if err := addMapArchitecture(root, &snapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot.Response.Architecture) {
		t.Fatal("unstable architecture")
	}
}
