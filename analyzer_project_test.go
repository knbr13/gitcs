package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeProject lays out a throwaway repository on disk so the analyzers can be
// exercised the way the CLI actually runs them: scan, build, analyze.
func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	return root
}

func edgeSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	graph, err := analyzeRepositoryGraph(root)
	if err != nil {
		t.Fatalf("analyzeRepositoryGraph: %v", err)
	}
	edges := make(map[string]bool, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges[string(edge.From)+" -> "+string(edge.To)] = true
	}
	return edges
}

func assertEdges(t *testing.T, edges map[string]bool, want, unwanted []string) {
	t.Helper()
	for _, edge := range want {
		if !edges[edge] {
			t.Errorf("missing edge %q (have: %v)", edge, keysOf(edges))
		}
	}
	for _, edge := range unwanted {
		if edges[edge] {
			t.Errorf("unexpected edge %q", edge)
		}
	}
}

func keysOf(edges map[string]bool) []string {
	list := make([]string, 0, len(edges))
	for edge := range edges {
		list = append(list, edge)
	}
	return list
}

func TestAnalyzeTypeScriptProject(t *testing.T) {
	root := writeProject(t, map[string]string{
		"package.json": `{"name":"demo","main":"src/main.ts"}`,
		"tsconfig.json": `{
  // aliases are the normal way real projects import
  "compilerOptions": {
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"], },
  },
}`,
		"src/main.ts": `
import { createRouter } from './router.js';
import { Button } from '@/components/button';
import { log } from '../shared/log';
import express from 'express';
createRouter(); Button(); log();
`,
		"src/router.ts":                   "export function createRouter() {}",
		"src/components/button/index.tsx": "export function Button() { return null; }",
		"shared/log.js":                   "export function log() {}",
	})

	edges := edgeSet(t, root)
	assertEdges(t, edges,
		[]string{
			// "./router.js" is TypeScript's emitted extension for router.ts.
			"src/main.ts -> src/router.ts",
			// "@/*" resolves through tsconfig paths.
			"src/main.ts -> src/components/button/index.tsx",
			"src/main.ts -> shared/log.js",
		},
		// A published package has no node in this repository.
		[]string{"src/main.ts -> express"},
	)

	graph, err := analyzeRepositoryGraph(root)
	if err != nil {
		t.Fatalf("analyzeRepositoryGraph: %v", err)
	}
	if !graph.Nodes["src/main.ts"].IsRoot {
		t.Error("package.json main should be marked as an entry point")
	}
}

func TestAnalyzeSvelteProject(t *testing.T) {
	root := writeProject(t, map[string]string{
		"src/main.js": `
import App from './App.svelte';
new App({ target: document.body });
`,
		"src/App.svelte": `
<script>
  import Card from './lib/Card.svelte';
  import { store } from './lib/store.js';
</script>
<h1>It doesn't break on apostrophes</h1>
<Card {store} />
`,
		"src/lib/Card.svelte": "<script>export let store;</script><div>{store}</div>",
		"src/lib/store.js":    "export const store = 0;",
	})

	assertEdges(t, edgeSet(t, root),
		[]string{
			"src/main.js -> src/App.svelte",
			"src/App.svelte -> src/lib/Card.svelte",
			"src/App.svelte -> src/lib/store.js",
		},
		nil,
	)
}

func TestAnalyzeRustProject(t *testing.T) {
	root := writeProject(t, map[string]string{
		"src/main.rs": `
mod config;
mod render;

use crate::render::html::Page;
use serde::Deserialize;

fn main() { let _ = Page::new(); }
`,
		"src/config.rs":      "pub struct Config;",
		"src/render/mod.rs":  "pub mod html;\npub mod text;",
		"src/render/html.rs": "use super::text::Plain;\npub struct Page;",
		"src/render/text.rs": "pub struct Plain;",
	})

	edges := edgeSet(t, root)
	assertEdges(t, edges,
		[]string{
			"src/main.rs -> src/config.rs",
			"src/main.rs -> src/render/mod.rs",
			"src/main.rs -> src/render/html.rs",
			"src/render/mod.rs -> src/render/html.rs",
			"src/render/mod.rs -> src/render/text.rs",
			"src/render/html.rs -> src/render/text.rs",
		},
		[]string{"src/main.rs -> serde"},
	)
}

// A repository with more than one language must produce one graph, not one per
// language, or the map cannot show how the halves of a project connect.
func TestAnalyzeMixedLanguageProject(t *testing.T) {
	root := writeProject(t, map[string]string{
		"src/main.rs":       "mod api;\nfn main() {}",
		"src/api.rs":        "pub fn serve() {}",
		"web/src/main.ts":   "import { render } from './render';\nrender();",
		"web/src/render.ts": "export function render() {}",
	})

	assertEdges(t, edgeSet(t, root),
		[]string{
			"src/main.rs -> src/api.rs",
			"web/src/main.ts -> web/src/render.ts",
		},
		nil,
	)
}

// A JavaScript project is rarely at the repository root -- it sits under web/,
// frontend/, or packages/*. Its manifest, its aliases, and its entry point all
// have to be read from where it actually lives.
func TestAnalyzeNestedAndMonorepoProjects(t *testing.T) {
	root := writeProject(t, map[string]string{
		// A web client nested inside a repository that is mostly something else.
		"frontend/package.json":   `{"name":"web","main":"src/main.js"}`,
		"frontend/src/main.js":    "import App from './App.svelte';\nnew App();",
		"frontend/src/App.svelte": "<script>export default {}</script>",

		// A monorepo package with an alias of its own.
		"packages/ui/package.json":  `{"name":"ui","module":"src/index.ts"}`,
		"packages/ui/tsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"~/*":["src/*"]}}}`,
		"packages/ui/src/index.ts":  "import { Button } from '~/button';\nexport { Button };",
		"packages/ui/src/button.ts": "export function Button() {}",

		// Same alias spelling, different package: it must not leak across.
		"packages/api/src/index.ts": "import { thing } from '~/thing';\nthing();",
		"packages/api/src/thing.ts": "export function thing() {}",
	})

	edges := edgeSet(t, root)
	assertEdges(t, edges,
		[]string{
			"frontend/src/main.js -> frontend/src/App.svelte",
			"packages/ui/src/index.ts -> packages/ui/src/button.ts",
		},
		[]string{
			// packages/api declares no alias, so "~/thing" resolves to nothing --
			// and certainly not into packages/ui.
			"packages/api/src/index.ts -> packages/ui/src/thing.ts",
			"packages/api/src/index.ts -> packages/api/src/thing.ts",
		},
	)

	graph, err := analyzeRepositoryGraph(root)
	if err != nil {
		t.Fatalf("analyzeRepositoryGraph: %v", err)
	}
	for _, id := range []NodeID{"frontend/src/main.js", "packages/ui/src/index.ts"} {
		if !graph.Nodes[id].IsRoot {
			t.Errorf("%q should be marked as an entry point", id)
		}
	}
}
