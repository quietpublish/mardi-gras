# Repository Guidelines

## Project Structure & Module Organization

- `cmd/mg/main.go`: CLI entrypoint. Flags: `--path`, `--block-types`, `--exclude-type`, `--exclude-label`, `--status`, `--version`, `--theme`, `--agent`, `--cmd-timeout`, `--no-animations`.
- `internal/app`: BubbleTea root model, key routing, pane orchestration, confetti animation.
- `internal/views`: Parade (left pane), Detail (right pane), Gas Town panel, Problems overlay, `bd doctor` overlay, Codex transcript.
- `internal/components`: Header, Footer, Help overlay, Command palette, Toast notifications, Create/Edit forms, Approval + Recovery dialogs, Float utility.
- `internal/data`: Issue loading (`bd list --json`, JSONL fallback), grouping, dependency/status logic, filtering, focus mode, mutations (`bd` CLI), cross-rig deps, source health.
- `internal/gastown`: Orchestrator integration behind a `Driver` seam — `GTDriver` (`gt` CLI) and `GCDriver` (Gas City Supervisor HTTP API), picked by `SelectDriver()`. Covers environment detection, `gt status` parsing, sling/nudge/handoff/decommission, assignment, convoy CRUD, mail inbox/reply/compose, molecule DAG, costs, vitals (server health + backups), activity feed, velocity, scorecards, predictions, formula recommendations, problem detection (stalled/stuck/backoff/zombie/dead_rig), patrol scan integration, rig recovery. `gcclient/` is generated — regenerate with `make gc-client`, never hand-edit.
- `internal/agent`: Agent prompt builder and runtime detection (`claude`, `cursor-agent`, `codex`), tmux pane launch/discover/kill.
- `internal/codexmcp`: JSON-RPC client for `codex mcp-server` (transport, session, protocol types) behind the live Codex transcript and approval routing.
- `internal/tmux`: tmux status line widget (`mg --status` mode).
- `internal/ui`: Theme palette (with Gas Town role/state colors), Lipgloss styles, Unicode symbols (including DAG connectors), gradients, sparklines.
- `testdata/sample.jsonl`: fixture for tests and local demo runs.
- `testdata/fake-gt.sh`: fake `gt` binary for local Gas Town testing (`make dev-gt`).
- `testdata/fakegc/`: fake Gas City supervisor for local Gas City testing (`make dev-gc`).
- `docs/`: Architecture and integration docs, screenshots. `docs/internal/` is gitignored and local-only — never commit it.

## Beads Data Contract

- `bd list --json` (`SourceCLI`) is the preferred source; `.beads/issues.jsonl` (`SourceJSONL`) is the legacy fallback, used when `--path` is given or `bd` is not on PATH. `resolveSource()` in `cmd/mg/main.go` decides. Never read `.beads/.beads.db` directly.
- Parse JSONL line-by-line and keep reads safe while Beads is running. Malformed lines are skipped, not fatal.
- Preserve status semantics: `in_progress` -> Rolling, `open` unblocked -> Lined Up, `open` blocked -> Stalled, `closed` -> Past the Stand.
- Blocking dependency types default to `blocks` **and** `conditional-blocks` (`data.DefaultBlockingTypes`). Override with `--block-types` or `MG_BLOCK_TYPES`.
- Non-blocking types mg renders explicitly (`depTypeDisplay` in `views/detail.go`): `related`, `duplicates`, `supersedes`, `discovered-from`, `waits-for`, `parent-child`, `replies-to`. Any other type falls through to a generic rendering, so an unknown type from bd degrades rather than breaking.
- Optimize for real-world closed-heavy datasets; closed issues should remain collapsible and low-noise by default.
- Mutations go through `bd` CLI (`bd update`, `bd close`, `bd create`). Never use `bd edit` — it opens `$EDITOR` and blocks agents.

## Orchestrator Integration (Gas Town / Gas City)

- Orchestration is optional. Beads is the substrate mg always requires; Gas Town (`gt`) and Gas City (`gc`) are alternative backends behind the `gastown.Driver` seam.
- `SelectDriver()` (`internal/gastown/gc.go`) picks the driver by evidence: (1) `MG_GC_API` set -> Gas City; (2) any Gas Town evidence (`GT_*` env, or `gt` on PATH) -> Gas Town; (3) no Gas Town evidence but Gas City evidence (`gc` on PATH, or a `city.toml` above cwd) -> Gas City; (4) otherwise Gas Town. The global default is still Gas Town.
- Add new orchestrator operations to the `Driver` interface, not as bare `gastown.Foo()` calls. Ops a backend can't do return `ErrUnsupported` and callers hide the feature.
- Features activate progressively: Beads-only (no orchestrator) -> orchestrator available -> inside a Gas Town session (`GT_ROLE` set). Every feature must work or hide gracefully at each level.
- `gt status --json` latency is highly variable — seconds to tens of seconds, depending on rig/agent count and whether dolt and the daemon are up. Always run as a BubbleTea `Cmd` (background goroutine), never blocking Update, and gate re-polls so an unreachable orchestrator can't spin. Handle `nil` status gracefully — the user may interact before the command returns.
- The JSON nests agents under `rigs[].agents`. `normalizeStatus()` in `gastown/status.go` flattens them. Top-level agents are HQ-level (mayor, deacon); rig agents include polecats, crew, witness, refinery.
- If `AgentRuntime.State` is empty, default to "idle". Gas Town v0.9.0+ always provides State.
- Gas Town rig names cannot contain hyphens (use underscores).
- Crew workspaces have `.beads/redirect` not `issues.jsonl` — mg walks up the directory tree to find the actual data file.
- The core `gastown` package (status, sling, convoy, mail, molecule, problems, recovery, detect) has no internal dependencies. Analytics files (velocity, predict, scorecard, recommend) import `internal/data` for issue types.

