# gitcs

[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)  

Git Commits Visualizer (`gitcs` shortly) is a command-line tool that allows developers to analyze their local Git repositories and generate a visual contributions graph. This tool proves valuable for developers working across multiple Git services like GitHub and GitLab (because there are already graphs provided online by each of them, but each has it's own data, this tool works locally, so no matter where you've pushed the project, commits will count), enabling them to visualize contributions seamlessly, even in offline or disconnected environments.

![gitcs](./gitcs.gif)

## Installation

### Download the CLI (recommended)

Download a binary for your system from [this fork's releases](https://github.com/roshan-ican/gitcs/releases). Release binaries include the map UI; Go and Node are not required to run them. Git is needed for repository operations.

Choose `windows_amd64` for most Windows PCs, `darwin_arm64` for Apple Silicon Macs, `darwin_amd64` for Intel Macs, or the matching Linux architecture. Rename the download to `gitcs.exe` (Windows) or `gitcs` (macOS/Linux), and place it in a directory on your `PATH`. On macOS/Linux, also run `chmod +x gitcs`. Binaries are currently unsigned, so your OS may show a security warning.

Then open a terminal in any Git repository and run `gitcs map`.

### Build and install from this checkout

With Go and Node installed, run these commands from the repository root:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
go install -tags webembed .
```

Add Go's binary directory (`go env GOPATH`, followed by `/bin`, unless `GOBIN` is set) to your `PATH`. You can then run `gitcs` from any directory.

The module still uses the upstream path `github.com/hrtsegv/gitcs`. Running `go install github.com/hrtsegv/gitcs@latest` installs upstream, not this fork, and does not bundle this fork's map UI. Use a release binary or the checkout instructions above for this version.

### Publish a release (maintainers)

Commit the desired code and release workflow, then push that branch to `roshan-ican/gitcs`. Create a new, unused version tag on that commit and push it, for example:

```sh
git tag v0.2.0
git push origin v0.2.0
```

Pushing a `v*` tag starts the **Create Release** workflow. It builds the frontend and standalone Windows, macOS, and Linux binaries, then publishes them with SHA-256 checksums using GitHub's built-in token (no `RELEASE_TOKEN` secret needed). Choose a different version if the example tag already exists. The manual workflow accepts an existing tag and builds that exact tag.

## Usage

### Live repository map

Run `gitcs map` inside a Git repository, or in a parent folder containing several repositories (for example, `lumah_v1` containing `frontend` and `lomah-nest`). The parent-folder view combines the repositories into one live map with repository-prefixed file paths and separate Git history and changes. Dependency and build folders are skipped. Discovery stops at each repository boundary; nested repositories inside an already selected repository are not mapped separately.

Build the browser UI once during development, then start the local map:

```bash
cd frontend && npm install && npm run build
cd ..
go run . map
```

The map is served only on `127.0.0.1:7331`. Release builds produced by
`make build` embed the browser UI, so Node is not required on the machine
running `gitcs`. The browser starts on **Architecture**, grouping source files
into projects and modules with static import/call connections. Select a module
and choose **View files** to see its files and direct neighbors (marked
**Related**); use **Architecture** to return. Select a module connection to
inspect the file evidence behind it. Changing, Calls, and Everything remain
available as file views.

Project filters use actual folders and manifests (`package.json`, `go.mod`,
and `Cargo.toml`), not programming languages: a TypeScript backend stays in its
own project. The first folder beneath each project forms a module, with `src/`
treated as a container and root-level files grouped as **Root files**.

Each project with tests has one collapsed **Tests** group. Detection recognizes
Go `_test.go`, JS/TS `.test.*` and `.spec.*`, Python `test_*.py` and `*_test.py`,
and files within `test/`, `tests/`, or `__tests__/`. Inline tests remain part of
their source file. Migrations stay grouped under their containing module.
Arrows describe static dependencies, not runtime execution or test coverage;
no test runner, AI service, or source upload is involved.

#### Supported languages

`gitcs map` draws connections for:

| Language | What it reads |
| --- | --- |
| Go | Top-level function definitions and calls between files |
| JavaScript / TypeScript | `import`, `export ... from`, `import()`, `require()` — including `.jsx`, `.tsx`, `.mjs`, `.cjs` |
| Svelte / Vue | Imports inside `<script>` blocks |
| Rust | `mod` declarations and `use` paths, across a Cargo workspace |

Module resolution follows each ecosystem's own rules, so it works on real
projects rather than only tidy ones: extensionless imports and directory
`index` files resolve, TypeScript's `./thing.js`-means-`thing.ts` convention is
understood, and `tsconfig.json`/`jsconfig.json` path aliases (`@/components/…`)
are read. Every directory with a `package.json` or `tsconfig.json` counts as its
own project, so nested clients and monorepo packages each get their own aliases
and entry points.

Imports of published packages produce no connection — only files that are
actually in the repository become nodes. Files in other languages still appear
on the map with their Git history; they simply have no connections drawn.

The basic usage of this tool is to just run it, it will generate a graph of commits from the last 6 months.
```bash
> gitcs -path "/home/user/dev"
```

These commits are committed by your global Git email address, but you can also use the -email flag to show commits for another Git email.
```bash
> gitcs -email "email@example.com" -path "/home/user/dev"
```

If you want to include commits from **all** authors (regardless of email), pass a wildcard:
```bash
> gitcs -email "*" -path "/home/user/dev"
```

If you want to see how the total work progressed over time, add the `-progress` flag:
```bash
> gitcs -path "/home/user/dev" -progress
```

If you want a weekly bar chart of commit history, add the `-bars` flag:
```bash
> gitcs -path "/home/user/dev" -bars
```

By default, the tool displays commits from the last 6 months, but you can configure this using the `since` and `until` flags.
```bash
> gitcs -since "2023-10-24" -until "2024-01-15" -path "/home/user/dev"
```

- If no global Git email is set on your machine, then you have to specify it using the `-email` flag.
- The since and until flags don't need to be specified together.

## Contributions

Contributions are welcome! If you would like to contribute to this project, please follow these steps:

1- Fork the repository.

2- Create a new branch for your feature or bug fix.

3- Make the necessary changes and commit them.

4- Push your changes to your fork.

5- Submit a pull request describing your changes.

## License

This project is licensed under the [MIT License](https://github.com/hrtsegv/gitcs/blob/main/LICENSE). See the [LICENSE](https://github.com/hrtsegv/gitcs/blob/main/LICENSE) file for details.
