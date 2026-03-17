# Changelog

All notable changes to Mardi Gras are documented here. For full release details including binaries and install instructions, see the [Releases](https://github.com/quietpublish/mardi-gras/releases) page.

## v0.12.1 (2026-03-16)

### Added
- **Propelled agent state** — Gas Town v0.12.1 adds a `propelled` state for agents under ACP propulsion. Renders with dark turquoise color and ⚡ symbol in the agent roster.

## v0.12.0 (2026-03-15)

### Added
- **Doctor diagnostics overlay** — press `D` to run `bd doctor --agent --json` and display results in a dedicated panel with severity indicators, category labels, and fix commands. Navigate with `j`/`k`, refresh with `R`.
- **Quick-action shortcuts** — `r` comment, `y` assign, `t` tag/label, `l` link/dependency. Each opens an input bar in the footer, submits via `bd` CLI, and shows a success/error toast. Bypasses the CLI discoverability gap.
- **Full-text search** — the `/` filter now searches across issue description, assignee, owner, notes, and labels — not just ID and title.
- **Inline issue editing** — press `e` to open a pre-populated edit form for the selected issue's title and priority. Tab cycles fields, `j`/`k` adjusts priority, enter saves.
- **Agent alias in roster** — Gas Town agent roster shows `AgentAlias` (e.g., `[sonnet-46]`) when available, preferring it over the raw `AgentInfo` field.
- **Zombie indicators in parade** — when a polecat's session dies but its hook is still active, the associated issue shows a ☠ indicator directly in the parade. Distinct from dead-rig orphans (💀) and suppressed when both apply.
- **Live agent output** — detail panel shows the last 15 lines of an active agent's tmux pane output in an AGENT OUTPUT section, captured via `tmux capture-pane` with ANSI stripping.
- **Superscript counts in Gas Town** — AGENTS, CONVOYS, and MAIL section headers show item counts as Unicode superscripts (e.g., AGENTS³).
- **Dual velocity sparkline** — VELOCITY section shows a 7-day created-vs-closed dual sparkline using braille characters.
- **bd version in footer** — workspace identity now includes the bd version (e.g., `mardi_gras/dolt v0.60.0`).

### Infrastructure
- **Command mocking** — exec functions converted to `var` function pointers for testability. Mock helpers (`mockRun`, `mockExecCapture`) in both `data` and `gastown` packages.
- **274 new tests** — mock-based tests for all 26 functions that shell out to `bd` or `gt`. Total test count: 532 → 850+.
- **CI hardening** — added `go vet`, coverage profiling with 55% threshold, coverage artifact upload, and `go.sum` drift check.
- **Gas Town contract tests** — embedded JSON fixtures and forward-compatibility tests for convoy, mail, costs, and comments.

## v0.11.0 (2026-03-15)

### Added
- **`--no-animations` flag** — disable confetti and header shimmer for SSH/low-bandwidth sessions. Also available as `MG_NO_ANIMATIONS=1` env var. (PR #2 by @jason-curtis)
- **`--cmd-timeout` flag** — scale external command timeouts for slow connections (default 30s, max 300s). Also available as `MG_CMD_TIMEOUT` env var. (PR #2 by @jason-curtis)
- **Multi-rig indicator** — header shows rig count when Gas Town reports multiple rigs. (PR #2 by @jason-curtis)
- **Convoy from epic** — pressing `C` on an epic auto-populates the convoy with child issues via `gt convoy create --from-epic`.
- **Workspace identity in footer** — footer shows database name and backend type from `bd context --json` (e.g., `bd list (cli) · 5s ago · mardi_gras/dolt`).

### Fixed
- bd version warning updated to reference v0.60.0+.
- Command timeout capped at 300s to prevent degenerate durations.

## v0.10.0 (2026-03-12)

### Added
- **Rig recovery confirmation dialog** — pressing `R` on a dead rig now opens a confirmation dialog showing orphaned issues and letting you choose between "Release + Re-sling" or "Release only" modes.
- **Orphan indicators** — issues assigned to dead rigs show a skull badge in the parade.
- **Recovery in command palette** — "Recover dead rigs" action available via `:` when dead rigs are detected.
- **Epic progress** — detail panel shows N/M completion progress for epic issues.
- **Pre-push hook** — `make test` and `make lint` run automatically before every `git push`.

### Changed
- CI GitHub Actions bumped to Node.js 24-compatible versions (checkout v6, setup-go v6, golangci-lint-action v9, goreleaser-action v7).
- All Go dependencies updated to latest (glamour v1.0.0, chroma v2.23, golang.org/x/net v0.52, and 10 others).

## v0.9.0 (2026-03-08)

### Added
- **Rig recovery** — detect dead rigs (0 polecats, orphaned work) and recover them via `R` key. Releases orphaned issues and optionally re-slings them to healthy polecats.
- **Dead rig detection** — problems view groups orphaned agents under dead-rig banners instead of individual zombie alerts.

## v0.8.0 (2026-03-06)

### Added
- **FIX_NEEDED polecat state** — renders in agent roster with distinct color and icon when a polecat needs manual intervention.
- **Dog agents in roster** — dog agents (reaper, compactor, etc.) render with a dog symbol in the Gas Town panel.

## v0.7.0 (2026-03-04)

### Added
- **JSON contract tests** — 19 tests verifying compatibility with `bd list --json` output format.
- **Structured JSON error handling** — parses bd v0.59.1+ structured JSON errors from stderr for clearer toast messages.
- **`bd show --current`** — header shows the currently active issue ID.

## v0.6.0 (2026-03-02)

### Added
- **Comments & timeline** — detail panel shows issue comments and activity timeline fetched via `bd comments --json`.
- **Molecule DAG rendering** — visual flow graph with parallel branching and connector lines between tiers.
- **HOP quality badges** — reputation stars, crystal/ephemeral indicators, and validator verdicts in detail panel.

## v0.5.0 (2026-02-28)

### Added
- **Vitals panel** — Dolt server health (port, PID, disk, connections, latency) and backup freshness in Gas Town dashboard.
- **Cost dashboard** — session counts, token usage, and cost breakdown per agent.
- **Activity feed** — real-time event ticker in Gas Town panel.
- **Velocity metrics** — issue flow rates and agent utilization.

## v0.4.0 (2026-02-26)

### Added
- **Gas Town panel** (`ctrl+g`) — full agent control surface with roster, convoys, and mail.
- **Sling & nudge** — dispatch issues to polecats via `gt sling`, nudge agents with `n`.
- **Mail inbox** — read, reply, compose, and archive messages between agents.
- **Convoy management** — create, land, and close delivery batches.

## v0.3.0 (2026-02-24)

### Added
- **Multi-select** — `space`/`x` to toggle, `Shift+J/K` to select and move, bulk status changes.
- **Command palette** — fuzzy-match palette via `:` or `Ctrl+K`.
- **Focus mode** — `f` to filter to assigned work and top-priority issues.
- **Issue creation** — `N` to create new issues with type, priority, and description.

## v0.2.0 (2026-02-22)

### Added
- **Detail panel** — metadata, dependencies, rich fields with markdown rendering.
- **Agent integration** — launch Claude Code or Cursor agents from the TUI.
- **tmux dispatch** — agents open in new tmux windows for multi-agent workflows.
- **Filter mode** — `/` with free text, type tokens, and priority shorthands.

## v0.1.0 (2026-02-20)

### Added
- Initial release: parade view, status changes, clipboard branch names, tmux status widget.