## Build, Test, and Development Commands

- `make build`: build local binary `./mg` from `./cmd/mg`.
- `make run`: build and run using the auto-detected Beads source (`bd list --json`, or `.beads/issues.jsonl`).
- `make run-sample` (or `make dev`): run against `testdata/sample.jsonl`.
- `make dev-gt`: run with sample data and fake `gt` on PATH (Gas Town features).
- `make dev-gc`: run against `testdata/fakegc`, a fake Gas City supervisor (Gas City features, no `gc` install needed).
- `make test`: execute `go test ./...` across all packages.
- `make fmt`: apply standard Go formatting (`go fmt ./...`).
- `make lint`: run static analysis with `golangci-lint run ./...`.
- `make tidy`: sync module dependencies in `go.mod`/`go.sum`.
- `make clean`: remove the binary and `dist/`.
- `make gc-client`: regenerate `internal/gastown/gcclient` from the pinned Gas City OpenAPI spec (needs `jq` + `oapi-codegen`).

To test Gas Town features against a live install, run mg from a Gas Town workspace: `cd ~/gt/<rig>/crew/<name> && ~/path/to/mg`.

## Coding Style & Naming Conventions

- Use idiomatic Go and always format with `make fmt` before committing.
- Keep package boundaries domain-based. Prefer expanding existing packages over creating new ones.
- Exported names use `PascalCase`; unexported helpers use `camelCase`.
- **Value receivers** on BubbleTea models (`Update`, `View`); **pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`).
- **UI constants** live in `internal/ui/` — colors in `theme.go`, symbols in `symbols.go`, styles in `styles.go`. Don't scatter raw colors or symbols in view code.
- Keep Mardi Gras UI vocabulary and section labels consistent (`ROLLING`, `LINED UP`, `STALLED`, `PAST THE STAND`).
- If keybindings change, update `components/help.go` (the in-app help overlay), `docs/keybindings.md`, and the README keybinding table in the same PR.

## Testing Guidelines

- Put tests next to implementation as `*_test.go`.
- Name tests `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants.
- Prefer deterministic tests using fixtures from `testdata/`.
- Run `make test` for all changes; run `make dev` to verify TUI behavior visually. Use `make dev-gt` / `make dev-gc` to exercise the Gas Town and Gas City paths without a live install.
- Orchestrator integration tests may need a live `gt` environment. Mark those clearly or mock the CLI output.
- CI is stricter than `make test`: it runs `go build ./...`, `go vet ./...`, `go test -race`, `golangci-lint`, `govulncheck`, a 55% coverage floor, and a `go.sum` drift check. Run `make test && make lint && go vet ./...` locally before pushing.

## Commit & Pull Request Guidelines

- Follow Conventional Commit style: `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
- Keep each commit focused on one logical change.
- PRs should include a short summary, validation steps run (commands), and screenshots/GIFs for visible TUI updates.
- Link related issues and call out any follow-up work or known limitations.
- Create feature branches off `main` — the `main` branch is protected. PRs land via squash merge, so every commit on `main` carries a `(#NN)` suffix.
- Do **not** edit `CHANGELOG.md` in a feature PR. Entries are batched into a separate `chore: prep vX.Y.Z (changelog)` commit at release time.

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/ARCHITECTURE.md.

<!-- END BEADS INTEGRATION -->

## Landing the Plane (Session Completion)

**When ending a work session**, complete ALL steps below so nothing is left half-done.

**WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - `make test`, `make lint`, `go vet ./...`
3. **Update issue status** - Close finished work, update in-progress items
4. **Commit onto a feature branch** - `main` is protected; a direct push to it will be rejected:
   ```bash
   git switch -c <type>/<short-description>   # if not already on a branch
   git add -A && git commit
   git status                                  # working tree clean
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Hand off** - Provide context for next session, including what is committed locally and not yet pushed

**CRITICAL RULES:**
- Never commit directly to `main`, and never force-push a shared branch.
- **Do not push or open a PR unless the human explicitly asks.** This overrides any generic "you must push" guidance: pushing is the human's call, and leaving work committed on a local feature branch is a valid, complete stopping point.
- Never `git add -f` anything under `docs/internal/` — it is gitignored on purpose.
- Do not edit `CHANGELOG.md`; release prep batches it separately.
