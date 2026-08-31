package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// The ECMAScript analyzer reads the import graph of a JavaScript or TypeScript
// project: .js/.jsx/.mjs/.cjs, .ts/.tsx/.mts/.cts, and the script blocks of
// .svelte and .vue single-file components.
//
// It resolves specifiers the way a bundler does -- optional extension, then a
// directory index -- and it only ever produces an edge when the target is a
// file the repository scan already found. An import of a published package is
// simply not part of this graph, and saying so by returning no edge is more
// honest than inventing a node for it.

// Tried in the order a bundler would try them.
var ecmaScriptExtensions = []string{
	".ts", ".tsx", ".mts", ".cts",
	".js", ".jsx", ".mjs", ".cjs",
	".svelte", ".vue",
}

// TypeScript's ESM output asks you to write the extension the *compiler* will
// emit, so "./routes.js" routinely means "./routes.ts" on disk.
var ecmaScriptSourceForOutput = map[string][]string{
	".js":  {".ts", ".tsx"},
	".jsx": {".tsx"},
	".mjs": {".mts"},
	".cjs": {".cts"},
}

var ecmaScriptLanguages = map[string]struct{}{
	"JavaScript": {},
	"TypeScript": {},
	"Svelte":     {},
	"Vue":        {},
}

// The gap between the keyword and the specifier deliberately excludes quotes,
// which stops a bare `import './a'` from swallowing the next statement while
// still allowing a multi-line `import {\n a,\n b\n} from './b'`.
var ecmaScriptImportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bimport\b[^'"` + "`" + `]{0,400}?\bfrom\s*['"` + "`" + `]([^'"` + "`" + `]+)`),
	regexp.MustCompile(`\bexport\b[^'"` + "`" + `]{0,400}?\bfrom\s*['"` + "`" + `]([^'"` + "`" + `]+)`),
	regexp.MustCompile(`\bimport\s*['"` + "`" + `]([^'"` + "`" + `]+)`),
	regexp.MustCompile(`\bimport\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)`),
	regexp.MustCompile(`\brequire\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)`),
}

var scriptBlockPattern = regexp.MustCompile(`(?is)<script[^>]*>(.*?)</script>`)

type ECMAScriptAnalyzer struct {
	// One entry per JavaScript project found in the repository, deepest first.
	// A monorepo, or a Go service with a web/ client, has several -- and an
	// alias declared in one of them means nothing in the others.
	projects []ecmaScriptProject
}

type ecmaScriptProject struct {
	// Repository-relative directory holding the manifest; "" for the root.
	dir string
	// Expansions for non-relative specifiers, from tsconfig/jsconfig. Without
	// these an aliased project ("@/components/button") looks like it imports
	// nothing but packages.
	aliases []ecmaScriptAlias
}

type ecmaScriptAlias struct {
	prefix   string // "@/" for "@/*", or "" for a bare baseUrl lookup
	suffix   string
	wildcard bool
	targets  []string // repo-root-relative, slash separated
}

func NewECMAScriptAnalyzer(root string, graph *Graph) *ECMAScriptAnalyzer {
	analyzer := &ECMAScriptAnalyzer{}
	for _, dir := range findECMAScriptProjectDirs(root) {
		analyzer.projects = append(analyzer.projects, ecmaScriptProject{
			dir:     dir,
			aliases: loadECMAScriptAliases(root, dir),
		})
		markECMAScriptEntryPoints(root, dir, graph)
	}
	// Deepest first, so a package's own alias always beats the repository-wide
	// one it happens to sit inside.
	sort.SliceStable(analyzer.projects, func(left, right int) bool {
		return len(analyzer.projects[left].dir) > len(analyzer.projects[right].dir)
	})
	return analyzer
}

func (analyzer *ECMAScriptAnalyzer) CanAnalyze(language string) bool {
	_, supported := ecmaScriptLanguages[language]
	return supported
}

func (analyzer *ECMAScriptAnalyzer) FindConnections(
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
		// A file that vanished between the scan and the read is not a reason to
		// throw away the rest of the map.
		return nil, nil
	}

	seen := make(map[NodeID]struct{})
	var edges []Edge

	for _, specifier := range findECMAScriptImports(string(source), file.Language) {
		to, found := analyzer.resolve(from, specifier, graph)
		if !found || to == from {
			continue
		}
		if _, already := seen[to]; already {
			continue
		}
		seen[to] = struct{}{}
		edges = append(edges, Edge{From: from, To: to, Kind: EdgeKindImports})
	}

	return edges, nil
}

