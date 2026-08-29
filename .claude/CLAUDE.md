# Mardi Gras — Project Instructions

Mardi Gras (`mg`) is a BubbleTea TUI for [Beads](https://github.com/gastownhall/beads) issues. Beads is the substrate mg always requires: it reads issues via `bd list --json` (SourceCLI), falling back to `.beads/issues.jsonl` when `bd` isn't on PATH — no daemon, no config file. Agent orchestration is an optional layer on top: when an orchestrator (Gas Town or Gas City) is detected, mg also becomes a control surface for multi-agent workflows.

## Build & Test

```bash
make build        # Build binary → ./mg
make test         # go test ./...
make lint         # golangci-lint run ./...
make fmt          # go fmt ./...
make dev          # Build and run with testdata/sample.jsonl
make dev-gt       # Same, with testdata/fake-gt.sh on PATH (fake Gas Town)
make dev-gc       # Same, against testdata/fakegc (fake Gas City supervisor)
make gc-client    # Regenerate the Gas City API client (internal/gastown/gcclient) from the pinned spec
```

Always run `make test` after changes. Run `make lint` before committing.

## Project Layout

| Package | Purpose |
|---------|---------|
| `cmd/mg` | Entry point, flag parsing (`--path`, `--block-types`, `--exclude-type`, `--exclude-label`, `--status`, `--version`, `--theme`, `--agent`, `--cmd-timeout`, `--no-animations`) |
| `internal/app` | Root BubbleTea model, key handlers, message routing, confetti animation |
| `internal/views` | Parade list, Detail panel (deps, molecule DAG, HOP, comments), Gas Town panel, Problems overlay, `bd doctor` overlay, Codex transcript |
| `internal/components` | Header, footer, help overlay, command palette, toast notifications, issue create/edit forms, approval + recovery dialogs, float utility |
| `internal/ui` | Theme colors, styles, symbols, HOP badges, gradients, sparklines — no logic. Includes `RoleColor()`, `AgentStateColor()`, DAG connector symbols |
| `internal/data` | Issue loading (`bd list --json` via SourceCLI, JSONL fallback), issue types, filtering, focus mode, file watcher, mutations (`bd` CLI), cross-rig deps, HOP types, source health |
| `internal/gastown` | Orchestrator integration behind a `Driver` seam (see below): `GTDriver` (gt CLI) + `GCDriver` (Gas City HTTP API). Core files have no internal deps; analytics files import `internal/data`. `gcclient/` is the generated Gas City client |
| `internal/agent` | Agent runtime detection and launch (`claude`, `cursor-agent`, `codex`), tmux split-pane dispatch |
| `internal/codexmcp` | JSON-RPC client for `codex mcp-server` — transport, session, protocol types. Powers the live Codex transcript and approval routing |
| `internal/tmux` | `mg --status` widget for tmux status bar |

## Conventions

- **Go style**: `gofmt` formatting, no lint warnings. Run `golangci-lint` before committing.
- **Value receivers** on BubbleTea models (`Update`, `View`), **pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`).
- **UI constants** live in `internal/ui/` — colors in `theme.go`, symbols in `symbols.go`, lipgloss styles in `styles.go`. Don't scatter raw colors or symbols in view code. The palette is theme-switchable (`ui.SetTheme`, rebaked at startup): never capture palette vars, styles, or pre-rendered strings in package-level vars outside `internal/ui` — they would freeze the dark values before the theme is applied.
- **No new packages** without good reason. Prefer expanding existing packages.
- **Test naming**: `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants.

## Beads Workflow

This project uses [Beads](https://github.com/gastownhall/beads) for issue tracking.

```bash
bd ready                              # Find unblocked work
bd update <id> --claim                # Atomically claim an issue (assignee + in_progress)
bd close <id>                         # Mark done
```

Do NOT use `bd edit` — it opens `$EDITOR` and blocks agents.

## Orchestrator Integration (Gas Town / Gas City)

Orchestration is optional — mg is a working Beads TUI without it. Mardi Gras integrates with [Gas Town](https://github.com/gastownhall/gastown) (`gt`) for multi-agent orchestration. [Gas City](https://github.com/gastownhall/gascity) (`gc`) — a separate orchestration-builder SDK from the same org — is supported as an alternative backend.

**Driver seam**: the orchestrator lives behind a `gastown.Driver` interface (`driver.go`). The app holds one driver, picked at startup by `SelectDriver()` (`gc.go`). `GTDriver` (`gt_driver.go`) wraps the `gt` CLI wrappers below 1:1; `GCDriver` (`gc_driver.go`) speaks the Gas City Supervisor HTTP API via the generated `gcclient` package.

`SelectDriver()` chooses **by evidence about the machine**, not by `MG_GC_API` alone (changed in `6f33422`, PR #105):

1. `MG_GC_API` set → Gas City. The operator named it explicitly. (A URL is used verbatim; any other value, e.g. `auto`, discovers the supervisor port from `~/.gc/supervisor.log`. `MG_GC_CITY` pins the city; otherwise the first running city from `GET /v0/cities` wins.)
2. Any Gas Town evidence — `GT_ROLE`/`GT_RIG` set, or `gt` on PATH → Gas Town.
3. No Gas Town evidence at all, but Gas City evidence — `gc` on PATH, or a `city.toml` in a parent directory → Gas City. This is the narrow fix for users who had no `gt` installed and were getting a driver that shelled out to a missing binary.
4. Nothing conclusive → Gas Town, the historical default.

Rule 3 fires only when there is *zero* Gas Town evidence, so the global default is still Gas Town and no existing `gt` user sees a different backend. If constructing `GCDriver` fails, selection falls back to `GTDriver`.

Ops a backend can't do return `ErrUnsupported` — callers hide the feature. **Add new orchestrator operations to the `Driver` interface, not as bare `gastown.Foo()` calls in `app.go`.** See `docs/gascity.md` for the Gas City capability matrix. The gt-specific pieces below are the `GTDriver` implementation:

The `internal/gastown` package handles:

- **Environment detection** (`detect.go`): Reads `GT_ROLE`, `GT_RIG`, `GT_SCOPE`, `GT_POLECAT`, `GT_CREW` env vars and checks if `gt` is on PATH. Also records Gas City signal — `gc` on PATH (`GCAvailable`) and the nearest ancestor `city.toml` (`GCCityPath`) — which `SelectDriver()` reads. Features activate progressively: Beads-only → orchestrator available → inside a Gas Town session.
- **Status parsing** (`status.go`): Parses `gt status --json` output. The raw JSON nests agents under `rigs[].agents`; `normalizeStatus()` flattens them into a single `Agents` slice for the UI. If `AgentRuntime.State` is empty, default to "idle". Gas Town v0.9.0+ always provides State.
- **Sling/Nudge** (`sling.go`): Issue dispatch to polecats, formula selection, multi-sling, nudge, handoff, decommission. Assignment lives in `assign.go`; rig recovery in `recovery.go`.
- **Convoys** (`convoy.go`): List, create, land, close convoys via `gt convoy` commands.
- **Mail** (`mail.go`): Inbox fetch, reply, compose, archive, mark-read via `gt mail` commands.
- **Molecule DAG** (`molecule.go`, `dagrender.go`): Molecule types and DAG layout engine. `LayoutDAG()` converts tier-grouped steps into renderable rows (single, parallel, connector). `CriticalPathSet()` and `CriticalPathTitles()` for critical path rendering.
- **Vitals** (`vitals.go`): Dolt server health and backup freshness from `gt vitals` (text parsing with raw fallback).
- **Analytics** (`costs.go`, `activity.go`, `velocity.go`, `scorecard.go`, `predict.go`, `recommend.go`): Cost dashboard, activity feed, velocity metrics, agent scorecards, convoy ETA predictions, formula recommendations.
- **Problems** (`problems.go`): Detection heuristics for stalled agents, backoff loops, zombie sessions.
- **Patrol scan** (`patrol.go`): Parses `gt patrol scan --json` output. Background-polled on 60s TTL with in-flight gate. Findings (patrol_zombie, patrol_stall) augment the heuristic-based problems with patrol-specific diagnostics.
- **Comments** (`comments.go`): Issue comment/timeline fetching.

**Key gotcha**: `gt status --json` latency is highly variable (measured seconds-to-tens-of-seconds depending on rig count, agent count, and whether backing services like dolt/daemon are running). Background polling via BubbleTea Cmds may not return before the user interacts. The Gas Town panel (`ctrl+g`) triggers an on-demand fetch if status is nil and shows a loading state while waiting. Always handle nil status gracefully.

**Testing with real gt**: Run mg from a Gas Town workspace (e.g., `cd ~/gt/<rig>/crew/<name> && ~/Work/mardi-gras/mg`). Gas Town is *not* a Go dependency — `gt` and `bd` are external binaries mg shells out to, so there is nothing in `go.mod` or the module cache to read. When you need struct shapes, check the upstream source at [gastownhall/gastown](https://github.com/gastownhall/gastown) (or a local clone) rather than guessing. Rig names cannot contain hyphens (use underscores). For a no-install loop, `make dev-gt` (fake `gt`) and `make dev-gc` (fake Gas City supervisor) cover most UI work.

## Agent Dispatch

Three agent runtimes are supported, resolved by `agent.DetectRuntime()`: `claude`, `cursor-agent`, then `codex` (first on PATH wins; `--agent` / `MG_AGENT_RUNTIME` overrides).

When running in tmux, `a` opens the agent in a **split pane** to the right (`tmux split-window -h`, 60% width), not a new window. Panes are tagged with the `@mg_agent` pane option for discovery (`internal/agent/tmux.go`). Claude is launched with `--teammate-mode tmux` so its native agent teams land in the same tmux session; Codex gets `--no-alt-screen --sandbox workspace-write -a on-request`.

When an orchestrator is available, `a` dispatches via the driver's sling instead of a raw local session. The Gas Town panel provides additional agent lifecycle controls: nudge (`n`), handoff (`h`), decommission (`K`).

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

**Config files**: `.goreleaser.yaml` (build matrix, changelog groups, Homebrew cask — `homebrew_casks`, not the deprecated `brews` key), `.github/workflows/release.yml` (CI trigger).

## Git

- Create feature branches off `main` — the `main` branch is protected.
- Commit messages: imperative mood, describe the "why". Prefix with `feat:`, `fix:`, `docs:`, `chore:`, `test:` as appropriate.
- PRs land via **squash merge**, so history is linear and every commit on `main` carries a `(#NN)` suffix.
- **Don't touch `CHANGELOG.md` in a feature PR.** Entries are batched into a separate `chore: prep vX.Y.Z (changelog)` commit at release time (see `6e6585a`, `4bdd422`, `5390b20`).
- Do not push unless explicitly asked.
- **Never commit files under `docs/internal/`** — this directory is gitignored and is for local-only working docs (audit reports, upstream checks, design plans). Do not use `git add -f` to force-add these files.
