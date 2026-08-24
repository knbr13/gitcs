# gitcs Architecture Notes

This document explains how `gitcs` works internally. It is written as study
notes, so it focuses on the moving parts, what each file owns, and why each
library is used.

## 1. What This Project Is

`gitcs` is a Go CLI that can be used in any Git project.

It currently has two main jobs:

1. Commit activity CLI
   - Scans one folder for Git repositories.
   - Counts commits by day.
   - Prints a terminal contribution heatmap.
   - Can also print progression and weekly bar charts.

2. Codemap UI
   - Runs from inside one Git repository.
   - Builds a graph of source files.
   - Detects Go file-to-file call connections.
   - Reads current Git changes.
   - Shows a browser UI with nodes, arrows, recent commits, activity bars, and
     previous-vs-now summaries.

There is no external backend, no database, no API key, and no model dependency.
Everything is local: filesystem, Git history, Go parsing, and a local web
server.

## 2. How To Run It

For the original commit heatmap:

```bash
go run . -path .
go run . -path . -bars
go run . -path . -progress
go run . -path . -bars -progress
```

For the browser codemap:

```bash
cd some-git-repo
gitcs map
```

During development in this repo, the browser assets need to exist first:

```bash
cd frontend
npm install
npm run build
cd ..
go run . map
```

## 3. High-Level Flow

### Commit Heatmap Flow

```text
main.go
  -> parse flags: -path, -email, -since, -until, -bars, -progress
  -> validate folder, email, and date range
  -> scan.go finds Git repositories under -path
  -> stats.go reads commits with go-git
  -> print.go renders the terminal output
```

Important idea: this mode can scan many repositories under one folder. It is
not limited to only the current repository.

### Browser Codemap Flow

```text
gitcs map
  -> main.go dispatches to runWebMap()
  -> map_web.go opens the current Git repository
  -> map_api.go builds a map snapshot
  -> frontend/dist is served by the local Go HTTP server
  -> frontend/src/App.svelte fetches /api/graph
  -> SvelteFlow renders nodes and arrows
  -> fsnotify watches files and pushes live updates through /events
```

Important idea: this mode is for the current repository. It uses
`git.PlainOpenWithOptions(... DetectDotGit: true)` so it still works if you run
it from a nested folder inside the repo.

## 4. Main Entry Point

File: `main.go`

`main()` decides which mode to run.

```text
go run .
  -> commit heatmap mode

go run . map
  -> browser codemap mode

```

The normal heatmap mode uses Go's standard `flag` package. The `map` command is
handled manually before flag parsing:

```go
if len(os.Args) > 1 && os.Args[1] == "map" {
    runMapCommand(os.Args[2:])
    return
}
```

That means `gitcs map` is a subcommand, while flags like `-path` belong to the
original heatmap command.

## 5. Commit Heatmap Files

| File | Responsibility |
| --- | --- |
| `main.go` | CLI entry point, flag parsing, validation, command dispatch |
| `scan.go` | Walks folders and finds directories containing `.git` |
| `stats.go` | Opens repos and counts commits in the requested date window |
| `print.go` | Prints heatmap, progression, and bar chart output |
| `utils.go` | Email validation, date parsing, path checks, Git email helpers |

### How Commit Counting Works

`scanGitFolders(path)` finds Git repositories.

`processRepos(repos, email, boundary)` loops through those repositories. Each
repository is processed in a goroutine, because each repo can be read
independently.

The result is a map:

```go
map[int]int
```

The key is `daysAgo`. The value is the number of commits on that day.

Example:

```text
0 -> commits today
1 -> commits yesterday
7 -> commits one week ago
```

`print.go` turns that map into terminal output.

## 6. Codemap Backend Files

| File | Responsibility |
| --- | --- |
| `map.go` | Shared map command logic, Git status reading, graph analysis setup |
| `map_web.go` | Local HTTP server, API routes, live file watching, browser opening |
| `map_api.go` | Builds the JSON payload used by the browser UI |
| `map_description.go` | Deterministic file and change summaries |
| `map_scan.go` | Finds supported source files and detects language by extension |
| `map_build.go` | Converts source files into graph nodes |
| `graph.go` | Core graph data structures: nodes and edges |
| `analyzer.go` | Generic analyzer interface |
| `go_analyzer.go` | Go-specific analyzer using Go AST parsing |
| `map_learning.go` | Shared Git change status and activity aggregation helpers |
| `map_assets_dev.go` | Serves `frontend/dist` from disk in development |
| `map_assets_embed.go` | Embeds `frontend/dist` into release binaries |

