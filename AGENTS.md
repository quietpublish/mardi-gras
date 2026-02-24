# Repository Guidelines

## Project Structure & Module Organization

- `cmd/mg/main.go`: CLI entrypoint, `--path`/`--block-types`/`--status`/`--version` flag handling.
- `internal/app`: BubbleTea root model, key routing, pane orchestration, confetti animation.
- `internal/views`: Parade (left pane), Detail (right pane), Gas Town panel, Problems overlay.
- `internal/components`: Header, Footer, Help overlay, Command palette, Toast notifications, Create form, Float utility.
- `internal/data`: JSONL loading, grouping, dependency/status logic, filtering, focus mode, mutations (`bd` CLI), cross-rig deps, HOP types.
- `internal/gastown`: Gas Town integration — environment detection, `gt status` parsing, sling/nudge/handoff/decommission, convoy CRUD, mail inbox/reply/compose, molecule DAG, costs, activity feed, velocity, scorecards, predictions, formula recommendations.
- `internal/agent`: Claude Code prompt builder, tmux window launch/discover/kill.
- `internal/tmux`: tmux status line widget (`mg --status` mode).
- `internal/ui`: Theme palette (with Gas Town role/state colors), Lipgloss styles, Unicode symbols (including DAG connectors), HOP badge rendering.
- `testdata/sample.jsonl`: fixture for tests and local demo runs.
- `docs/`: Architecture docs, internal design docs, screenshots.

## Beads Data Contract

- Treat `.beads/issues.jsonl` as the source of truth; do not rely on `.beads/.beads.db`.
- Parse JSONL line-by-line and keep reads safe while Beads is running.
- Preserve status semantics: `in_progress` -> Rolling, `open` unblocked -> Lined Up, `open` blocked -> Stalled, `closed` -> Past the Stand.
- Eight dependency types: `blocks`, `blocked-by`, `related`, `duplicates`, `supersedes`, `parent-child`, `discovered-from`, `depends-on`. The `--block-types` flag controls which count as blockers (default: `blocks`).
- Optimize for real-world closed-heavy datasets; closed issues should remain collapsible and low-noise by default.
- Mutations go through `bd` CLI (`bd update`, `bd close`, `bd create`). Never use `bd edit` — it opens `$EDITOR` and blocks agents.

## Gas Town Integration

- Gas Town features activate progressively: Beads-only (no `gt`) -> Gas Town available (`gt` on PATH) -> Inside Gas Town (`GT_ROLE` env var set). Every feature must work or hide gracefully at each level.
- `gt status --json` takes ~9 seconds. Always run as a BubbleTea `Cmd` (background goroutine), never blocking Update. Handle `nil` status gracefully — the user may interact before the command returns.
- The JSON nests agents under `rigs[].agents`. `normalizeStatus()` in `gastown/status.go` flattens them. Top-level agents are HQ-level (mayor, deacon); rig agents include polecats, crew, witness, refinery.
- If `AgentRuntime.State` is empty, default to "idle". Gas Town v0.8.0+ always provides State.
- Gas Town rig names cannot contain hyphens (use underscores).
- Crew workspaces have `.beads/redirect` not `issues.jsonl` — mg walks up the directory tree to find the actual data file.
- The core `gastown` package (status, sling, convoy, mail, molecule, problems, detect) has no internal dependencies. Analytics files (velocity, predict, scorecard, recommend) import `internal/data` for issue types.

## Build, Test, and Development Commands

- `make build`: build local binary `./mg` from `./cmd/mg`.
- `make run`: build and run using auto-detected `.beads/issues.jsonl`.
- `make run-sample` (or `make dev`): run against `testdata/sample.jsonl`.
- `make test`: execute `go test ./...` across all packages.
- `make fmt`: apply standard Go formatting (`go fmt ./...`).
- `make lint`: run static analysis with `golangci-lint run ./...`.
- `make tidy`: sync module dependencies in `go.mod`/`go.sum`.

To test Gas Town features, run mg from a Gas Town workspace: `cd ~/gt/<rig>/crew/<name> && ~/path/to/mg`.

## Coding Style & Naming Conventions

- Use idiomatic Go and always format with `make fmt` before committing.
- Keep package boundaries domain-based. Prefer expanding existing packages over creating new ones.
- Exported names use `PascalCase`; unexported helpers use `camelCase`.
- **Value receivers** on BubbleTea models (`Update`, `View`); **pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`).
- **UI constants** live in `internal/ui/` — colors in `theme.go`, symbols in `symbols.go`, styles in `styles.go`. Don't scatter raw colors or symbols in view code.
- Keep Mardi Gras UI vocabulary and section labels consistent (`ROLLING`, `LINED UP`, `STALLED`, `PAST THE STAND`).
- If keybindings change, update `components/help.go` (the in-app help overlay) and the README keybinding tables in the same PR.

## Testing Guidelines

- Put tests next to implementation as `*_test.go`.
- Name tests `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants.
- Prefer deterministic tests using fixtures from `testdata/`.
- Run `make test` for all changes; run `make dev` to verify TUI behavior visually.
- Gas Town integration tests may need a live `gt` environment. Mark those clearly or mock the CLI output.

## Commit & Pull Request Guidelines

- Follow Conventional Commit style: `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
- Keep each commit focused on one logical change.
- PRs should include a short summary, validation steps run (commands), and screenshots/GIFs for visible TUI updates.
- Link related issues and call out any follow-up work or known limitations.
- Create feature branches off `main` — the `main` branch is protected.
