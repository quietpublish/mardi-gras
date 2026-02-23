# Contributing to Mardi Gras

Thanks for your interest in making the parade better! This guide covers everything you need to get started.

## Prerequisites

- **Go 1.24+** ([install](https://go.dev/doc/install))
- **Git**
- **golangci-lint** for linting ([install](https://golangci-lint.run/welcome/install/))
- A Beads project, or use the included `testdata/sample.jsonl`

Optional (for Gas Town features):
- **Gas Town** (`gt`) on PATH — enables agent orchestration, convoys, mail
- A Gas Town workspace (rig + crew) for full integration testing

## Getting Started

```bash
git clone https://github.com/matt-wright86/mardi-gras.git
cd mardi-gras
make build
```

Run against the included sample data:

```bash
make dev
```

This builds the `mg` binary and launches it with `testdata/sample.jsonl`.

### Testing with Gas Town

To test Gas Town features, run `mg` from inside a Gas Town workspace:

```bash
cd ~/gt/<rig>/crew/<name>
~/path/to/mardi-gras/mg
```

The Gas Town panel (`ctrl+g`), sling/nudge, convoys, and mail all require a live `gt` environment. Without it, those features are hidden and mg works as a standalone Beads viewer.

## Development Commands

```bash
make build        # compile the mg binary
make run          # build + run (auto-detects .beads/issues.jsonl)
make dev          # build + run with sample data
make test         # go test ./...
make lint         # golangci-lint run ./...
make fmt          # go fmt ./...
make tidy         # go mod tidy
make clean        # remove binary and dist/
```

CI runs tests with `-race` and lints with the same `.golangci.yml` config, so run `make test` and `make lint` locally before pushing.

## Project Structure

```
cmd/mg/main.go        Entry point (flags, path resolution, bootstrap)

internal/
  app/                Root BubbleTea model (lifecycle, routing, layout)
  data/               Domain types, JSONL parsing, filtering, file watcher
  views/              Parade, Detail, Gas Town panel, Problems overlay
  components/         Header, Footer, Help, Command palette, Toast, Create form
  agent/              Claude Code integration and tmux dispatch
  gastown/            Gas Town integration (status, sling, convoy, mail, analytics)
  tmux/               tmux status line widget (--status mode)
  ui/                 Theme colors, lipgloss styles, Unicode symbols, HOP badges

testdata/             Sample JSONL for development
docs/                 Architecture docs, internal design docs, screenshots
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a deeper walkthrough of the data flow, BubbleTea model structure, package dependencies, and Gas Town integration.

## How to Contribute

### Reporting Bugs

Open a GitHub issue with:

- What you expected vs. what happened
- Steps to reproduce
- Terminal emulator and OS
- Output of `mg --version`
- Whether Gas Town was active (`gt` on PATH, `GT_ROLE` env var)

### Suggesting Features

Open an issue describing the feature and why it would be useful. The [README](README.md#possible-future-ideas) lists some ideas we've been thinking about.

### Submitting a Pull Request

1. Fork the repo and create a branch from `main`.
2. Make your changes.
3. Add or update tests if the change affects behavior.
4. Run `make fmt && make lint && make test` and fix any issues.
5. Write clear commit messages that explain the *why*, not just the *what*.
6. Open a PR against `main`.

Keep PRs focused — one feature or fix per PR makes review faster for everyone.

## Code Conventions

### General

- **Formatting**: `gofmt` (enforced by CI).
- **Linting**: golangci-lint with the config in `.golangci.yml`.
- **Naming**: follow standard Go conventions. Exported names should be clear without a package prefix.
- **Errors**: return errors rather than panicking. Use `fmt.Errorf` with `%w` for wrapping.
- **Tests**: `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants. Table-driven tests where appropriate. Test files live alongside the code they test.
- **Dependencies**: Mardi Gras intentionally has a small dependency footprint (Charmbracelet toolkit + clipboard). Propose new dependencies in the PR description with a rationale.

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
- `gastown` has no internal dependencies — stdlib and `encoding/json` only.
- No package imports `app` — it is the root.
- **Prefer expanding existing packages** over creating new ones. New packages need a clear reason.

## Architecture Notes

Mardi Gras follows the [Elm Architecture](https://guide.elm-lang.org/architecture/) via BubbleTea:

- **Model** holds all state in `app.Model`.
- **Update** routes messages (key presses, file changes, agent events, Gas Town results) to handlers.
- **View** composes sub-models (parade, detail/gastown, header, footer) into the final screen, with overlays (help, palette, toast, create form) layered on top.

Key design constraints:

- Single binary, no runtime dependencies. Cross-compiles via GoReleaser.
- **Graceful degradation**: features activate progressively. Beads-only (no `gt`) → Gas Town available (`gt` on PATH) → Inside Gas Town (`GT_ROLE` set). Every feature must work or hide gracefully at each level.
- **Async caution**: `gt status --json` takes ~9 seconds. Any `exec.Command` call to `gt` should run as a BubbleTea `Cmd` (background goroutine), never blocking the main Update loop. Always handle `nil` status gracefully — the user may interact before the command returns.

## Known Gotchas

- `gt status --json` nests agents under `rigs[].agents`, not flat at top level. `normalizeStatus()` in `gastown/status.go` flattens them.
- Gas Town rig names cannot contain hyphens (use underscores).
- Crew workspaces have `.beads/redirect` not `issues.jsonl` — mg walks up the directory tree to find the actual data file.
- `bd edit` opens `$EDITOR` and blocks — never use it from agents or tests. Use `bd update` for field changes.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
