package main

import (
	"reflect"
	"testing"
)

func rustFixtureGraph(ids ...NodeID) *Graph {
	graph := NewGraph()
	for _, id := range ids {
		graph.AddNode(Node{ID: id, Language: "Rust", Kind: NodeKindFile})
	}
	return graph
}

func TestFindRustModuleDeclarations(t *testing.T) {
	code := stripComments(`
mod parser;
pub mod graph;
pub(crate) mod store;
mod inline { fn helper() {} }
// mod commented;
/* mod blocked; */
`, rustComments)

	got := findRustModuleDeclarations(code)
	want := []string{"parser", "graph", "store"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findRustModuleDeclarations = %v, want %v", got, want)
	}
}

func TestFindRustUsePathsExpandsGroups(t *testing.T) {
	code := stripComments(`
use crate::graph::Node;
use crate::{analyzer, store::Index};
use crate::render::{html::Page, text::{Plain, Rich}};
use std::collections::HashMap;
use super::helper;
use crate::legacy as old;
`, rustComments)

	got := findRustUsePaths(code)
	want := []string{
		"crate::graph::Node",
		"crate::analyzer",
		"crate::store::Index",
		"crate::render::html::Page",
		"crate::render::text::Plain",
		"crate::render::text::Rich",
		"std::collections::HashMap",
		"super::helper",
		"crate::legacy",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findRustUsePaths = %v, want %v", got, want)
	}
}

func TestRustModuleDirectory(t *testing.T) {
	tests := map[NodeID]string{
		"src/lib.rs":        "src",
		"src/main.rs":       "src",
		"src/parser.rs":     "src/parser",
		"src/parser/mod.rs": "src/parser",
		"src/a/b.rs":        "src/a/b",
	}
	for from, want := range tests {
		if got := rustModuleDirectory(from); got != want {
			t.Errorf("rustModuleDirectory(%q) = %q, want %q", from, got, want)
		}
	}
}

func TestResolveRustModuleFileBothLayouts(t *testing.T) {
	graph := rustFixtureGraph("src/lib.rs", "src/parser.rs", "src/store/mod.rs")

	if got, found := resolveRustModuleFile("src/parser", graph); !found || got != "src/parser.rs" {
		t.Errorf("flat layout = (%q, %t), want (src/parser.rs, true)", got, found)
	}
	if got, found := resolveRustModuleFile("src/store", graph); !found || got != "src/store/mod.rs" {
		t.Errorf("directory layout = (%q, %t), want (src/store/mod.rs, true)", got, found)
	}
	if _, found := resolveRustModuleFile("src/missing", graph); found {
		t.Error("a module with no file must not resolve")
	}
}

func TestResolveRustUsePath(t *testing.T) {
	graph := rustFixtureGraph(
		"src/lib.rs",
		"src/graph.rs",
		"src/render/mod.rs",
		"src/render/html.rs",
		"src/render/text.rs",
	)

	tests := []struct {
		name      string
		from      NodeID
		usePath   string
		want      NodeID
		wantFound bool
	}{
		{
			name:      "crate-relative path to a sibling module",
			from:      "src/render/html.rs",
			usePath:   "crate::graph::Node",
			want:      "src/graph.rs",
			wantFound: true,
		},
		{
			name:      "longest prefix wins over the type name",
			from:      "src/lib.rs",
			usePath:   "crate::render::html::Page",
			want:      "src/render/html.rs",
			wantFound: true,
		},
		{
			// `super` is the parent *module*, so from crate::render::html it
			// means crate::render -- not the crate root.
			name:      "super climbs one module, not to the root",
			from:      "src/render/html.rs",
			usePath:   "super::text::Plain",
			want:      "src/render/text.rs",
			wantFound: true,
		},
		{
			name:    "super does not reach past its own parent",
			from:    "src/render/html.rs",
			usePath: "super::graph",
		},
		{
			name:      "self stays in the current module",
			from:      "src/render/mod.rs",
			usePath:   "self::html::Page",
			want:      "src/render/html.rs",
			wantFound: true,
		},
		{
			name:    "the standard library is not in the repository",
			from:    "src/lib.rs",
			usePath: "std::collections::HashMap",
		},
		{
			name:    "an external crate resolves to nothing",
			from:    "src/lib.rs",
			usePath: "serde::Deserialize",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := resolveRustUsePath(test.from, test.usePath, graph)
			if found != test.wantFound || got != test.want {
				t.Fatalf(
					"resolveRustUsePath(%q, %q) = (%q, %t), want (%q, %t)",
					test.from, test.usePath, got, found, test.want, test.wantFound,
				)
			}
		})
	}
}

// A Cargo workspace has several crate roots; each file must resolve `crate`
// against its own, or every crate collapses into whichever root is found first.
func TestRustCrateRootPerWorkspaceMember(t *testing.T) {
	graph := rustFixtureGraph(
		"crates/api/src/lib.rs",
		"crates/api/src/routes.rs",
		"crates/worker/src/main.rs",
		"crates/worker/src/queue.rs",
	)

	got, found := resolveRustUsePath("crates/worker/src/main.rs", "crate::queue::Job", graph)
	if !found || got != "crates/worker/src/queue.rs" {
		t.Fatalf("worker crate = (%q, %t), want (crates/worker/src/queue.rs, true)", got, found)
	}

	got, found = resolveRustUsePath("crates/api/src/lib.rs", "crate::routes::Router", graph)
	if !found || got != "crates/api/src/routes.rs" {
		t.Fatalf("api crate = (%q, %t), want (crates/api/src/routes.rs, true)", got, found)
	}

	// The worker has no `routes` module of its own.
	if _, found := resolveRustUsePath("crates/worker/src/main.rs", "crate::routes::Router", graph); found {
		t.Fatal("crate:: must not reach across workspace members")
	}
}

func TestMarkRustCrateRoots(t *testing.T) {
	graph := rustFixtureGraph("src/main.rs", "src/parser.rs", "crates/api/src/lib.rs")
	markRustCrateRoots(graph)

	for id, wantRoot := range map[NodeID]bool{
		"src/main.rs":           true,
		"crates/api/src/lib.rs": true,
		"src/parser.rs":         false,
	} {
		if graph.Nodes[id].IsRoot != wantRoot {
			t.Errorf("%q IsRoot = %t, want %t", id, graph.Nodes[id].IsRoot, wantRoot)
		}
	}
}