## 7. Graph Model

File: `graph.go`

The codemap is built from two basic objects:

```go
type Node struct {
    ID       NodeID
    Label    string
    Path     string
    Language string
    Kind     NodeKind
    IsRoot   bool
}

type Edge struct {
    From NodeID
    To   NodeID
    Kind EdgeKind
}
```

A node usually represents a source file.

An edge represents a connection between files. Right now the important edge is:

```go
EdgeKindCalls
```

That means one Go file calls a function that is defined in another Go file.

## 8. Source File Scanning

File: `map_scan.go`

`findSourceFiles(root)` walks the repository and keeps files with supported
source extensions:

- Go
- JavaScript / TypeScript
- Svelte / Vue
- Python
- Rust
- Java
- CSS / HTML
- and other common source file types

It skips folders that should not be analyzed, such as `.git`, `node_modules`,
`vendor`, `dist`, and similar generated/noisy directories.

This gives the map enough files to display frontend and backend code without
including lockfiles, ignored build output, or Git internals.

## 9. Go Code Analysis

File: `go_analyzer.go`

The Go analyzer uses the standard library packages:

```go
go/parser
go/ast
go/token
```

The flow is:

```text
buildGoFunctionIndex()
  -> parse every Go file
  -> record which file defines each top-level function

FindConnections()
  -> parse a Go file
  -> find simple function calls
  -> if a called function is defined in exactly one other file
  -> create an edge from caller file to target file
```

The analyzer intentionally skips ambiguous calls. For example, if two files
both define a function named `Start`, the analyzer does not guess which one was
called.

Current limitation: this is a lightweight analyzer. It understands simple
top-level Go functions and simple calls, but it is not a full Go type checker.

## 10. Git Changes And Summaries

Files: `map.go`, `map_api.go`, `map_description.go`

The codemap reads the current working tree with `go-git`:

```go
worktree.Status()
```

That gives changed files and status codes:

- added
- modified
- deleted
- renamed

For files that are part of the graph, `map_api.go` compares:

```text
previous = file content at Git HEAD
now      = file content on disk
```

Then it computes deterministic facts:

- additions
- deletions
- first changed line
- touched Go symbols
- previous top-level symbols
- current top-level symbols
- incoming/outgoing graph connections

`map_description.go` turns those facts into the four summary sections shown in
the UI:

- Previously
- Now
- Changed
- Possible impact

This is deliberately not AI-generated. The summary describes local evidence and
does not guess product intent.

## 11. Browser Server

File: `map_web.go`

`gitcs map` starts a local server at:

```text
http://127.0.0.1:7331
```

Routes:

| Route | Purpose |
| --- | --- |
| `GET /` | Serves the built Svelte app |
| `GET /api/graph` | Returns the current map snapshot as JSON |
| `POST /api/open` | Opens a selected file in VS Code |
| `GET /events` | Server-Sent Events stream for live updates |

The server watches the repository with `fsnotify`. When files change, it
rebuilds the map snapshot, increments a revision number, and broadcasts an
event to the browser.

The browser then refetches `/api/graph`.

## 12. Frontend

Folder: `frontend/`

The frontend is a Svelte app built with Vite.

| File | Responsibility |
| --- | --- |
| `frontend/src/main.js` | Mounts the Svelte app |
| `frontend/src/App.svelte` | Main dashboard: toolbar, graph, side panel, timeline |
| `frontend/src/CodeCard.svelte` | Individual file/node card shown inside the graph |
| `frontend/vite.config.js` | Vite build config |
| `frontend/package.json` | Frontend dependencies and scripts |

Main frontend libraries:

- `svelte`: UI framework.
- `vite`: development/build tool.
- `@xyflow/svelte`: graph canvas used for nodes, arrows, zoom, pan, and fit view.

The frontend does not read Git directly. It only talks to the local Go server:

```text
GET /api/graph
POST /api/open
GET /events
```

## 13. Browser UI State

File: `frontend/src/App.svelte`

Important state:

```js
let graph = null;
let nodes = [];
let edges = [];
let selectedId = '';
let view = 'changes';
let scope = 'all';
let period = '30';
```

