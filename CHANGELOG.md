# Changelog

All notable changes to Mardi Gras are documented here. For full release details including binaries and install instructions, see the [Releases](https://github.com/quietpublish/mardi-gras/releases) page.

## v0.31.0 (2026-08-29)

A community release: the first outside bug report and the first outside code contribution both land here.

### Fixed
- **Parade indentation follows real parent-child relationships, not dotted IDs** ([#104](https://github.com/quietpublish/mardi-gras/pull/104), fixing [#102](https://github.com/quietpublish/mardi-gras/issues/102)) — mg indented rows by counting dots in the issue ID, so an issue reparented to the top level while keeping its hierarchical ID stayed indented, appearing to be a child of whatever unrelated row happened to sit above it. Indentation now walks the current `parent-child` dependency chain, stopping at a missing parent or a cycle so a row is never nested under an ancestor mg cannot see. Reported by [@dcaixinha](https://github.com/dcaixinha), fixed by [@pttydou](https://github.com/pttydou).

### Changed
- **Go dependencies and GitHub Actions updated** ([#109](https://github.com/quietpublish/mardi-gras/pull/109)) — bubbletea 2.0.6 → 2.0.9, lipgloss 2.0.3 → 2.0.6, bubbles 2.1.0 → 2.2.1, oapi-codegen/runtime 1.4.1 → 1.5.0, sahilm/fuzzy 0.1.2 → 0.1.3; checkout, setup-go and codecov-action all v6 → v7. Transitively this moves go-runewidth 0.0.23 → 0.0.27 and x/ansi 0.11.7 → 0.11.8 — the libraries behind the single-row toast truncation added in v0.30.1 — so that path was re-verified by rendering rather than assumed from a green build.
- **`Issue.NestingDepth` removed.** The indentation fix above left it with no production caller. `Issue.ParentID` remains, since the detail panel's epic progress still parses dotted IDs.

### Known issues
- **Cross-section nesting and collapse/expand are not addressed** ([#110](https://github.com/quietpublish/mardi-gras/issues/110)) — when a parent and child fall in different parade sections, the child is still indented beneath whatever precedes it in its own section. This predates [#104](https://github.com/quietpublish/mardi-gras/pull/104); `main` behaved identically with dotted IDs. The collapse/expand keybinding requested in [#102](https://github.com/quietpublish/mardi-gras/issues/102) is also still open, as is the same ID-shape assumption in the detail panel's epic progress count.

## v0.30.1 (2026-08-29)

A correctness release out of an upstream check: six fixes, then a documentation audit that found a seventh bug and a broken security-disclosure path.

### Fixed
- **Schema-skew advice was backwards for anyone on a healthy upgrade** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — bd v1.3.0 migrates the store v53 → v66, which gives `schema version mismatch` a second, entirely legitimate cause. The hint assumed only the accidental v1.2.0/v1.2.1 releases and sent every user to the rollback runbook; for a v1.3.0 fleet the database is fine and the *binary* is stale, so rolling the schema cursor back would damage a correct upgrade. It now reads the version out of bd's own message: v65 means roll back, v66+ means upgrade the lagging clients forward, and an unparseable message keeps both remedies on screen rather than guessing.
- **mg picked a backend it could see was wrong** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — `detect.go` has always computed whether `gc` is on PATH and whether a `city.toml` sits above the working directory, and nothing ever read either value. A user inside a real Gas City workspace with no `gt` installed got a Gas Town driver that shelled out to a binary they did not have. Backend selection now uses that evidence, in strict precedence: an explicit `MG_GC_API` wins, then any Gas Town evidence, then Gas City evidence, then the historical default. The third rule fires only when there is no sign of Gas Town at all, so no existing user's backend changed and the global default is still Gas Town.
- **Issue comments were unavailable on Gas City** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — the driver returned "unsupported" on the grounds that the Supervisor API has no comments endpoint. That reasoning was itself the bug: comments come from `bd comments`, a Beads call that works identically whichever orchestrator is running, so Gas City users lost an entire panel for data mg could already read.
- **A long error toast pushed the layout off the bottom of the screen** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — the toast occupies a single divider row, but it was rendered with a width that sets a *minimum* and wraps anything longer. A real 362-character bd error rendered as five rows at 80 columns and three at 120, hiding the entire footer — the help, palette and filter hints, and the data-source line — at exactly the moment something had gone wrong. It now truncates to one row, measuring display cells rather than bytes so emoji and box-drawing characters count as the terminal renders them.
- **An unreachable orchestrator spun forever instead of reporting itself** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — two independent causes: the status handler discarded its error, and the loading check never consulted whether a poll was actually in flight. A missing `gt` binary and a supervisor that was not listening both rendered as an endless spinner, indistinguishable from the genuinely slow case that line exists for. The panel now shows the underlying error and a backend-specific remedy, keeps any status it already had, and returns to the spinner while a retry runs.
- **`a` did nothing on a single issue without a local agent runtime** ([#107](https://github.com/quietpublish/mardi-gras/pull/107)) — the runtime guard sat ahead of the sling branch, while the multi-select and Gas City branches ran before it, so the two selection modes disagreed: a multi-selection dispatched fine and a single issue silently did nothing. Only direct launch needs an agent binary on the machine, because that is the path where mg execs it; the orchestrator starts the agent itself. Focusing an already-running agent's pane no longer requires the runtime either.
- **`mg --help` and the in-app help described things mg does not do** ([#106](https://github.com/quietpublish/mardi-gras/pull/106)) — `--agent` was advertised as "claude or cursor" though codex has been supported for some time, and the help overlay said `a` opens a tmux *window* when it opens a split *pane*.

### Changed
- **`bd show` is asked for `--brief-deps` where the binary accepts it** ([#105](https://github.com/quietpublish/mardi-gras/pull/105)) — the detail panel's fetch inlines every dependency at full body; upstream measured one response at 214 KB of which 193 KB was the dependencies array. mg parses that array and throws it away, because `bd show` and `bd list` return different dependency shapes and only `list`'s matches what mg models. A latching probe sends the flag once and falls back cleanly on older bd, so this costs one wasted call per process rather than a doubled subprocess on every open.
- **Every user-facing document was audited against the code** ([#106](https://github.com/quietpublish/mardi-gras/pull/106)) — claims were verified at the source rather than trusted from the prose. Two docs were wrong within the hour because of the fixes above; others had drifted further. README badges advertised Beads ≥ v0.60 and Gas Town ≥ v0.12 against a GitHub org that has since moved, and are now dropped rather than bumped, since nothing in the code gates on either number and mg already reports version problems at runtime. Gas City is no longer described as Gas Town's successor. `keybindings.md` misdescribed four keys and omitted whole sections; `filtering.md` missed the `label:` token and wrongly claimed AND semantics for free text.
- **Private vulnerability reporting is enabled on the repository** ([#107](https://github.com/quietpublish/mardi-gras/pull/107)) — the advisory link in `SECURITY.md` was reachable only by users with admin access, so an outside reporter following the documented route received a 404. The workaround it had accumulated — open a public issue with no details — is removed.

## v0.30.0 (2026-08-17)

Two more Gas City capabilities, unlocked by the v1.4.1 API bump in v0.29.1.

### Added
- **Create & assign to crew works on Gas City** ([#100](https://github.com/quietpublish/mardi-gras/pull/100)) — previously a Gas Town-only action. Gas City takes the assignee inline when the bead is created, so unlike a create-then-assign pair the issue is never briefly unowned. With nudge enabled the crew member's session is woken afterwards; if only the nudge fails, mg reports the partial success rather than implying the issue was not created. The create form's crew field now appears whenever any orchestrator is live, not only when the `gt` binary is present.
- **Convoy create-from-epic works on Gas City** ([#100](https://github.com/quietpublish/mardi-gras/pull/100)) — Gas City has no `--from-epic` flag, so mg walks the epic's dependency graph and enrols its members, excluding the epic itself. An epic with no members is reported as an error instead of producing an empty convoy.

### Changed
- **`docs/gascity.md` now records *why* each unsupported operation is unsupported** ([#100](https://github.com/quietpublish/mardi-gras/pull/100)) — every remaining `ErrUnsupported` operation was checked against the v1.4.1 spec. Comments have no endpoint and beads carry no comments field; `sling` is create-only so there is no unsling; `close` takes no cascade parameter; convoy land/watch/unwatch have no endpoint. The molecule DAG remains unsupported for a subtler reason: Gas City's runs are *formula* runs and its bead graph has no tier structure, so it is a modelling question rather than missing wiring.

## v0.29.1 (2026-08-17)

A Gas City correctness release: features that only exist in Gas Town no longer pretend to work on a Gas City backend.

### Fixed
- **Gas Town-only features are no longer offered on Gas City** ([#98](https://github.com/quietpublish/mardi-gras/pull/98)) — three operations call `gt` directly rather than going through the driver seam, and each failed badly on a Gas City backend instead of cleanly. "Recover dead rigs" appeared in the command palette and then died with `exec: "gt": executable not found`; handoff blamed tmux for what was actually a missing binary; and the recent-activity section silently never appeared, because it reads `~/gt/.events.jsonl` off local disk. They are now gated like vitals/costs/patrol already were — recovery is hidden, handoff explains itself, and the activity feed returns empty without looking for a file that cannot exist.
- **Stopping an agent no longer reports a false success on Gas City** ([#98](https://github.com/quietpublish/mardi-gras/pull/98)) — `shift+A` gated on the `gt` binary, so on Gas City it fell through to killing the tmux window and reported success while the orchestrator still held the agent. It now asks the active driver to unsling and surfaces a truthful "not supported" when the backend cannot.
- **Dead-rig advice no longer names a binary you may not have** ([#98](https://github.com/quietpublish/mardi-gras/pull/98)) — the suggested fix was hardcoded `gt sling <issue> <rig>`. It is now backend-specific, and the Problems overlay simply omits the command rather than printing wrong advice.

### Changed
- **Pinned Gas City Supervisor spec refreshed to v1.4.1** ([#98](https://github.com/quietpublish/mardi-gras/pull/98)) — the committed OpenAPI spec was from 2026-06-12; the generated client now covers 127 paths (up from 111, none removed). Error handling decodes the raw problem+json response body instead of a generated typed field, so it survives future regenerations regardless of how the spec models its errors.
- **`docs/gascity.md` capability table corrected** ([#98](https://github.com/quietpublish/mardi-gras/pull/98)) — it previously listed only convoy land/watch/unwatch and vitals/costs/patrol as unsupported, omitting comments, unsling, cascade close, crew assign, the molecule DAG trio, and convoy create-from-epic.

## v0.29.0 (2026-08-17)

A safety release prompted by an accident upstream: beads published two untested versions, and mg now tells you when it is talking to one of them.

### Added
- **Warning when bd is a known-bad release** ([#96](https://github.com/quietpublish/mardi-gras/pull/96)) — beads published **v1.2.0 and v1.2.1 by accident** on 2026-08-11 without release testing. Running either one *even once* migrates the local Dolt schema v53 → v65, after which every other bd binary halts with `schema version mismatch`. mg now warns on startup when it is talking to one of them, and says which version to move to. The version comes from the `bd context --json` fetch that already runs at startup, so the check costs no extra subprocess. [beads v1.2.2](https://github.com/gastownhall/beads/releases/tag/v1.2.2) is the recovery release; it re-issues the tested 1.1.2 code under a higher version number.
- **Comment count in the parade** ([#96](https://github.com/quietpublish/mardi-gras/pull/96)) — issues carrying discussion now show a muted `💬N` after the priority badge, so you can see where the conversation is without opening anything. `bd list --json` has returned `comment_count` since bd v1.1.0 and mg was discarding it, so this costs no extra CLI call. The detail panel continues to count the comments it actually fetches.

### Fixed
- **Schema-skew errors point somewhere useful** ([#96](https://github.com/quietpublish/mardi-gras/pull/96)) — when the Beads database is ahead of the bd binary reading it (the v1.2.1 aftermath), mg answered with "Ensure the Dolt server is running", which is the wrong place to look. It now names the skew, links the recovery guide, and mentions the `BD_IGNORE_SCHEMA_SKEW=1` stopgap. Every other `bd list` failure keeps the original advice.

## v0.28.1 (2026-07-29)

A one-fix patch release: typing in the filter no longer quits the app.

### Fixed
- **`q` and `?` are literals in the filter input** ([#91](https://github.com/quietpublish/mardi-gras/issues/91)) — `handleFilteringKey` intercepted `q` as quit and `?` as help before the keypress reached the text input, so a query containing either was truncated there and `q` killed the app mid-typing (filtering for `label:asdq` exited instead of entering the `q`). The filter now intercepts only `esc`/`enter` and forwards the rest, matching every other input mode in the app. `ctrl+c` still quits from the filter; help stays reachable with `?` from the parade and via the command palette.

## v0.28.0 (2026-07-24)

A design-quality release driven by a screenshot-based audit of every surface ([#89](https://github.com/quietpublish/mardi-gras/pull/89)) — before/after pairs in `docs/screenshots/design-audit/`.

### Fixed
- **Selection rendering** — cursor rows in the parade, agent roster, convoys, mail, and command palette now paint clean full-row highlights. Previously inner style resets punched holes in the background, leaving a detached block at the row edge (and a wrapped orphan bar on narrow panes). Shared mechanism: `ui.FillBackground`/`ui.SelectedRow`.
- **Footer pushed off-screen** — a latent layout bug rebuilt the detail viewport at full pane height behind `SetSize`'s back; now routed through `Detail.ResetViewport` and guarded by a screen-height regression test.
- **Command palette rows wrapped** — the right-aligned hotkey overflowed the box by a hair, rendering every hotkey on its own line. Items are now single-line with a leading key chip and fuzzy-match highlighting.

### Changed
- **Truncation favors titles** — issue titles get a guaranteed width floor; blocker hints degrade to id-only (`→ vv-001`) and overdue badges compress (`▲151d`) before a title loses characters.
- **Responsive Gas Town roster** — narrow panes shrink the Role column, then drop the sparkline, Work column, and session tags; `[Session]` tags that just restate the agent name are gone everywhere.
- **Better orientation** — persistent `⚜ FOCUS` footer badge, live filter match count, detail scroll-position cue, humanized ages (`5w ago`, not `969h ago`), footer hints elide whole chips instead of clipping mid-keyword, header counts pair tightly in parade order, and the progress bar is labeled (`✓13%`).
- **Calmer hierarchy** — closed issues render muted, markdown H2–H4 show slim bars instead of literal `##`, expanded mail meta collapses to one line, spare parade space carries a status legend, and toasts occupy the divider row so key hints stay visible.
- **Forms** — create/edit dialogs are content-fit centered modals with complete hints and type/priority options in their semantic colors.

### Docs
- All committed screenshots (`demo.gif`, Gas City panels, light theme) regenerated against the new UI.

## v0.27.0 (2026-07-23)

A light theme release. Mardi Gras now adapts to light-background terminals ([#86](https://github.com/quietpublish/mardi-gras/issues/86)) — the palette, every derived style, the markdown theme, and all overlay surfaces follow the active theme.

### Added
- **Light theme with terminal background auto-detection** ([#87](https://github.com/quietpublish/mardi-gras/pull/87)) — a full light palette selectable via `--theme light|dark|auto` or `MG_THEME`, defaulting to auto: the terminal background is queried (OSC 11) before the TUI starts, falling back to dark on pipes or unresponsive terminals. The brand look survives the flip — purple/gold/green stay, with inks darkened for light backgrounds; the glamour markdown theme and Gas Town panel follow suit.

### Fixed
- **Overlay background stripes on light terminals** ([#87](https://github.com/quietpublish/mardi-gras/pull/87)) — all overlay surfaces (help, command palette, create/edit forms, approval/recovery dialogs) now render through a shared `ui.OverlayBox` that re-asserts the box background after inner style resets. The stripes existed before but were invisible on dark terminals.

### Internal
- **Theme-switchable design system** ([#87](https://github.com/quietpublish/mardi-gras/pull/87)) — `internal/ui` styles, pre-rendered strings, and gradients rebake through `ui.SetTheme`; theme-invariant color aliases live in one place. New light-theme screenshot pipeline (`make screenshot-light`) with a shared vhs runner consolidating the capture scripts. Bumped `golang.org/x/text` to v0.39.0 (GO-2026-5970).

## v0.26.0 (2026-06-13)

A UI-polish release. The detail panel and the Gas Town panel get brand-cohesive refinements, the slow orchestrator poll no longer reads as frozen, and the README gains an animated demo backed by a reproducible screenshot pipeline.

### Added
- **Mardi Gras markdown theme in the detail panel** ([#73](https://github.com/quietpublish/mardi-gras/pull/73)) — issue description / design / notes / acceptance-criteria now render through a custom [glamour](https://github.com/charmbracelet/glamour) theme matched to the brand palette (gold-on-purple H1 banner, bright-purple subheadings, gold emphasis and links, green inline code) instead of the generic dark theme. Code-block highlighting and spacing are inherited from the dark base.
- **Pane focus indicator** ([#73](https://github.com/quietpublish/mardi-gras/pull/73)) — the detail pane's left border doubles as the pane divider and a focus cue: a dim-purple rule normally, brightening to gold when the detail pane holds focus (`Tab`), so it's obvious which pane the scroll/nav keys drive.
- **Convoy progress in the Gas City convoy list** ([#71](https://github.com/quietpublish/mardi-gras/pull/71)) — the Gas City (`MG_GC_API`) convoy list now shows per-convoy progress (closed/total), matching the Gas Town panel.

### Changed
- **Branded loading splash** ([#72](https://github.com/quietpublish/mardi-gras/pull/72)) — the bare `Loading...` shown before the first frame is now a branded splash (⚜ MARDI GRAS + an animated gold spinner via `bubbles/spinner` + "lining up the parade…"). Honors `--no-animations` (renders a static frame).
- **Animated Gas Town loading line** ([#74](https://github.com/quietpublish/mardi-gras/pull/74)) — `gt status --json` (and the Gas City status poll) can take seconds to tens of seconds; the panel's loading line now animates the gold spinner instead of a static message that reads as frozen. Driven by "the panel is open and waiting on a fetch", so it covers both the Gas Town and Gas City backends (the latter previously fell through to "not available" during its first fetch).

### Docs
- **Animated README hero GIF** ([#75](https://github.com/quietpublish/mardi-gras/pull/75)) — the static hero screenshot is replaced with an animated demo (browse the parade → focus the themed detail pane → open the Gas Town roster), captured against a fake Gas City supervisor so it needs no live `gt`/`gc`. Regenerate with `make demo-gif`.

### Internal
- **Fake Gas City supervisor + deterministic screenshot pipeline** ([#70](https://github.com/quietpublish/mardi-gras/pull/70)) — a canned Supervisor API stub (`testdata/fakegc`) plus `vhs` tapes and `make` targets (`make screenshots-gc`, `make demo-gif`) for repeatable, reviewable captures. The supervisor's `-delay` flag exercises the loading spinner.

## v0.25.0 (2026-06-13)

Completes the Gas City backend: agent dispatch, convoys, and activation without Gas Town installed. With this, mg drives Gas City over the Supervisor API for the full control surface (roster, mail, formulas, nudge, decommission, sling, convoys) — opt in with `MG_GC_API`; Gas Town remains the default.

### Added
- **Gas City works without Gas Town installed** — the agent control surface (panel, roster polling, sling/nudge/decommission/convoys/mail, header/footer indicators) now activates when a Gas City supervisor is configured (`MG_GC_API`) even if `gt` is not on PATH. Previously every orchestration feature was gated on the `gt` binary; it's now gated on "an orchestrator is reachable" (`gt` **or** the Gas City driver). No change for Gas Town users. Patrol scan stays Gas Town-only (no Gas City equivalent).
- **Gas City convoys + sling** — closes the remaining Gas City gaps. Convoys (`ctrl+g` panel) list/create/close over the Supervisor API, where Gas City models a convoy as a bead. Agent dispatch (`a`) works too: because Gas City requires an explicit target agent (unlike gt's auto-pick), `a` on the Gas City backend prompts for a target before slinging. `convoy land`/`watch`/`unwatch` have no Gas City endpoint and stay unavailable there. Validated against a live `gc` v1.2.1 supervisor.

## v0.24.0 (2026-06-12)

The Gas City release. Mardi Gras can now drive [Gas City](https://github.com/gastownhall/gascity) (`gc`) — Gas Town's pack-based successor — through its Supervisor HTTP API, alongside the existing Gas Town CLI backend. It's opt-in via `MG_GC_API`; without it, behavior is unchanged. (This release also carries the codex MCP approval routing prepared as v0.23.0, which was never tagged — see below.)

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
