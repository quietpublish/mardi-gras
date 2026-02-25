# Mardi Gras — Project Instructions

Mardi Gras (`mg`) is a BubbleTea TUI for Beads issues with full Gas Town agent orchestration. It reads `.beads/issues.jsonl` directly — no daemon, no config file. When `gt` is on PATH, it becomes a control surface for multi-agent workflows.

## Build & Test

```bash
make build        # Build binary → ./mg
make test         # go test ./...
make lint         # golangci-lint run ./...
make fmt          # go fmt ./...
make dev          # Build and run with testdata/sample.jsonl
```

Always run `make test` after changes. Run `make lint` before committing.

## Project Layout

| Package | Purpose |
|---------|---------|
| `cmd/mg` | Entry point, flag parsing (`--path`, `--block-types`, `--status`, `--version`) |
| `internal/app` | Root BubbleTea model, key handlers, message routing, confetti animation |
| `internal/views` | Parade list, Detail panel (deps, molecule DAG, HOP, comments), Gas Town panel, Problems overlay |
| `internal/components` | Header, footer, help overlay, command palette, toast notifications, issue create form, float utility |
| `internal/ui` | Theme colors, styles, symbols, HOP badges — no logic. Includes `RoleColor()`, `AgentStateColor()`, DAG connector symbols |
| `internal/data` | JSONL loading, issue types, filtering, focus mode, file watcher, mutations (`bd` CLI), cross-rig deps, HOP types |
| `internal/gastown` | Gas Town integration (see below). Core files have no internal deps; analytics files import `internal/data` |
| `internal/agent` | Claude Code launch, tmux window dispatch |
| `internal/tmux` | `mg --status` widget for tmux status bar |

## Conventions

- **Go style**: `gofmt` formatting, no lint warnings. Run `golangci-lint` before committing.
- **Value receivers** on BubbleTea models (`Update`, `View`), **pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`).
- **UI constants** live in `internal/ui/` — colors in `theme.go`, symbols in `symbols.go`, lipgloss styles in `styles.go`. Don't scatter raw colors or symbols in view code.
- **No new packages** without good reason. Prefer expanding existing packages.
- **Test naming**: `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants.

## Beads Workflow

This project uses [Beads](https://github.com/beads-project/beads) for issue tracking.

```bash
bd ready                              # Find unblocked work
bd update <id> --claim                # Atomically claim an issue (assignee + in_progress)
bd close <id>                         # Mark done
bd sync                               # Sync beads data
```

Do NOT use `bd edit` — it opens `$EDITOR` and blocks agents.

## Gas Town Integration

Mardi Gras integrates with [Gas Town](https://github.com/steveyegge/gastown) (`gt`) for multi-agent orchestration. The `internal/gastown` package (15 files, no internal deps) handles:

- **Environment detection** (`detect.go`): Reads `GT_ROLE`, `GT_RIG`, `GT_SCOPE`, `GT_POLECAT`, `GT_CREW` env vars and checks if `gt` is on PATH. Features activate progressively: Beads-only → gt available → inside Gas Town.
- **Status parsing** (`status.go`): Parses `gt status --json` output. The raw JSON nests agents under `rigs[].agents`; `normalizeStatus()` flattens them into a single `Agents` slice for the UI. If `AgentRuntime.State` is empty, default to "idle". Gas Town v0.8.0+ always provides State.
- **Sling/Nudge** (`sling.go`): Issue dispatch to polecats, formula selection, multi-sling, nudge, handoff, decommission.
- **Convoys** (`convoy.go`): List, create, land, close convoys via `gt convoy` commands.
- **Mail** (`mail.go`): Inbox fetch, reply, compose, archive, mark-read via `gt mail` commands.
- **Molecule DAG** (`molecule.go`, `dagrender.go`): Molecule types and DAG layout engine. `LayoutDAG()` converts tier-grouped steps into renderable rows (single, parallel, connector). `CriticalPathSet()` and `CriticalPathTitles()` for critical path rendering.
- **Analytics** (`costs.go`, `activity.go`, `velocity.go`, `scorecard.go`, `predict.go`, `recommend.go`): Cost dashboard, activity feed, velocity metrics, HOP scorecards, convoy ETA predictions, formula recommendations.
- **Problems** (`problems.go`): Detection heuristics for stalled agents, backoff loops, zombie sessions.
- **Comments** (`comments.go`): Issue comment/timeline fetching.

**Key gotcha**: `gt status --json` takes ~9 seconds to run. Background polling via BubbleTea Cmds may not return before the user interacts. The Gas Town panel (`ctrl+g`) triggers an on-demand fetch if status is nil and shows a loading state while waiting. Always handle nil status gracefully.

**Testing with real gt**: Run mg from a Gas Town workspace (e.g., `cd ~/gt/<rig>/crew/<name> && ~/Work/mardi-gras/mg`). The `gt` source code is at `~/go/pkg/mod/github.com/steveyegge/gastown@v0.8.0/` — check it directly rather than guessing struct shapes. Rig names cannot contain hyphens (use underscores).

## Agent Dispatch

When running in tmux, `mg` launches Claude agents in new tmux windows. Agents are tagged with `@mg_agent` window options for tracking. The `--teammate-mode tmux` flag enables Claude Code's native agent teams.

When Gas Town is available, `a` dispatches via `gt sling` instead of raw Claude sessions. The Gas Town panel provides additional agent lifecycle controls: nudge (`n`), handoff (`h`), decommission (`K`).

## Releasing

Releases are automated via GoReleaser + GitHub Actions. Pushing a semver tag triggers the pipeline.

```bash
# 1. Ensure main is clean and tests pass
make test
go vet ./...

# 2. Tag the release (use annotated tags with a summary)
git tag -a v0.X.Y -m "v0.X.Y: short summary"

# 3. Push the tag — this triggers .github/workflows/release.yml
git push origin v0.X.Y

# 4. Monitor the release workflow
gh run list --limit 1
gh run watch <run-id> --exit-status
```

The workflow runs `goreleaser` which builds binaries for linux/darwin/windows (amd64/arm64), creates a GitHub Release with auto-generated changelog, and updates the Homebrew tap (`matt-wright86/homebrew-tap`).

**Versioning**: Semver. Bump minor (0.X.0) for feature releases, patch (0.0.X) for bug-fix-only releases.

**Config files**: `.goreleaser.yaml` (build matrix, changelog groups, Homebrew formula), `.github/workflows/release.yml` (CI trigger).

## Git

- Create feature branches off `main` — the `main` branch is protected.
- Commit messages: imperative mood, describe the "why". Prefix with `feat:`, `fix:`, `docs:`, `chore:`, `test:` as appropriate.
- Do not push unless explicitly asked.