The API returns raw repository data. The frontend transforms it into the shape
required by SvelteFlow.

Important frontend functions:

| Function | Purpose |
| --- | --- |
| `loadGraph()` | Fetches `/api/graph` |
| `renderGraph()` | Converts API nodes/edges into SvelteFlow nodes/edges |
| `visibleGraph()` | Chooses which graph to show based on selected mode |
| `reviewGraph()` | Shows changed files plus important connected files |
| `buildScopedGraph()` | Adds Frontend/Backend parent nodes and scope filtering |
| `layoutNodes()` | Gives every node an x/y position |
| `openNode()` | Calls `/api/open` to open a file in VS Code |

The frontend creates the visual grouping for Frontend and Backend. The backend
does not currently store those as real graph nodes.

## 14. Build And Release

The Makefile has release-oriented tasks:

```bash
make frontend
make build
make compress
make all
```

Important detail:

```bash
go build -tags webembed
```

The `webembed` build tag switches from `map_assets_dev.go` to
`map_assets_embed.go`.

In development:

```text
map_assets_dev.go -> reads frontend/dist from disk
```

In release binaries:

```text
map_assets_embed.go -> embeds frontend/dist into the Go binary
```

That is what makes it possible for `gitcs map` to work after installation
without shipping a separate frontend folder.

## 15. Dependencies

### Go dependencies

| Dependency | Used for |
| --- | --- |
| `github.com/go-git/go-git/v5` | Reading repositories, commits, HEAD trees, and working-tree status |
| `github.com/sergi/go-diff` | Computing line-level diff facts for changed files |
| `github.com/fsnotify/fsnotify` | Watching file changes for live browser updates |
| `github.com/briandowns/spinner` | Loading spinner in heatmap mode |
| `github.com/gookit/color` | Terminal colors for CLI output/errors |

### Frontend dependencies

| Dependency | Used for |
| --- | --- |
| `svelte` | UI components and reactivity |
| `vite` | Frontend build tool |
| `@xyflow/svelte` | Interactive graph rendering |

### Standard Library Pieces Worth Learning

| Package | Used for |
| --- | --- |
| `flag` | CLI flags |
| `net/http` | Local browser server |
| `encoding/json` | API responses |
| `embed` | Embedding frontend assets into release binaries |
| `path/filepath` | Cross-platform file paths |
| `io/fs` | Filesystem walking and embedded files |
| `sync` | Mutexes, wait groups, shared state |
| `os/exec` | Opening browser / VS Code |
| `go/parser`, `go/ast`, `go/token` | Parsing Go source code |

## 16. What Is Deterministic vs What Is Guessed

Deterministic facts:

- Git commit counts.
- File status from `git status`.
- Previous content from HEAD.
- Current content from disk.
- Additions/deletions from diff.
- Top-level Go symbols from AST parsing.
- Simple Go function-call connections.

Things the project avoids guessing:

- Business/product intent.
- Whether a change is good or bad.
- Runtime behavior that cannot be proven from static source.
- Connections in languages that do not yet have analyzers.

## 17. Current Limitations

- Only Go files get rich symbol and call analysis.
- Frontend/backend categorization is currently done in the browser from file
  path and language.
- The browser layout is custom/manual, not a full graph layout engine.
- The change summary is per file, not a full project narrative.
- The open-in-editor behavior currently targets VS Code.
- The local browser server uses a fixed port: `127.0.0.1:7331`.

## 18. Suggested Reading Order

To understand the project without getting lost:

1. `main.go`
2. `scan.go`, `stats.go`, `print.go`
3. `map.go`
4. `graph.go`
5. `map_scan.go`, `map_build.go`
6. `go_analyzer.go`
7. `map_api.go`
8. `map_description.go`
9. `map_web.go`
10. `frontend/src/App.svelte`
11. `frontend/src/CodeCard.svelte`
12. `map_learning.go`

## 19. Mental Model

The easiest way to think about the project:

```text
Git repo on disk
  -> scan source files
  -> build graph nodes
  -> parse Go files for call edges
  -> read Git status and commit history
  -> enrich graph with activity and change summaries
  -> serve JSON to Svelte
  -> SvelteFlow draws the map
```

The CLI is the product surface. Go owns the real analysis. Svelte owns the
visual experience.
