# Contributing to Mardi Gras

Thanks for your interest in making the parade better! This guide covers everything you need to get started.

## Prerequisites

- **Go 1.25+** ([install](https://go.dev/doc/install)) — `go.mod` declares `go 1.25.0`, and CI builds on 1.25.x
- **Git**
- **golangci-lint** for linting ([install](https://golangci-lint.run/welcome/install/)) — CI pins v2.11
- A Beads project, or use the included `testdata/sample.jsonl`

Optional (for orchestrator features — mg works fine without any of these):
- **[Gas Town](https://github.com/gastownhall/gastown)** (`gt`) on PATH — enables agent orchestration, convoys, mail
- **[Gas City](https://github.com/gastownhall/gascity)** (`gc`) — an orchestration-builder SDK from the same org; mg speaks its Supervisor HTTP API as an alternative backend
- A Gas Town workspace (rig + crew) for full integration testing

## Getting Started

```bash
git clone https://github.com/quietpublish/mardi-gras.git
cd mardi-gras
make build
```

Run against the included sample data:

```bash
make dev
```

This builds the `mg` binary and launches it with `testdata/sample.jsonl`.

### Testing with an orchestrator

To test Gas Town features locally without a live workspace, use the fake `gt` script:

```bash
make dev-gt
```

This puts `testdata/fake-gt.sh` on PATH, providing canned responses for `gt status`, `gt vitals`, `gt costs`, `gt convoy list`, and `gt mail inbox`. Press `ctrl+g` to open the Gas Town panel with full sample data.

The Gas City path has an HTTP analogue — a fake supervisor in `testdata/fakegc`, no `gc` install needed:

```bash
make dev-gc
```

For full integration testing against a real Gas Town environment:

```bash
cd ~/gt/<rig>/crew/<name>
~/path/to/mardi-gras/mg
```

Without an orchestrator, those features are hidden and mg works as a standalone Beads viewer.

## Development Commands

```bash
make build        # compile the mg binary
make run          # build + run (auto-detects the Beads source)
make dev          # build + run with sample data
make dev-gt       # build + run with sample data and fake gt (Gas Town features)
make dev-gc       # build + run against a fake Gas City supervisor
make test         # go test ./...
make lint         # golangci-lint run ./...
make fmt          # go fmt ./...
make tidy         # go mod tidy
make clean        # remove binary and dist/
make gc-client    # regenerate internal/gastown/gcclient from the pinned OpenAPI spec
```

CI is stricter than `make test`. It runs `go build ./...`, `go vet ./...`, `go test -race` with a **55% coverage floor**, `golangci-lint` with the same `.golangci.yml` config, `govulncheck`, and a `go.sum` drift check (`go mod tidy` must be a no-op) — on both Linux and macOS. Run `make test && make lint && go vet ./...` locally before pushing.

`internal/gastown/gcclient` is generated code. Change the pinned `openapi.json` and re-run `make gc-client` (needs `jq` and `oapi-codegen`) rather than hand-editing `client_gen.go`.

## Project Structure

```
cmd/mg/main.go        Entry point (flags, path resolution, bootstrap)

internal/
  app/                Root BubbleTea model (lifecycle, routing, layout)
  data/               Domain types, bd CLI + JSONL loading, filtering, file watcher
  views/              Parade, Detail, Gas Town panel, Problems overlay, bd doctor, Codex transcript
  components/         Header, Footer, Help, Command palette, Toast, Create/Edit forms, dialogs
  agent/              Agent runtime detection (Claude Code, Cursor, Codex) and tmux dispatch
  gastown/            Orchestrator integration behind the Driver seam (Gas Town CLI + Gas City HTTP)
    gcclient/         GENERATED Gas City API client — see `make gc-client`
  codexmcp/           JSON-RPC client for `codex mcp-server` (transport, session, protocol)
  tmux/               tmux status line widget (--status mode)
  ui/                 Theme colors, lipgloss styles, Unicode symbols, gradients, sparklines

testdata/             Sample JSONL, fake gt script, fake Gas City supervisor, vhs tapes
docs/                 Architecture and integration docs, screenshots
```

`docs/internal/` is gitignored — it holds local-only working notes and is never committed.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a deeper walkthrough of the data flow, BubbleTea model structure, package dependencies, and Gas Town integration.

## How to Contribute

### Reporting Bugs

Open a GitHub issue with:

- What you expected vs. what happened
- Steps to reproduce
- Terminal emulator and OS
- Output of `mg --version`
- Which orchestrator was active, if any (`gt` on PATH, `GT_ROLE` set, `gc` on PATH, or `MG_GC_API` set)

### Suggesting Features

Open an issue describing the feature and why it would be useful. The [README](README.md#possible-future-ideas) lists some ideas we've been thinking about.

### Submitting a Pull Request

1. Fork the repo and create a branch from `main`.
2. Make your changes.
3. Add or update tests if the change affects behavior.
4. Run `make fmt && make lint && make test && go vet ./...` and fix any issues.
5. Write clear commit messages that explain the *why*, not just the *what*. Conventional Commit prefixes: `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
6. Open a PR against `main`. PRs land via squash merge, so the PR title becomes the commit subject on `main`.

Keep PRs focused — one feature or fix per PR makes review faster for everyone.

**Don't edit `CHANGELOG.md` in your PR.** Entries are batched into a separate `chore: prep vX.Y.Z (changelog)` commit at release time — a per-PR edit just creates a conflict.

## Code Conventions

### General

- **Formatting**: `gofmt` (enforced by CI).
- **Linting**: golangci-lint with the config in `.golangci.yml`.
- **Naming**: follow standard Go conventions. Exported names should be clear without a package prefix.
- **Errors**: return errors rather than panicking. Use `fmt.Errorf` with `%w` for wrapping.
- **Tests**: `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants. Table-driven tests where appropriate. Test files live alongside the code they test.
- **Dependencies**: Mardi Gras intentionally has a small dependency footprint — the Charmbracelet toolkit (bubbletea/bubbles/lipgloss/glamour/ultraviolet), plus a handful of small direct deps (`atotto/clipboard`, `sahilm/fuzzy`, `muesli/termenv`, `yuin/goldmark`, `oapi-codegen/runtime` for the generated Gas City client). Propose new dependencies in the PR description with a rationale.

### Receivers

- **Value receivers** on BubbleTea models (`Update`, `View`) — required by the Elm architecture.
- **Pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`) — internal state updates that don't return a new model.

### UI Constants

All visual constants live in `internal/ui/`:

- Colors in `theme.go` — includes `RoleColor()` and `AgentStateColor()` for Gas Town
- Styles in `styles.go` — pre-built lipgloss styles for every view context
- Symbols in `symbols.go` — Unicode characters for status, dependencies, DAG connectors

**Don't scatter raw colors or symbols in view code.** If you need a new color or symbol, add it to the appropriate `ui/` file and reference it from the view.

### Package Boundaries

- `data` and `ui` have no internal dependencies beyond stdlib and lipgloss — keep them that way.
- Core `gastown` files (status, sling, convoy, mail, molecule, problems, recovery, detect) have no internal dependencies. Analytics files (velocity, predict, scorecard, recommend) import `internal/data` for issue types.
- No package imports `app` — it is the root.
- **Prefer expanding existing packages** over creating new ones. New packages need a clear reason.

## Architecture Notes

Mardi Gras follows the [Elm Architecture](https://guide.elm-lang.org/architecture/) via BubbleTea:

- **Model** holds all state in `app.Model`.
- **Update** routes messages (key presses, file changes, agent events, Gas Town results) to handlers.
- **View** composes sub-models (parade, detail/gastown, header, footer) into the final screen, with overlays (help, palette, toast, create form) layered on top.

Key design constraints:

- Single binary, no runtime dependencies. Cross-compiles via GoReleaser.
- **Beads is the substrate; orchestration is a layer.** mg always needs Beads. Gas Town and Gas City sit behind the `gastown.Driver` interface, and an operation a backend can't serve returns `ErrUnsupported` so the caller hides the feature rather than erroring.
- **Graceful degradation**: features activate progressively. Beads-only (no orchestrator) → orchestrator available → inside a Gas Town session (`GT_ROLE` set). Every feature must work or hide gracefully at each level.
- **Async caution**: `gt status --json` latency is highly variable — seconds to tens of seconds, depending on rig/agent count and whether dolt and the daemon are running. Any `exec.Command` call to `gt`, and any HTTP call to a Gas City supervisor, should run as a BubbleTea `Cmd` (background goroutine), never blocking the main Update loop. Always handle `nil` status gracefully, and gate re-polls behind an in-flight check so an unreachable orchestrator can't spin.

## Known Gotchas

- `gt status --json` nests agents under `rigs[].agents`, not flat at top level. `normalizeStatus()` in `gastown/status.go` flattens them.
- Gas Town rig names cannot contain hyphens (use underscores).
- Crew workspaces have `.beads/redirect` not `issues.jsonl` — mg walks up the directory tree to find the actual data file.
- `bd edit` opens `$EDITOR` and blocks — never use it from agents or tests. Use `bd update` for field changes.
- **Driver selection is evidence-based, not a simple env flag.** `SelectDriver()` in `gastown/gc.go` prefers Gas City when `MG_GC_API` is set, otherwise Gas Town whenever there is *any* Gas Town evidence (`GT_*` env vars or `gt` on PATH), otherwise Gas City when `gc` is on PATH or a `city.toml` sits above the cwd, and Gas Town as the final default. If you have `gt` installed you will always get the Gas Town driver unless you set `MG_GC_API`.
- The UI palette is theme-switchable and rebaked at startup. Never capture palette values, styles, or pre-rendered strings in package-level vars outside `internal/ui` — they freeze the dark theme before `ui.SetTheme` runs.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
