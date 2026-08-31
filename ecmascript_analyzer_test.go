package main

import (
	"encoding/json"
	"testing"
)

func TestResolveECMAScriptImport(t *testing.T) {
	graph := NewGraph()
	for _, id := range []NodeID{
		"src/app.ts",
		"src/routes.ts",
		"src/components/button/index.tsx",
		"shared/log.js",
	} {
		graph.AddNode(Node{ID: id, Kind: NodeKindFile})
	}

	tests := []struct {
		name      string
		from      NodeID
		specifier string
		want      NodeID
		wantFound bool
	}{
		{
			name:      "extensionless TypeScript file",
			from:      "src/app.ts",
			specifier: "./routes",
			want:      "src/routes.ts",
			wantFound: true,
		},
		{
			name:      "directory index",
			from:      "src/app.ts",
			specifier: "./components/button",
			want:      "src/components/button/index.tsx",
			wantFound: true,
		},
		{
			name:      "explicit JavaScript extension",
			from:      "src/app.ts",
			specifier: "../shared/log.js",
			want:      "shared/log.js",
			wantFound: true,
		},
		{
			name:      "package import is outside the repository graph",
			from:      "src/app.ts",
			specifier: "svelte",
			wantFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := resolveECMAScriptImport(test.from, test.specifier, graph)
			if found != test.wantFound || got != test.want {
				t.Fatalf(
					"resolveECMAScriptImport(%q, %q) = (%q, %t), want (%q, %t)",
					test.from,
					test.specifier,
					got,
					found,
					test.want,
					test.wantFound,
				)
			}
		})
	}
}

func TestFindECMAScriptImports(t *testing.T) {
	source := `
import { mount } from 'svelte';
import App from "./App.svelte";
import './lib/tokens.css';
import type { Node } from './types';
import {
  first,
  second
} from './wrapped';
export { helper } from './helper';
export * from './everything';
const lazy = await import('./lazy');
const legacy = require('./legacy');
// import { commented } from './commented';
/* import { blocked } from './blocked'; */
const text = "import { quoted } from './quoted';";
`

	got := findECMAScriptImports(source, "JavaScript")
	want := map[string]bool{
		"svelte":           true,
		"./App.svelte":     true,
		"./lib/tokens.css": true,
		"./types":          true,
		"./wrapped":        true,
		"./helper":         true,
		"./everything":     true,
		"./lazy":           true,
		"./legacy":         true,
		"./commented":      false,
		"./blocked":        false,
		"./quoted":         false,
	}

	found := make(map[string]bool, len(got))
	for _, specifier := range got {
		found[specifier] = true
	}

	for specifier, expected := range want {
		if found[specifier] != expected {
			t.Errorf("import %q: found = %t, want %t (all: %v)", specifier, found[specifier], expected, got)
		}
	}
}

func TestFindECMAScriptImportsInSingleFileComponent(t *testing.T) {
	source := `
<script lang="ts">
  import Card from './Card.svelte';
</script>

<p>It doesn't matter that this apostrophe is here.</p>
<div class="x">import { fake } from './fake';</div>
`

	got := findECMAScriptImports(source, "Svelte")
	if len(got) != 1 || got[0] != "./Card.svelte" {
		t.Fatalf("findECMAScriptImports(svelte) = %v, want [./Card.svelte]", got)
	}
}

func TestResolveECMAScriptImportPrefersTypeScriptSource(t *testing.T) {
	graph := NewGraph()
	graph.AddNode(Node{ID: "src/app.ts", Kind: NodeKindFile})
	graph.AddNode(Node{ID: "src/routes.ts", Kind: NodeKindFile})

	// TypeScript ESM asks for the emitted extension; the file on disk is .ts.
	got, found := resolveECMAScriptImport("src/app.ts", "./routes.js", graph)
	if !found || got != "src/routes.ts" {
		t.Fatalf("resolveECMAScriptImport = (%q, %t), want (src/routes.ts, true)", got, found)
	}
}

func TestResolveECMAScriptImportStaysInsideTheRepository(t *testing.T) {
	graph := NewGraph()
	graph.AddNode(Node{ID: "src/app.ts", Kind: NodeKindFile})

	if _, found := resolveECMAScriptImport("src/app.ts", "../../outside/thing", graph); found {
		t.Fatal("an import that escapes the repository root must not resolve")
	}
}

func TestExpandECMAScriptAlias(t *testing.T) {
	project := ecmaScriptProject{
		aliases: []ecmaScriptAlias{
			{prefix: "@/", wildcard: true, targets: []string{"src/*"}},
			{prefix: "@config", targets: []string{"config/index.ts"}},
			{wildcard: true, targets: []string{"src/*"}},
		},
	}

	tests := []struct {
		specifier string
		want      string
	}{
		{"@/components/button", "src/components/button"},
		{"@config", "config/index.ts"},
		{"components/button", "src/components/button"},
	}

	for _, test := range tests {
		candidates := project.expandAlias(test.specifier)
		if len(candidates) == 0 || candidates[0] != test.want {
			t.Errorf("expandAlias(%q) = %v, want first = %q", test.specifier, candidates, test.want)
		}
	}

	if candidates := project.expandAlias("./relative"); candidates != nil {
		t.Errorf("expandAlias(relative) = %v, want nil", candidates)
	}
}

func TestNormalizeJSONC(t *testing.T) {
	source := `{
  // a line comment
  "compilerOptions": {
    /* a block comment */
    "baseUrl": ".", // trailing
    "paths": { "@/*": ["./src/*"], },
  },
}`

	var config tsconfigFile
	if err := json.Unmarshal([]byte(normalizeJSONC(source)), &config); err != nil {
		t.Fatalf("normalizeJSONC produced invalid JSON: %v", err)
	}
	if got := config.CompilerOptions.Paths["@/*"]; len(got) != 1 || got[0] != "./src/*" {
		t.Fatalf("paths = %v, want [./src/*]", got)
	}
}