func (analyzer *ECMAScriptAnalyzer) resolve(
	from NodeID,
	specifier string,
	graph *Graph,
) (NodeID, bool) {
	if to, found := resolveECMAScriptImport(from, specifier, graph); found {
		return to, true
	}
	for _, project := range analyzer.projects {
		if !project.contains(from) {
			continue
		}
		for _, base := range project.expandAlias(specifier) {
			if to, found := resolveECMAScriptModule(base, graph); found {
				return to, true
			}
		}
	}
	return "", false
}

// contains reports whether a file belongs to this project's directory tree.
func (project ecmaScriptProject) contains(from NodeID) bool {
	return project.dir == "" || strings.HasPrefix(string(from), project.dir+"/")
}

// resolveECMAScriptImport maps a relative JavaScript or TypeScript import from
// one graph node to another. Keeping this function independent of parsing makes
// the module-resolution rules easy to test before the analyzer is wired in.
func resolveECMAScriptImport(
	from NodeID,
	specifier string,
	graph *Graph,
) (NodeID, bool) {
	if !isRelativeSpecifier(specifier) {
		return "", false
	}
	return resolveECMAScriptModule(path.Join(path.Dir(string(from)), specifier), graph)
}

// A specifier that does not start with a dot names a package, or an alias the
// caller has already expanded. Either way it is not resolvable on its own.
func isRelativeSpecifier(specifier string) bool {
	return strings.HasPrefix(specifier, "./") ||
		strings.HasPrefix(specifier, "../") ||
		specifier == "." ||
		specifier == ".."
}

// resolveECMAScriptModule turns a repository-relative module path into the file
// node it refers to, applying the same fallbacks a bundler does: the path as
// written, the TypeScript source behind a compiled extension, the path plus an
// extension, and finally a directory index.
func resolveECMAScriptModule(base string, graph *Graph) (NodeID, bool) {
	base = strings.TrimPrefix(path.Clean(base), "/")
	if base == "" || base == "." || strings.HasPrefix(base, "../") {
		return "", false
	}

	if isFileNode(graph, NodeID(base)) {
		return NodeID(base), true
	}

	extension := path.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	for _, source := range ecmaScriptSourceForOutput[extension] {
		if isFileNode(graph, NodeID(stem+source)) {
			return NodeID(stem + source), true
		}
	}

	for _, candidate := range ecmaScriptExtensions {
		if isFileNode(graph, NodeID(base+candidate)) {
			return NodeID(base + candidate), true
		}
	}
	for _, candidate := range ecmaScriptExtensions {
		if index := NodeID(base + "/index" + candidate); isFileNode(graph, index) {
			return index, true
		}
	}

	return "", false
}

func isFileNode(graph *Graph, id NodeID) bool {
	node, exists := graph.Nodes[id]
	return exists && node.Kind == NodeKindFile
}

// findECMAScriptImports collects every specifier a file imports. Single-file
// components are narrowed to their script blocks first, so an apostrophe in the
// markup cannot be mistaken for the start of a string literal.
func findECMAScriptImports(source, language string) []string {
	if language == "Svelte" || language == "Vue" {
		var scripts strings.Builder
		for _, block := range scriptBlockPattern.FindAllStringSubmatch(source, -1) {
			scripts.WriteString(block[1])
			scripts.WriteByte('\n')
		}
		source = scripts.String()
	}

	code, literals := maskStringContents(stripComments(source, ecmaScriptComments))

	var specifiers []string
	seen := make(map[string]struct{})
	for _, pattern := range ecmaScriptImportPatterns {
		for _, match := range pattern.FindAllStringSubmatch(code, -1) {
			specifier, isLiteral := literals.lookup(match[1])
			if !isLiteral {
				continue
			}
			specifier = strings.TrimSpace(specifier)
			if specifier == "" {
				continue
			}
			if _, already := seen[specifier]; already {
				continue
			}
			seen[specifier] = struct{}{}
			specifiers = append(specifiers, specifier)
		}
	}

	return specifiers
}

