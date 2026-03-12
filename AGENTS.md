# Repository Guidelines

## Project Structure & Module Organization

- `cmd/mg/main.go`: CLI entrypoint, `--path`/`--block-types`/`--status`/`--version` flag handling.
- `internal/app`: BubbleTea root model, key routing, pane orchestration, confetti animation.
- `internal/views`: Parade (left pane), Detail (right pane), Gas Town panel, Problems overlay.
- `internal/components`: Header, Footer, Help overlay, Command palette, Toast notifications, Create form, Float utility.
- `internal/data`: JSONL loading, grouping, dependency/status logic, filtering, focus mode, mutations (`bd` CLI), cross-rig deps, HOP types.
- `internal/gastown`: Gas Town integration — environment detection, `gt status` parsing, sling/nudge/handoff/decommission, convoy CRUD, mail inbox/reply/compose, molecule DAG, costs, vitals (server health + backups), activity feed, velocity, scorecards, predictions, formula recommendations, problem detection (stalled/stuck/backoff/zombie/dead_rig), rig recovery.
- `internal/agent`: Claude Code prompt builder, tmux window launch/discover/kill.
- `internal/tmux`: tmux status line widget (`mg --status` mode).
- `internal/ui`: Theme palette (with Gas Town role/state colors), Lipgloss styles, Unicode symbols (including DAG connectors), HOP badge rendering.
- `testdata/sample.jsonl`: fixture for tests and local demo runs.
- `testdata/fake-gt.sh`: fake `gt` binary for local Gas Town testing (`make dev-gt`).
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
- If `AgentRuntime.State` is empty, default to "idle". Gas Town v0.9.0+ always provides State.
- Gas Town rig names cannot contain hyphens (use underscores).
- Crew workspaces have `.beads/redirect` not `issues.jsonl` — mg walks up the directory tree to find the actual data file.
- The core `gastown` package (status, sling, convoy, mail, molecule, problems, recovery, detect) has no internal dependencies. Analytics files (velocity, predict, scorecard, recommend) import `internal/data` for issue types.

## Build, Test, and Development Commands

- `make build`: build local binary `./mg` from `./cmd/mg`.
- `make run`: build and run using auto-detected `.beads/issues.jsonl`.
- `make run-sample` (or `make dev`): run against `testdata/sample.jsonl`.
- `make dev-gt`: run with sample data and fake `gt` on PATH (Gas Town features).
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
- Run `make test` for all changes; run `make dev` to verify TUI behavior visually. Use `make dev-gt` to test Gas Town features.
- Gas Town integration tests may need a live `gt` environment. Mark those clearly or mock the CLI output.

## Commit & Pull Request Guidelines

- Follow Conventional Commit style: `feat:`, `fix:`, `docs:`, `test:`, `chore:`.
- Keep each commit focused on one logical change.
- PRs should include a short summary, validation steps run (commands), and screenshots/GIFs for visible TUI updates.
- Link related issues and call out any follow-up work or known limitations.
- Create feature branches off `main` — the `main` branch is protected.

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

For more details, see README.md and docs/QUICKSTART.md.

<!-- END BEADS INTEGRATION -->

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
