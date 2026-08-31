package main

import (
	"os"
	"path"
	"regexp"
	"strings"
)

// The Rust analyzer reads a crate's module tree.
//
// Rust says where its modules live in two different ways, and both matter. A
// `mod foo;` declaration *creates* the link between a file and its child module
// file; a `use crate::foo::bar::Thing;` path *follows* one. Reading only the
// second would miss the tree's actual shape, and reading only the first would
// miss which modules genuinely depend on which.
//
// Paths are resolved against the crate root -- the directory holding lib.rs or
// main.rs -- found by walking up from each file, so a Cargo workspace with many
// crates maps correctly instead of collapsing into one.

var (
	// `mod foo;` declares a child module in another file. `mod foo { ... }` is
	// inline and has no file of its own, so the terminating semicolon matters.
	rustModulePattern = regexp.MustCompile(`(?m)^[ \t]*(?:pub(?:\s*\([^)]*\))?\s+)?mod\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	rustUsePattern    = regexp.MustCompile(`(?s)\buse\s+([^;]+);`)
	rustPathSegment   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Files that name their module after their directory rather than themselves.
var rustDirectoryModules = map[string]struct{}{
	"mod.rs":  {},
	"lib.rs":  {},
	"main.rs": {},
}

type RustAnalyzer struct{}

func NewRustAnalyzer(graph *Graph) *RustAnalyzer {
	markRustCrateRoots(graph)
	return &RustAnalyzer{}
}

func (analyzer *RustAnalyzer) CanAnalyze(language string) bool {
	return language == "Rust"
}

func (analyzer *RustAnalyzer) FindConnections(
	root string,
	file SourceFile,
	graph *Graph,
) ([]Edge, error) {
	from, ok := repositoryNodeID(root, file.Path)
	if !ok {
		return nil, nil
	}
	if _, exists := graph.Nodes[from]; !exists {
		return nil, nil
	}

	source, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, nil
	}
	code := stripComments(string(source), rustComments)

	seen := make(map[NodeID]struct{})
	var edges []Edge

	add := func(to NodeID, found bool) {
		if !found || to == from {
			return
		}
		if _, already := seen[to]; already {
			return
		}
		seen[to] = struct{}{}
		edges = append(edges, Edge{From: from, To: to, Kind: EdgeKindImports})
	}

	for _, name := range findRustModuleDeclarations(code) {
		add(resolveRustModuleFile(path.Join(rustModuleDirectory(from), name), graph))
	}
	for _, usePath := range findRustUsePaths(code) {
		add(resolveRustUsePath(from, usePath, graph))
	}

	return edges, nil
}

// findRustModuleDeclarations returns the names in `mod name;` declarations.
func findRustModuleDeclarations(code string) []string {
	var names []string
	for _, match := range rustModulePattern.FindAllStringSubmatch(code, -1) {
		names = append(names, match[1])
	}
	return names
}

// findRustUsePaths returns one fully-qualified path per imported item, with
// brace groups expanded: `use crate::{a, b::c};` becomes crate::a and
// crate::b::c, because only the expanded forms name a module that might be a
// file.
func findRustUsePaths(code string) []string {
	var paths []string
	for _, match := range rustUsePattern.FindAllStringSubmatch(code, -1) {
		body := strings.TrimSpace(match[1])
		// `use a::b as c;` -- the rename says nothing about where the file is.
		if index := strings.Index(body, " as "); index >= 0 && !strings.Contains(body, "{") {
			body = body[:index]
		}
		paths = append(paths, expandRustUseGroups(body)...)
	}
	return paths
}

// expandRustUseGroups flattens `prefix::{a, b::{c, d}}` into full paths.
func expandRustUseGroups(body string) []string {
	open := strings.Index(body, "{")
	if open < 0 {
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	closing := matchingBrace(body, open)
	if closing < 0 {
		return nil
	}

	prefix := strings.TrimSpace(body[:open])
	suffix := strings.TrimSpace(body[closing+1:])
	var expanded []string

	for _, part := range splitTopLevel(body[open+1 : closing]) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, inner := range expandRustUseGroups(part) {
			expanded = append(expanded, strings.TrimSpace(prefix+inner+suffix))
		}
	}

	return expanded
}

func matchingBrace(body string, open int) int {
	depth := 0
	for index := open; index < len(body); index++ {
		switch body[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// splitTopLevel splits on commas that are not inside a nested brace group.
func splitTopLevel(body string) []string {
	var parts []string
	depth, start := 0, 0
	for index := 0; index < len(body); index++ {
		switch body[index] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, body[start:])
}

// --- Resolution -----------------------------------------------------------

// rustModuleDirectory is where a file's child modules live. A file named after
// its directory (mod.rs, lib.rs, main.rs) owns that directory; any other file
// owns a directory named after itself.
func rustModuleDirectory(from NodeID) string {
	directory, name := path.Split(string(from))
	directory = strings.TrimSuffix(directory, "/")
	if _, ownsDirectory := rustDirectoryModules[name]; ownsDirectory {
		return directory
	}
	return path.Join(directory, strings.TrimSuffix(name, path.Ext(name)))
}

// resolveRustModuleFile finds the file backing a module path, in either of the
// two layouts Rust allows: foo.rs beside its parent, or foo/mod.rs.
func resolveRustModuleFile(modulePath string, graph *Graph) (NodeID, bool) {
	modulePath = strings.TrimPrefix(path.Clean(modulePath), "/")
	if modulePath == "" || modulePath == "." || strings.HasPrefix(modulePath, "../") {
		return "", false
	}
	if id := NodeID(modulePath + ".rs"); isFileNode(graph, id) {
		return id, true
	}
	if id := NodeID(modulePath + "/mod.rs"); isFileNode(graph, id) {
		return id, true
	}
	return "", false
}

// resolveRustUsePath follows a `use` path to the file that defines it. The
// trailing segments of such a path name types and functions rather than
// modules, so the longest prefix that resolves to a file is the answer.
func resolveRustUsePath(from NodeID, usePath string, graph *Graph) (NodeID, bool) {
	segments := splitRustPath(usePath)
	if len(segments) == 0 {
		return "", false
	}

	var base string
	switch segments[0] {
	case "crate":
		base = rustCrateRoot(from, graph)
		segments = segments[1:]
	case "self":
		base = rustModuleDirectory(from)
		segments = segments[1:]
	case "super":
		base = path.Dir(rustModuleDirectory(from))
		segments = segments[1:]
		// Rust allows super::super::; each one climbs another level.
		for len(segments) > 0 && segments[0] == "super" {
			base = path.Dir(base)
			segments = segments[1:]
		}
	case "std", "core", "alloc":
		// The standard library is not part of anyone's repository.
		return "", false
	default:
		// Either a 2015-edition crate-relative path or an external crate. Trying
		// the crate root costs nothing: if no file matches, it was a package.
		base = rustCrateRoot(from, graph)
	}

	for length := len(segments); length > 0; length-- {
		candidate := path.Join(append([]string{base}, segments[:length]...)...)
		if id, found := resolveRustModuleFile(candidate, graph); found {
			return id, true
		}
	}

	return "", false
}

// splitRustPath returns the plain identifier segments of a use path, stopping
// at anything that is not one -- a glob, a rename, a generic argument.
func splitRustPath(usePath string) []string {
	var segments []string
	for _, segment := range strings.Split(usePath, "::") {
		segment = strings.TrimSpace(segment)
		if !rustPathSegment.MatchString(segment) {
			break
		}
		segments = append(segments, segment)
	}
	return segments
}

// rustCrateRoot walks up from a file to the nearest directory holding lib.rs or
// main.rs, so each crate in a workspace resolves against its own root.
func rustCrateRoot(from NodeID, graph *Graph) string {
	directory := path.Dir(string(from))
	for {
		if isFileNode(graph, NodeID(path.Join(directory, "lib.rs"))) ||
			isFileNode(graph, NodeID(path.Join(directory, "main.rs"))) {
			return directory
		}
		parent := path.Dir(directory)
		if parent == directory || parent == "." {
			return directory
		}
		directory = parent
	}
}

// markRustCrateRoots flags every crate entry point in the repository, including
// each member of a Cargo workspace.
func markRustCrateRoots(graph *Graph) {
	for id, node := range graph.Nodes {
		if node.Kind != NodeKindFile || node.Language != "Rust" {
			continue
		}
		switch path.Base(string(id)) {
		case "main.rs", "lib.rs":
			markRootNode(graph, id)
		}
	}
}