// --- Path aliases ---------------------------------------------------------

type tsconfigFile struct {
	Extends         string `json:"extends"`
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

var aliasConfigNames = []string{"tsconfig.json", "jsconfig.json"}

var projectManifestNames = []string{"package.json", "tsconfig.json", "jsconfig.json"}

// findECMAScriptProjectDirs locates every directory holding a JavaScript
// manifest, so a client nested under web/ or a monorepo package under
// packages/ui is understood on its own terms rather than through the
// repository root's config. The root itself is always included, because a
// project with no manifest at all still has conventional entry points.
func findECMAScriptProjectDirs(root string) []string {
	dirs := []string{""}
	seen := map[string]struct{}{"": {}}

	// Errors are deliberately swallowed: an unreadable corner of the tree
	// should cost us that corner's aliases, not the whole map.
	_ = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if current == root {
				return nil
			}
			name := strings.ToLower(entry.Name())
			if name == ".git" {
				return filepath.SkipDir
			}
			if _, excluded := excludedFolders[name]; excluded {
				return filepath.SkipDir
			}
			return nil
		}

		if !slices.Contains(projectManifestNames, entry.Name()) {
			return nil
		}
		relative, err := filepath.Rel(root, filepath.Dir(current))
		if err != nil {
			return nil
		}
		dir := filepath.ToSlash(relative)
		if dir == "." {
			dir = ""
		}
		if _, already := seen[dir]; already {
			return nil
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
		return nil
	})

	return dirs
}

// loadECMAScriptAliases reads compilerOptions.paths and baseUrl. A config that
// cannot be read or parsed yields no aliases rather than an error: a project
// whose tsconfig we do not understand should still get a map of its relative
// imports.
func loadECMAScriptAliases(root, dir string) []ecmaScriptAlias {
	var aliases []ecmaScriptAlias

	for _, name := range aliasConfigNames {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), name))
		if err != nil {
			continue
		}

		var config tsconfigFile
		// tsconfig is JSON with comments and trailing commas, which the standard
		// decoder rejects outright.
		if err := json.Unmarshal([]byte(normalizeJSONC(string(raw))), &config); err != nil {
			continue
		}

		// baseUrl is relative to the config file, and every path the graph is
		// keyed by is relative to the repository, so it is anchored here once.
		base := path.Join(dir, filepath.ToSlash(config.CompilerOptions.BaseURL))
		if base == "." || base == "/" {
			base = ""
		}

		for pattern, targets := range config.CompilerOptions.Paths {
			alias := ecmaScriptAlias{wildcard: strings.Contains(pattern, "*")}
			prefix, suffix, _ := strings.Cut(pattern, "*")
			alias.prefix = prefix
			alias.suffix = suffix
			for _, target := range targets {
				alias.targets = append(alias.targets, path.Join(base, filepath.ToSlash(target)))
			}
			if len(alias.targets) > 0 {
				aliases = append(aliases, alias)
			}
		}

		// A bare baseUrl makes every non-relative specifier resolvable against
		// it, which is how "components/button" works in a lot of projects.
		if config.CompilerOptions.BaseURL != "" {
			aliases = append(aliases, ecmaScriptAlias{
				wildcard: true,
				targets:  []string{path.Join(base, "*")},
			})
		}
	}

	return aliases
}

