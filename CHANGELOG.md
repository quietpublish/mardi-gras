# Changelog

All notable changes to Mardi Gras are documented here. For full release details including binaries and install instructions, see the [Releases](https://github.com/quietpublish/mardi-gras/releases) page.

## Unreleased

Gas City integration. Mardi Gras can now drive [Gas City](https://github.com/gastownhall/gascity) (`gc`) — Gas Town's pack-based successor — through its Supervisor HTTP API, alongside the existing Gas Town CLI backend. It's opt-in via `MG_GC_API`; without it, behavior is unchanged.

### Added
- **`Driver` interface seam** ([#57](https://github.com/quietpublish/mardi-gras/pull/57)) — the orchestrator is abstracted behind a `gastown.Driver` interface. `GTDriver` wraps the existing `gt` CLI 1:1 (no behavior change); the app routes every orchestrator call through a single driver selected at startup. This is the abstraction that makes a second backend possible.
- **Gas City read path** ([#60](https://github.com/quietpublish/mardi-gras/pull/60)) — `GCDriver` speaks the Gas City Supervisor HTTP API via an `oapi-codegen`-generated client (pinned to the gascity v1.2.1 spec). Opt in with `MG_GC_API` — a base URL, or `auto` to discover the supervisor's port — and optionally `MG_GC_CITY` to pin a city. Brings the live agent roster over HTTP instead of the CLI.
- **Gas City mail + formulas** ([#59](https://github.com/quietpublish/mardi-gras/pull/59)) — mail inbox/read/reply/send/archive/mark-read and formula listing over the Supervisor API, with the required `X-GC-Request` anti-CSRF header on every mutation. New `make gc-client` target regenerates the client.
- **Gas City nudge + decommission** ([#64](https://github.com/quietpublish/mardi-gras/pull/64)) — `n` (nudge) and `K` (decommission) work on the Gas City backend: mg resolves the roster agent to a live session (`GET .../sessions`) and submits a message or kills the session. Validated against a live `gc` supervisor.

### Fixed
- **Gas City API discovery + formula scope** ([#61](https://github.com/quietpublish/mardi-gras/pull/61)) — found by validating `GCDriver` against a live `gc` v1.2.1 supervisor. The Supervisor API binds a *dynamically assigned* port (not a fixed one) and its control socket isn't HTTP, so `MG_GC_API=auto` now reads the live address from `~/.gc/supervisor.log`. The formula listing also now sends the required `scope_kind`/`scope_ref` parameters.

### Notes
- Gas City support is opt-in. Roster, mail, formulas, nudge, and decommission work; agent dispatch (sling) and convoys are not yet wired to Gas City — use Gas Town for those. Vitals/costs/patrol have no Gas City equivalent. See [docs/gascity.md](docs/gascity.md) for the full capability matrix.

## v0.23.0 (2026-05-28)

The approvals release. The MCP launch path no longer silently auto-approves every shell command and patch under `workspace-write` — `M`-key sessions now prompt before codex runs an exec or applies a patch, making the MCP path strictly safer than the tmux path.

### Added
- **Codex MCP exec/patch approval modal** — when an `M`-key (human-present) session is running and codex wants to run a shell command or apply a patch, mg surfaces a centered modal with the command (or changed-file list), the cwd / reason, and four choices: *Approve once*, *Approve for this session*, *Deny*, *Abort turn*. The transcript keeps streaming under the modal while the user decides. The load-bearing protocol detail: codex sends approvals as a **server-initiated `elicitation/create` JSON-RPC request** with a server-allocated id, separate from the `codex/event` render hint — mg must reply on that request's id (not the event id, and not via a new `tools/call`). `internal/codexmcp/client.go::readLoop` previously dropped these silently because it had no branch for server-initiated requests; the new branch routes them to a `ServerRequests()` channel, and `Respond` echoes the raw id verbatim. The codex MCP `elicitation` capability is now advertised in the `initialize` handshake. `M`-key launches default to `approval-policy=on-request` (polecat / `gt sling` / tmux launches stay `never` — no human at the terminal). A per-session queue handles back-to-back approvals; unsupported elicitations auto-deny so the agent loop can't stall. Implements [#48](https://github.com/quietpublish/mardi-gras/issues/48) ([#52](https://github.com/quietpublish/mardi-gras/pull/52)).

### Fixed
- **Latent `readLoop` crash on non-integer JSON-RPC ids** — `response.ID` was `*int`, so any inbound message whose id wasn't a JSON number (allowed by the MCP spec, used by some servers) would have failed the whole-line decode and killed the read loop, taking the client and the in-flight session with it. Unreachable before this release because mg pinned `approval-policy=never` and codex's MCP server didn't send anything that would carry a string id — but became reachable the moment `elicitation/create` started flowing. Hardened: `response.ID` is now `json.RawMessage`, our outbound request ids stay int-allocated, and server-request ids are echoed verbatim on the reply. Regression locked in via `TestServerRequestStringIDSurvivesAndEchoes`.

### Changed
- **`github.com/sahilm/fuzzy` 0.1.1 → 0.1.2** — pulls in `FindFrom` respecting original input order on score ties ([upstream #28](https://github.com/sahilm/fuzzy/pull/28)) plus a NUL-rune crash guard. mg feeds pre-sorted slices into `FindFrom` (parade order for issues, definition order for palette commands), so the tie-break is now the intended order rather than arbitrary. Strict improvement for the `/` filter and command palette. ([#33](https://github.com/quietpublish/mardi-gras/pull/33))

### Deferred / known
- **Codex 0.134.0 guardian flow** — empirical probes against codex 0.134.0 (workspace-write+untrusted; read-only+on-request) showed that in current codex, most approvals are mediated by the **guardian** flow (`guardian_assessment` + `guardian_warning` → `exec_command_begin`) rather than by `elicitation/create`. The elicitation path is still wired in the native binary (`strings` confirms the `elicitation/create`, `codex_elicitation`, `exec-approval`, `patch-approval`, `codex_command`, `codex_cwd`, `codex_changes`, `codex_reason` symbols are all present in 0.134.0), and the new modal is correctly shaped for when it fires — but for typical 0.134.0 coding sessions the modal may fire less often than the design doc anticipated. A follow-up issue will scope routing the guardian flow (different event shape, different reply Op) through the same modal so the UX is preserved on current codex.
- **Strand risk on unforwarded approval types** — `request_permissions`, `request_user_input`, and `elicitation_request` are still emitted only as `codex/event` notifications, not forwarded as `elicitation/create` requests (`codex_tool_runner.rs` has a `TODO`). An `on-request` session that hits one of those would stall waiting on a reply mg can't send. Common coding sessions don't hit them; if it surfaces, the fallback is to default to `never` and expose `on-request` as opt-in (the plumbing still ships).
- **Amendment decision variants and inline diff viewer** — the lean-minimum scope deliberately offers only `approved` / `approved_for_session` / `denied` / `abort`. The amendment variants (`approved_execpolicy_amendment`, `network_policy_amendment`) carry structured payloads unverified upstream and are deferred. The patch modal shows the changed-file list only; an inline diff viewer is deferred.

## v0.22.0 (2026-05-17)

The replies release. Codex MCP sessions are no longer one-shot — press `r` in the transcript overlay to continue the conversation against the same `threadId` without spawning a fresh subprocess.

### Added
- **Codex MCP follow-up replies (Phase 2 of [#40](https://github.com/quietpublish/mardi-gras/issues/40))** — the transcript overlay is no longer one-shot. Press `r` while the overlay is open and the prior turn has terminated to send a follow-up prompt against the existing codex conversation via the `codex-reply` tool. The codex subprocess is reused, so replies are near-instant: in the real-codex integration test the first turn took 26–38s (cold sub-MCP startup) and the reply turn took **1.6–2.0s** — a 13–24× speedup. The transcript naturally interleaves both sides of the conversation via the existing `user_message`/`agent_message` rendering. The reply input lives in the bottom bar (same pattern as `mailReplyInput`); `enter` submits, `esc` cancels. Gated on `Status != "running"` and a non-empty `ThreadID`; mid-turn replies surface a toast rather than racing the prior `tools/call`. Implements [#47](https://github.com/quietpublish/mardi-gras/issues/47) ([#50](https://github.com/quietpublish/mardi-gras/pull/50)). Phase 3 (approval routing) and Phase 4 (mg-restart resume) carved off into [#48](https://github.com/quietpublish/mardi-gras/issues/48) and [#49](https://github.com/quietpublish/mardi-gras/issues/49).

### Fixed
- **Reply path repeats the v0.21.0 launch-ctx bug** — caught during the [#50](https://github.com/quietpublish/mardi-gras/pull/50) `/simplify` review. `codexReplyCmd`'s 30s ctx timeout propagated into `awaitResponse` and killed the reply session the instant the dispatch goroutine returned. `CodexMCPHandle.Reply` now uses `context.Background()` for `StartReplySession`, mirroring `LaunchCodexMCP`'s detachment. Regression locked in via `TestReplyCtxDoesNotKillSession`. The integration test passed before the fix only because it bypassed `codexReplyCmd` and called `Reply(context.Background(), ...)` directly.

## v0.21.1 (2026-05-16)

### Fixed
- **Codex MCP transcript stuck at "waiting for first event…"** — the transcript overlay opened, status displayed `running`, and the elapsed timer ticked, but no events ever rendered even though codex was streaming them on the wire. Two collaborating bugs caused this:
  1. `internal/app/codex.go::codexLaunchCmd` ran a `defer cancel()` on the 90s launch context. `internal/agent/codex_mcp.go::LaunchCodexMCP` then handed that same context to `Client.StartSession`, which parented its session-wide `callCtx` to it. As soon as the launch goroutine returned `codexLaunchedMsg`, the deferred cancel fired and `awaitResponse` picked `<-ctx.Done()`, pushing `Err: context.Canceled` onto the session's done channel — the session died before the first event was rendered. Fix: `LaunchCodexMCP` now passes `context.Background()` to `StartSession`; the launch ctx still bounds the handshake (Dial), but the session itself outlives the launch goroutine and lives until `CodexMCPHandle.Close()`.
  2. `codexNextEventCmd`'s select had two ready cases when the session terminated (`Events` closed AND `Done` populated). Go picks pseudo-randomly, so half the time the closed-Events branch won, returned `codexEventMsg{done: true}`, and the handler returned `nil` without scheduling another reader — the terminal result on `Done` was never delivered and the UI stayed at `running` indefinitely. Fix: when `Events` returns closed, block on `Done` in the same Cmd. `awaitResponse` always pushes to `Done` before `signalStop` closes events, so the read returns immediately.
- **Companion fix in `internal/codexmcp/session.go::awaitResponse`** — added a `<-s.client.Done()` arm so the goroutine unblocks when the subprocess exits before responding. Without it, the new background-context lifetime meant `awaitResponse` could block forever on `respCh` if codex crashed mid-session.

Regression test in `internal/agent/codex_mcp_test.go::TestLaunchCtxDoesNotKillSession` reproduces the original failure mode and asserts events + terminal result both still flow after the launch ctx is canceled.

## v0.21.0 (2026-05-16)

The MCP release. Adds first-class Model Context Protocol support so mg can speak directly to `codex mcp-server`, surfacing live agent state in the TUI instead of black-boxing a tmux pane.

### Added
- **Codex MCP integration (Phase 1)** — mg now speaks the Model Context Protocol over stdio to `codex mcp-server`, letting it surface live agent state inside the TUI rather than black-boxing a tmux pane. Press `M` on a selected issue to spawn a codex session via MCP and open a live transcript overlay that renders `agent_message`, `exec_command_*`, `mcp_tool_call_*`, `task_started/complete`, and `error` events as they arrive. The tmux dispatch path (`a`) is unchanged. New `internal/codexmcp` package implements a focused JSON-RPC client + typed event envelopes + session wrapper (race-tested, integration-tested against real codex 0.130.0). Subprocess lifecycle: codex is spawned via stdio pipes, terminated cleanly on app quit via `Model.Cleanup()`. Deferred to follow-ups: interactive replies on a running session, exec/patch approval routing, resume-on-restart. ([#40](https://github.com/quietpublish/mardi-gras/issues/40) / [#43](https://github.com/quietpublish/mardi-gras/pull/43))

## v0.20.0 (2026-05-13)

The codex release. Adds OpenAI Codex as a third agent runtime alongside Claude Code and Cursor, with first-class Gas Town routing and a session-resume palette action for codex-specific workflows.

### Added
- **Codex as a third agent runtime** — `codex` is now detected on PATH alongside `claude` and `cursor-agent`. Default detection order is claude → cursor-agent → codex, and the existing `MG_AGENT_RUNTIME` env var / `--agent` flag now accept `codex` as a value. mg launches codex with `--sandbox workspace-write -a on-request -C <projectDir>` (plus `--no-alt-screen` inside tmux) so unattended agents don't block on permission prompts. Docs cover first-run auth (`codex login`), project-trust gating, nvm install caveats, and the AGENTS.md ecosystem. ([#39](https://github.com/quietpublish/mardi-gras/pull/39))
- **Gas Town sling routing for codex** — when mg's active runtime is codex and `gt` is on PATH, sling dispatches `gt sling --agent codex <id>` so `MG_AGENT_RUNTIME=codex` propagates into Gas Town. Requires gt v1.1.0+ which added first-class `--agent` support. Claude/cursor behavior with gt is intentionally unchanged. ([#41](https://github.com/quietpublish/mardi-gras/pull/41))
- **"Resume last Codex session" palette entry** — visible when codex is the active runtime inside tmux. Launches `codex resume --last --no-alt-screen -C <projectDir>` in a new tmux split. Gated on a rollout file actually existing under `~/.codex/sessions/YYYY/MM/DD/*.jsonl` so a never-launched codex doesn't surface as a confusing empty pane. ([#41](https://github.com/quietpublish/mardi-gras/pull/41))

### Changed
- **Test coverage raised from 64% → 71.6%** — backfill of high-ROI tests across `internal/ui`, `internal/data`, `internal/gastown`, and `internal/views`. No behavior change. ([#38](https://github.com/quietpublish/mardi-gras/pull/38))

### Deferred
- **Codex MCP-server integration** ([#40](https://github.com/quietpublish/mardi-gras/issues/40)) — codex exposes an `mcp-server` stdio mode that mg could speak to and surface live agent state (tool calls, messages) inside the TUI rather than black-boxing a tmux pane. Substantially bigger scope (MCP client in Go, BubbleTea message routing, transcript UI), filed as a separate issue.

## v0.19.0 (2026-05-13)

### Added
- **`label:` filter token** — the `/` filter now accepts `label:foo` alongside `type:` and `priority:`. Case-insensitive exact match against issue labels, AND across tokens (`label:backend label:security` matches issues carrying both), OR within an issue (matches if any of the issue's labels equals the value). Mirrors the v0.18.0 `--exclude-label` flag's semantics so the inline filter and the launch-time flag behave symmetrically. ([#30](https://github.com/quietpublish/mardi-gras/issues/30) / [#35](https://github.com/quietpublish/mardi-gras/pull/35))
- **`MG_AGENT_RUNTIME` env var + `--agent` flag** — pick between Claude Code and Cursor when both are installed, instead of relying on the hardcoded claude-first detection order. Accepts `claude` or `cursor` (case-insensitive). If the requested binary isn't on PATH, mg falls back to the default detection order rather than failing silently — so a stale env var never leaves you with no runtime. The override applies only to the local launch path; Gas Town `gt sling` dispatch continues to choose the runtime per formula. ([#29](https://github.com/quietpublish/mardi-gras/issues/29) / [#36](https://github.com/quietpublish/mardi-gras/pull/36))

### Fixed
- **Detail viewport no longer snaps to top during polls** — when reading a long issue, the 5-second CLI / 1.2-second JSONL refresh tick was calling `Viewport.GotoTop()` unconditionally inside `SetIssue`, scrolling the user back to the top every poll. `SetIssue` now only resets scroll position when the displayed issue ID actually changes, matching the existing behavior of `SetMolecule` / `SetComments` / `SetRichDetail` / `SetSize`. Switching to a different issue still snaps to top. Fix also covers the Gas Town status poll path (`propagateAgentState`). Thanks @fixpunkt for the report, root-cause, and clean fix in their first PR. ([#31](https://github.com/quietpublish/mardi-gras/issues/31) / [#32](https://github.com/quietpublish/mardi-gras/pull/32))

## v0.18.0 (2026-05-13)

### Added
- **`--exclude-label` CLI flag** — hide issues carrying specific labels from the parade and status output, mirroring `--exclude-type`. Case-insensitive match. Issues with no labels are always kept. Useful for suppressing `gt:agent` bot-tracked beads. Parallels the upstream [beads `bd ready --exclude-label`](https://github.com/gastownhall/beads/commit/34a580c) that landed 2026-04-24.
- **`bd prune` palette actions** — new command-palette entries for `bd prune --older-than 30d --dry-run` (preview) and `bd prune --older-than 30d --force` (delete). Tees into the upstream [beads `bd prune`](https://github.com/gastownhall/beads/pull/3353) command. Available when bd v1.1+ is installed; older bd will surface a command-not-found error toast.
- **`Claim next ready` palette action** — atomically claims the highest-priority ready bead via `bd ready --claim --json` and selects it in the parade. Replaces the two-step "find ready, then `bd update --claim`" flow with a single CAS-protected call. Available on bd v1.0.4+ ([beads#3578](https://github.com/gastownhall/beads/pull/3578)); older bd surfaces an "unknown flag" toast.
- **Gas City detection (informational)** — `gastown.Detect()` now also reports `gc` binary availability and the nearest `city.toml` ancestor path. No behavior change; groundwork for a future Gas City driver. Driven by the Gas City v1.0 rewrite announcement.
- **`patrolling` agent state** — Gas Town v1.1.0 main exposes a typed `AgentState` enum that promotes `patrolling` to a first-class state (witness/deacon scanning rounds). mg now renders it with a sky-blue badge (`StatePatrolling = #5DADE2`) and a `⊙` symbol, distinct from `idle` and `working`. Previously patrolling agents fell through to idle styling.

### Changed
- **Pin `BD_JSON_ENVELOPE=0` for `bd` subprocesses** — defensive against the upcoming beads v2.0 default where `--json` output will be wrapped as `{schema_version, data}`. Set in `internal/data/exec.go` and `internal/gastown/exec.go` so a user's shell setting can't flip bd output into a shape mg doesn't yet parse. Migration before bd v2.0 will add envelope-aware unmarshalling.
- **Pin `BD_DOLT_AUTO_COMMIT=off` for read-only `bd` subprocesses** — `internal/data/exec.go` now allowlists read-only bd subcommands (`list`, `show`, `context`, `doctor`, `--version`, plain `ready`, `prune --dry-run`, and `comments` read) and pins `BD_DOLT_AUTO_COMMIT=off` for them. Without this, each read fires a no-op `dolt_commit()` that opens a fresh connection and fails with "nothing to commit". Mutating calls keep bd's auto-commit default. Mirrors the upstream gas town pattern from [gastown GH#3596](https://github.com/steveyegge/gastown/issues/3596).
- **Dependencies updated** — `charmbracelet/ultraviolet` dated bump (2026-04-16 → 2026-04-22 → 2026-04-28).

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
