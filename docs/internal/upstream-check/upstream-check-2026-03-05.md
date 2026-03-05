# Upstream Check — 2026-03-05

## TL;DR

Quiet day — no new releases on either repo. Beads main has ~11 post-v0.58.0 commits (docs, Nix flake, doctor warning suppression, idle-monitor fix, DerivePort regression fix). Gas Town main has ~14 non-backup commits post-v0.10.0 (hook_bead removal confirmed, cursor hooks merged, enriched convoy dashboard, daemon pressure checks PR open, polecat list JSON state fix PR open). No breaking changes for mg. Two feature opportunities worth tracking: enriched convoy progress data and GitHub Issues tracker plugin (Beads PR #2373).

## Current baseline

- mg version: v0.6.1
- Beads: v0.58.0 (latest release, main ~11 commits ahead)
- Gas Town: v0.10.0 (latest release, main ~14 non-backup commits ahead)
- Previous check: [upstream-check-2026-03-04.md](upstream-check-2026-03-04.md)

## Breaking changes

### None for mg

All changes since the last check are additive or internal. The `hook_bead` removal flagged in the previous check has been confirmed merged (`fa9dc28`), but mg already handles this via hook data enrichment — no action needed.

## Feature opportunities

### 1. Enriched convoy dashboard data (Gas Town main)

**Commit**: `3b9b0f0` — "feat(dashboard): enrich convoy panel with progress %, ready/active counts, assignees"

**What**: Gas Town's dashboard template now includes `ProgressPct`, ready/active bead breakdown chips, and assignee chips for convoys. If these fields are exposed in `gt convoy list --json`, mg could show richer convoy information.

**How mg could use it**: Add progress percentage bars and assignee chips to the convoy section in the Gas Town panel. Currently mg renders convoy progress from issue status counts — this would give server-computed values.

**Effort**: Small. Check `gt convoy list --json` output after gt v0.10.1+ ships.

### 2. GitHub Issues integration (Beads PR #2373 — open)

**What**: Bidirectional sync between Beads and GitHub Issues via scoped labels. New `bd github sync/status/repos` commands.

**How mg could use it**: If an issue has a linked GitHub Issue, surface the link in the detail panel. Could also show sync status in the footer when GitHub integration is active.

**Effort**: Small-medium. Would need to detect the integration and parse linked issue metadata.

**Status**: PR is open, not merged yet. Track for future check.

### 3. Polecat list JSON state reconciliation (Gas Town PR #2379 — open)

**What**: Fixes `gt polecat list --json` to reconcile state with tmux session liveness (previously only plain output did this). Would make JSON state more accurate — e.g., `state=working` with dead session → `state=done`.

**How mg could use it**: More accurate agent states in the Gas Town panel without mg needing to do its own reconciliation. Currently `problems.go` detects zombies partly because JSON state is unreliable.

**Effort**: Zero — mg already consumes `gt status --json` which may benefit from the same reconciliation. Potential simplification of zombie detection in `problems.go`.

**Status**: PR is open. Monitor.

### 4. Daemon pressure checks (Gas Town PR #2370 — open)

**What**: Opt-in system resource pressure gating before agent spawns. Checks CPU/memory/disk before allowing new polecats.

**How mg could use it**: Surface pressure state in the Gas Town panel vitals section — "system under pressure" warning. Could also expose in problems view.

**Effort**: Small. Would need pressure data in `gt status --json` or `gt vitals`.

**Status**: PR is open. Monitor.

## Informational changes

### Beads (v0.58.0 → main)

| Commit | Description | Category |
|--------|-------------|----------|
| `7f41edd` | feat(nix): modernize flake for nixpkgs-25.11 | Packaging |
| `b4b586d` | feat(doctor): allow suppressing specific warnings via config | Quality of life |
| `d9a719e` | fix(doltserver): idle-monitor kills itself via Stop() | Bug fix |
| `0e286c0` | fix: restore DerivePort as standalone default in DefaultConfig | Bug fix (regression) |
| `ee6bcef` | fix(doltserver): log when Start() falls back to DerivePort | Observability |
| `95c85e3` | docs(doltserver): document birthday-problem collision recovery | Docs |
| `431d840` | fix(docs): disambiguate duplicate nvim-beads entry | Docs |
| `5ea0eaa` | feat: add OpenCode recipe to bd setup | Community |
| Various | Merge PRs for docs, community tools | Docs |

### Gas Town (v0.10.0 → main)

| Commit | Description | Category |
|--------|-------------|----------|
| `fa9dc28` | Remove agent bead hook slot: use direct bead tracking | Refactor (flagged) |
| `aa7dd7e` | Merge feat/cursor-hooks-support | Feature |
| `3b9b0f0` | feat(dashboard): enrich convoy panel with progress % | Feature |
| `330aec8` | feat: add context-budget guard as external script | Feature |
| `72798af` | fix(daemon): 5-minute grace period before auto-closing empty convoys | Bug fix |
| `39f7bf7` | fix: gt done uses wrong rig when Claude Code resets shell cwd | Bug fix |
| `fbfb3cf` | fix(dolt): server-side timeouts to prevent CLOSE_WAIT accumulation | Reliability |
| `65c0cb1` | fix(patrol): cap stale cleanup and break early on active patrol | Performance |
| `b1ee19a` | fix(tmux): refresh cycle bindings when prefix pattern is stale | Bug fix |

### Notable open PRs

**Beads:**
- **#2373**: GitHub Issues integration (tracker plugin) — bidirectional sync, big feature
- **#2368**: Deterministic ordering in SearchIssues and GetReadyWork — stability fix
- **#2370**: Warn on user-modified legacy hooks during migration
- **#2361**: Skip tombstone entries in `bd init --from-jsonl`

**Gas Town:**
- **#2379**: Polecat list JSON state reconciliation — more accurate agent states
- **#2377**: Resolve external tracked IDs during convoy launch collection
- **#2370**: Daemon pressure checks — opt-in resource gating
- **#2374**: Normalize double slashes in GT_ROLE parsing

## Recommended actions

| # | Action | Priority | Effort | Files |
|---|--------|----------|--------|-------|
| 1 | Monitor hook_bead removal post-upgrade | medium | zero | `internal/gastown/status.go` |
| 2 | Track enriched convoy data for panel | low | small | `internal/views/gastown.go` |
| 3 | Track GitHub Issues integration PR | low | — | — |
| 4 | Track polecat JSON state fix PR | low | zero | `internal/gastown/problems.go` |
| 5 | Track daemon pressure checks PR | low | small | `internal/views/gastown.go` |

No critical or high-priority actions. All items are monitoring/tracking.

## Previous action items status

From 2026-03-04 check:
1. ~~Monitor hook_bead after gt v0.10.0 upgrade~~ → Confirmed removed, mg handles it. Carry forward as monitoring.
2. ~~Upgrade bd to 0.58.0~~ → Not yet done (bd binary at `/usr/local/bin/bd`).
3. ~~Upgrade gt to 0.10.0~~ → Not yet done.
4. ~~Surface `bd doctor --agent` in problems overlay~~ → **Done** (v0.6.0).
5. ~~Dogs in agent roster~~ → Still pending. No new data on dog visibility in `gt status --json`.
6. ~~`bd show --current` in header/footer~~ → Still pending.

## Raw commit log

### Beads (since 2026-03-04)
```
7f41edd 2026-03-04 feat(nix): modernize flake for nixpkgs-25.11, fix-merge PR #2314
e4f27bf 2026-03-04 Merge PR #2313: fix/defaultconfig-port-regression
431d840 2026-03-04 fix(docs): disambiguate duplicate nvim-beads entry
9bb3d1d 2026-03-04 Merge PR #2320: fix/gh-1321-opencode-recipe
dac0a37 2026-03-04 Merge PR #2325: fix/idle-monitor-shutdown
5d159dd 2026-03-04 Merge PR #2317: Add nvim-beads to COMMUNITY_TOOLS.md
b4b586d 2026-03-04 feat(doctor): allow suppressing specific warnings via config
637c817 2026-03-04 Merge PR #2318: docs: update GIT_INTEGRATION.md
7a53725 2026-03-04 Merge PR #2319: docs: add nvim-beads to Community Tools
8d3de51 2026-03-04 Merge PR #2322: fix(docs): correct QUICKSTART.md examples
d9a719e 2026-03-04 fix(doltserver): idle-monitor kills itself via Stop()
```

### Gas Town (since 2026-03-04, excluding backups)
```
fa9dc28 2026-03-04 Remove agent bead hook slot: use direct bead tracking
aa7dd7e 2026-03-04 Merge branch 'feat/cursor-hooks-support'
76ef3fa 2026-03-04 refactor: extract shared IsAutonomousRole into hookutil
cdb2f04 2026-03-04 fix(guard): use portable reverse-file for macOS compat
2a6a60f 2026-03-04 fix(convoy): add omitempty to strandedConvoyInfo.CreatedAt
3b9b0f0 2026-03-04 feat(dashboard): enrich convoy panel with progress %
fbfb3cf 2026-03-04 fix(dolt): server-side timeouts for CLOSE_WAIT
330aec8 2026-03-04 feat: add context-budget guard as external script
72798af 2026-03-04 fix(daemon): 5-minute grace before auto-closing empty convoys
39f7bf7 2026-03-04 fix: gt done uses wrong rig when shell cwd resets
b1ee19a 2026-03-04 fix(tmux): refresh cycle bindings when prefix stale
65c0cb1 2026-03-04 fix(patrol): cap stale cleanup, break early on active patrol
0071001 2026-03-04 Merge PR #2302: fix(ci): resolve lint errors, Windows test failures
35929e8 2026-03-04 fix: address review feedback on Docker setup
```