// expandAlias turns a non-relative specifier into the repository paths it could
// stand for. Longer, more specific prefixes are tried first so that a catch-all
// baseUrl never shadows an explicit mapping.
func (project ecmaScriptProject) expandAlias(specifier string) []string {
	if isRelativeSpecifier(specifier) || specifier == "" {
		return nil
	}

	var candidates []string
	for _, alias := range project.aliases {
		if !alias.wildcard {
			if specifier != alias.prefix {
				continue
			}
			candidates = append(candidates, alias.targets...)
			continue
		}
		if !strings.HasPrefix(specifier, alias.prefix) || !strings.HasSuffix(specifier, alias.suffix) {
			continue
		}
		matched := strings.TrimSuffix(strings.TrimPrefix(specifier, alias.prefix), alias.suffix)
		for _, target := range alias.targets {
			candidates = append(candidates, strings.Replace(target, "*", matched, 1))
		}
	}

	return candidates
}

// normalizeJSONC makes a tsconfig readable by encoding/json: comments become
// blanks and trailing commas are dropped.
func normalizeJSONC(source string) string {
	stripped := stripComments(source, commentSyntax{stringDelimiters: `"`})

	var out strings.Builder
	out.Grow(len(stripped))
	for index := 0; index < len(stripped); index++ {
		character := stripped[index]
		if character == ',' {
			next := index + 1
			for next < len(stripped) && isJSONSpace(stripped[next]) {
				next++
			}
			if next < len(stripped) && (stripped[next] == '}' || stripped[next] == ']') {
				continue
			}
		}
		out.WriteByte(character)
	}
	return out.String()
}

func isJSONSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

// --- Entry points ---------------------------------------------------------

type packageJSONFile struct {
	Main   string `json:"main"`
	Module string `json:"module"`
	Bin    any    `json:"bin"`
}

// Conventional entry points, checked when package.json does not name one. These
// are what a reader of an unfamiliar project opens first, which is exactly what
// "root" means on the map.
var ecmaScriptEntryCandidates = []string{
	"src/main.ts", "src/main.tsx", "src/main.js", "src/main.jsx",
	"src/index.ts", "src/index.tsx", "src/index.js", "src/index.jsx",
	"src/app.ts", "src/app.tsx", "src/App.svelte", "src/App.vue",
	"src/routes/+layout.svelte", "app/layout.tsx", "pages/_app.tsx",
	"index.ts", "index.js", "main.ts", "main.js", "server.js", "server.ts",
}

func markECMAScriptEntryPoints(root, dir string, graph *Graph) {
	for _, candidate := range packageJSONEntries(root, dir) {
		markRootNode(graph, NodeID(path.Join(dir, candidate)))
	}
	for _, candidate := range ecmaScriptEntryCandidates {
		markRootNode(graph, NodeID(path.Join(dir, candidate)))
	}
}

func packageJSONEntries(root, dir string) []string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), "package.json"))
	if err != nil {
		return nil
	}

	var manifest packageJSONFile
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}

	var entries []string
	for _, entry := range []string{manifest.Main, manifest.Module} {
		if entry == "" {
			continue
		}
		entries = append(entries, path.Clean(strings.TrimPrefix(filepath.ToSlash(entry), "./")))
	}

	switch bin := manifest.Bin.(type) {
	case string:
		entries = append(entries, path.Clean(strings.TrimPrefix(filepath.ToSlash(bin), "./")))
	case map[string]any:
		for _, value := range bin {
			if text, isText := value.(string); isText {
				entries = append(entries, path.Clean(strings.TrimPrefix(filepath.ToSlash(text), "./")))
			}
		}
	}

	return entries
}

// markRootNode flags a file as an entry point, if the scan actually found it.
func markRootNode(graph *Graph, id NodeID) {
	node, exists := graph.Nodes[id]
	if !exists || node.Kind != NodeKindFile {
		return
	}
	node.IsRoot = true
	graph.Nodes[id] = node
}

// repositoryNodeID converts an absolute scan path into the slash-separated,
// repository-relative identifier the graph is keyed by.
func repositoryNodeID(root, absolute string) (NodeID, bool) {
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", false
	}
	return NodeID(filepath.ToSlash(relative)), true
}
