# Changelog

All notable changes to Mardi Gras are documented here. For full release details including binaries and install instructions, see the [Releases](https://github.com/quietpublish/mardi-gras/releases) page.

## v0.17.0 (2026-04-19)

### Added
- **`started_at` timestamp in Detail panel** — Beads v1.0.1 added `started_at` to the issue JSON, auto-set on the first `in_progress` transition and preserved across later status changes. mg parses the field into `Issue.StartedAt` and renders a "Started" event in the Detail activity timeline between Created and Due. Contract tests cover populated, minimal, and explicit-null fixtures.

### Changed
- **`gt status` latency note** — replaced the obsolete "~9 seconds" gotcha in `CLAUDE.md` with a variability note. Gas Town v1.0.0 parallelizes within-rig work ([gastown#3504](https://github.com/steveyegge/gastown/pull/3504)), but latency still ranges from seconds to tens of seconds depending on rig count and whether dolt/daemon/tmux are running.
- **Dependencies updated** — `bubbletea/v2` 2.0.2 → 2.0.6, `lipgloss/v2` 2.0.2 → 2.0.3, `charmbracelet/ultraviolet` dated bump (2026-03-16 → 2026-04-16), `charmbracelet/x/ansi` 0.11.6 → 0.11.7, plus indirect refresh of `regexp2`, `mattn/go-isatty`, `mattn/go-runewidth`, `yuin/goldmark`, and `golang.org/x/{net,sys,term,text}`. All patch- or date-level within the same major.

## v0.16.0 (2026-04-09)

### Added
- **Beads v1.0.0 issue types** — `spike`, `story`, and `milestone` are now first-class types with distinct colors in the parade and detail views. Matches the types added in beads v1.0.0 ([beads#2923](https://github.com/steveyegge/beads/pull/2923)).
- **Convoy watch/unwatch** — new convoy-panel actions to subscribe to or unsubscribe from convoy notifications via `gt convoy watch` / `gt convoy unwatch`.
- **Mail mark-all-read** — bulk-dismiss mail inbox via `R` in the Gas Town mail section (`gt mail mark-read --all`).

### Security / Hardened
- **Input validation, source resilience, and ANSI stripping** — broader hardening of CLI-argument paths, `.beads/` discovery fallbacks, and output sanitization.

### Changed
- **Dependencies updated** — `charm.land/bubbles/v2` 2.0.0 → 2.1.0, `lucasb-eyer/go-colorful` 1.3.0 → 1.4.0. CI: `codecov/codecov-action` 5 → 6.

## v0.15.1 (2026-03-31)

### Added
- **Patrol scan integration** — Problems overlay now includes findings from `gt patrol scan --json` (requires Gas Town v0.13.0+). Polled every 60s in the background with TTL gating and in-flight dedup. Patrol-detected zombies and stalls appear alongside existing heuristics, with agent identity preserved for nudge/handoff/decommission actions. Header warning count updates immediately when patrol data arrives.

### Changed
- **Performance optimizations** — dependency evaluation cached on parade items (eliminates 3-4x redundant `EvaluateDependencies` calls per issue per render), glamour markdown renderer cached on detail panel (recreated only on resize), confetti particles and necklace beads pre-styled at creation time, status indicators and priority badges pre-rendered as package-level vars, age-colored issue IDs cached during parade rebuild. Contributed by @asbjaare. ([#16](https://github.com/quietpublish/mardi-gras/pull/16))
- **Dependencies updated** — charmbracelet/ultraviolet, charmbracelet/x, goldmark v1.7.17 (XSS URL escaping fix, table cell panic fix), kr/pretty v0.3.1.

### Fixed
- **Hyphenated issue prefixes** — CLI mode now correctly handles issue prefixes containing hyphens (e.g., `mcc-tools-7pk`). Previously `issuePrefixFromID()` split on the first hyphen, extracting `mcc` instead of `mcc-tools`. ([#17](https://github.com/quietpublish/mardi-gras/issues/17))

## v0.15.0 (2026-03-22)

### Added
- **`--exclude-type` flag** — hide issue types from the parade and status output (e.g., `mg --exclude-type=epic,chore`). Excluded issues remain in dependency graphs and the detail panel.
- **Claim-next on close** — closing a single issue now runs `bd close --claim-next`, automatically claiming the next ready issue. The parade selects the claimed issue and fetches its detail. Falls back gracefully when no ready work exists.
- **Add note** — new palette action (`:` → "Add note") to append notes via `bd note`. Notes appear in the detail panel after reload.
- **Create & assign to crew** — new palette shortcut (`:` → "Create & assign to crew") for the Gas Town crew assignment flow.

### Removed
- **HOP dead code** — removed ~650 lines of dead HOP (Hierarchy of Proof) code after beads v0.62.0 dropped these fields from the schema. Types, views, tests, scorecard logic, UI constants, and docs all cleaned up. `SymCrystal` renamed to `SymDiamond` for molecule critical-path reuse.

### Fixed
- **Detail cache refresh** — molecule, comments, and rich detail now auto-refresh when the selected issue changes after a reload (e.g., via claim-next). Previously required manually pressing `enter`.

## v0.14.0 (2026-03-20)

### Added
- **Assign to crew** — when Gas Town is available, the issue create form (`N`) shows a "Crew" field. Enter a crew member name to create the issue, hook it, and nudge the agent in one step via `gt assign`. The field is optional — leave it empty for a normal `bd create`.

### Changed
- **Documentation restructured** — README slimmed from 430 to 211 lines. Detailed docs moved to topic-based files under `docs/`:
  - [Keybindings](docs/keybindings.md) — full shortcut reference
  - [Parade and filtering](docs/filtering.md) — sections, detail panel, filtering syntax, command palette
  - [Agent integration](docs/agents.md) — runtime detection, tmux dispatch
  - [Gas Town integration](docs/gastown.md) — sling, assign, convoys, operational intelligence, problems
- Updated hero screenshot to current UI.

## v0.13.1 (2026-03-18)

### Fixed
- **Navigation sluggishness** — reduced OSC guard suppression window from 500ms to 80ms. Terminal capability reply bursts (OSC 11, DECRPM) complete within ~60ms; the old 500ms window was eating real `j`/`k` keypresses. Also reduced deferred key delay from 60ms to 30ms for snappier input. ([#9](https://github.com/quietpublish/mardi-gras/issues/9))
- Added debug logging for OSC guard pass-through decisions and deferred key lifecycle (`MG_DEBUG=1`).
- Sanitized environment variables in debug log output to prevent accidental secret exposure.

## v0.13.0 (2026-03-17)

### Added
- **CODE_OF_CONDUCT.md** — Contributor Covenant v2.1.
- **SECURITY.md** — vulnerability reporting policy with scope, response timeline, and credit.
- **Dependabot** — automated weekly updates for Go modules and GitHub Actions.
- **GitHub issue templates** — structured bug report and feature request forms.
- **Pull request template** — checklist for tests, lint, changelog, and screenshots.
- **`.editorconfig`** — cross-editor formatting standards for Go, YAML, Markdown, and Makefile.
- **`.gitattributes`** — line ending normalization and binary file markers.
- **macOS CI job** — test suite now runs on both Linux and macOS.
- **Codecov integration** — coverage uploads on push to main with badge in README.
- **Man page via Homebrew** — `man mg` now works after `brew install`.

### Security
- **CLI argument hardening** — added `--` separator before user-supplied positional args in mail, convoy, sling, and mutate commands to prevent flag injection.
- **ANSI stripping upgrade** — replaced hand-rolled CSI-only regex with `charmbracelet/x/ansi.Strip()` for full escape sequence coverage (OSC, DCS, APC).
- **Path traversal guard** — `.beads/redirect` resolution now rejects paths containing `..` components.
- **`--path` flag sanitization** — applies `filepath.Clean` before use.
- **govulncheck in CI** — dependency vulnerability scanning on every push and PR.
- **Debug log permissions** — restricted from 0644 to 0600.
- **Error message sanitization** — raw stderr in toast notifications truncated to first line (max 200 chars) to avoid leaking internal paths.
- **`.gitignore` hardening** — added `.env`, `.pem`, `.key`, `credentials.json` patterns.

### Changed
- **Man page updated** — reflects current features (v0.12.1): CLI mode as preferred data source, all flags and env vars documented, `gt(1)` in SEE ALSO.
- **Linters expanded** — golangci-lint now runs `errcheck`, `staticcheck`, `gosec`, and `unused` in addition to `gocritic` and `misspell`.

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
