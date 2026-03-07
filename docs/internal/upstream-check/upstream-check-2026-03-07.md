# Upstream Check — 2026-03-07

## TL;DR

New releases on both repos: Beads v0.59.0 and Gas Town v0.11.0. **Critical bug in bd v0.59.0**: `--tree` became the default display mode, and the `--json` flag is silently ignored — this breaks `bd list --json` which mg depends on. Fix exists on main (PR #2432) but is unreleased. Do NOT upgrade bd past v0.58.0 until v0.59.1+. Gas Town v0.11.0 ships stuck agent detection moved to Dog plugin, `--cascade` close, and beads dep bumped to v0.59.0.

## Current baseline

- mg version: v0.6.1
- Beads: v0.58.0 installed (`/usr/local/bin/bd`); v0.59.0 released (DO NOT UPGRADE)
- Gas Town: v0.10.0 installed; v0.11.0 released
- go.mod: no beads/gastown Go deps (mg calls CLI only)
- Previous check: [upstream-check-2026-03-05.md](upstream-check-2026-03-05.md)

## Breaking changes

### 1. bd v0.59.0: `bd list --json` silently broken (CRITICAL)

**What changed**: `364691fa fix(list): make --tree the default display mode for bd list` landed in v0.59.0. A subsequent bug causes `--json` flag to be ignored when `--tree` is the default.

**Fix on main**: `15a9d16 fix(list): --json flag ignored when --tree defaults to true` — not yet released. PR #2432 (`fix: use Root().PersistentFlags() to check --json in PersistentPreRun`) provides the proper fix.

**mg impact**: `internal/data/source.go` calls `bd list --json` via `FetchIssuesCLI()`. If bd is upgraded to v0.59.0, mg will receive tree-formatted text instead of JSON, causing parse failures and no issues loading.

**Action**: **Do NOT upgrade bd to v0.59.0.** Stay on v0.58.0 until v0.59.1+ ships with the fix. Consider adding a bd version check in mg's CLI source to warn if running a known-broken version.

### 2. bd v0.59.0: daemon infrastructure removed

**What changed**: `b3decc2 feat(bd): remove daemon infrastructure (w-bd-001)` — the daemon is fully removed.

**mg impact**: None directly. mg uses CLI mode (`bd list --json`) not daemon. But `bd sync` references in docs/help are being cleaned up (PR #2440). The `--allow-stale` flag was briefly broken then restored as a no-op for gt compatibility.

**Action**: No code changes needed. When upgrading, verify `bd list --json` still works without daemon.

## Feature opportunities

### 1. `bd done <id> <message>` syntax (Beads main, post-v0.59.0)

**Commit**: `2df714f feat: bd done <id> <message> treats last arg as reason`

**What**: `bd done` (alias for `bd close`) now accepts the closing reason as a positional arg instead of requiring `--reason`. Simpler syntax for quick closes.

**How mg could use it**: If mg's close action (`internal/data/mutate.go`) adds a reason, it could use the simpler syntax. Low priority since mg already uses `bd close`.

**Effort**: Small.

### 2. Stuck agent detection moved to Dog plugin (Gas Town v0.11.0)

**Commit**: `5a5deaa fix: move stuck agent detection from daemon to Dog plugin`

**What**: Stuck agent detection is no longer in the daemon — it's now a Dog plugin. This means `gt status --json` may report stuck agents differently, and Dogs now have more operational responsibilities.

**How mg could use it**: The problems overlay (`internal/gastown/problems.go`) does its own stuck detection. If Dog-detected stuck agents are surfaced in `gt status --json`, mg could consume them directly instead of reimplementing the heuristic.

**Effort**: Small-medium. Check `gt status --json` for Dog-reported problems after v0.11.0 upgrade.

### 3. `--cascade` close flag (Gas Town v0.11.0)

**Commit**: `38bc447 feat(close): add --cascade flag to close parent and all children (GH#998)`

**What**: `gt close --cascade` closes a parent issue and all its children atomically.

**How mg could use it**: Add a "cascade close" option to the detail panel when viewing a parent issue. Useful for closing epics with all sub-tasks.

**Effort**: Small. Add keybinding in detail view, call `gt close --cascade <id>`.

### 4. Enriched convoy dashboard (Gas Town v0.11.0 — previously tracked)

Already noted in 2026-03-05 check. Now shipped in v0.11.0 release. `ProgressPct`, ready/active counts, and assignees available.

**Effort**: Small. Check `gt convoy list --json` output after upgrade.

### 5. `beads_prefix` in `gt rig list --json` (Gas Town PR #2477 — open)

**What**: PR adds `beads_prefix` field to rig list JSON output.

**How mg could use it**: Cross-rig issue references in the Gas Town panel could be resolved more accurately.

**Effort**: Small.

### 6. Metadata merge fix (Beads PR #2423 — open)

**What**: `bd update --metadata` now merges with existing metadata instead of replacing it.

**How mg could use it**: Safer metadata updates from mg if we ever add metadata editing. Low priority.

### 7. Reap idle polecats (Gas Town PR #2478 — open)

**What**: Auto-reap idle polecat sessions to prevent API slot burn.

**How mg could use it**: Fewer zombie agents to detect in problems overlay. Informational only.

## Recommended actions

| # | Action | Priority | Effort | Files |
|---|--------|----------|--------|-------|
| 1 | **DO NOT upgrade bd to v0.59.0** — `--json` flag broken | critical | zero | — |
| 2 | Track bd v0.59.1 release for `--json` fix (PR #2432) | critical | zero | — |
| 3 | Upgrade gt to v0.11.0 | medium | small | — |
| 4 | Add bd version check warning in CLI source | medium | small | `internal/data/source.go` |
| 5 | Surface Dog-detected problems from `gt status --json` | low | small-medium | `internal/gastown/problems.go` |
| 6 | Add `--cascade` close option to detail panel | low | small | `internal/views/detail.go`, `internal/data/mutate.go` |
| 7 | Render enriched convoy data after gt upgrade | low | small | `internal/views/gastown.go` |
| 8 | Render dogs in agent roster | low | medium | `internal/views/gastown.go` |
| 9 | Surface `bd show --current` in header/footer | low | small | `internal/components/header.go` |

## Previous action items status

From 2026-03-05 check:
1. ~~Monitor hook_bead removal post-upgrade~~ -> Still monitoring. Not yet upgraded.
2. ~~Upgrade bd to 0.58.0~~ -> Still pending. **Now blocked**: v0.59.0 has critical bug, stay on v0.58.0.
3. ~~Upgrade gt to 0.10.0~~ -> v0.11.0 now available. Can upgrade gt independently.
4. ~~Track enriched convoy data~~ -> Now shipped in v0.11.0. Ready to implement after gt upgrade.
5. ~~Track GitHub Issues integration PR~~ -> PR #2373 not in recent commit list. May be superseded or still open.
6. ~~Track polecat JSON state fix PR~~ -> **Merged** as PR #2379 (`774eec9`). In v0.11.0.
7. ~~Track daemon pressure checks PR~~ -> Not visible in recent commits. Still open or superseded.

## Raw commit log

### Beads (v0.59.0 release + 39 commits on main)

#### In v0.59.0 release
```
364691f fix(list): make --tree the default display mode for bd list
b3decc2 feat(bd): remove daemon infrastructure (w-bd-001)
b4b586d feat(doctor): allow suppressing specific warnings via config
7702ebe feat(doctor): detect fresh clone state on Dolt server
1ca5e8c feat(dolt): surface sync.git-remote in database-not-found errors
4e408bc feat(init): distinguish server-reachable from DB-exists in init guard
5ea0eaa feat: add OpenCode recipe to bd setup
7c8cd89 feat: add version to END marker for better future matching
1b921b9 fix(dep): add cross-prefix routing to dep commands (#2296)
f06795e fix(dolt): gate CLI routing on local remote availability
2ba09ad fix(dolt): batch all IN-clause queries to prevent full table scans
bb7be24 fix: skip tombstone entries in bd init --from-jsonl
049ec26 fix: add id tiebreaker to all ORDER BY clauses
4197335 fix: deterministic ordering in SearchIssues and GetReadyWork
```

#### Post-v0.59.0 (on main, unreleased)
```
0ed3202 Merge PR #2427: db/interfaces (storage refactoring)
d5b754b /internal/storage/storage.go: fix store name
9437d4f Update embeddeddolt store
6500eae /internal/storage: add interfaces for methods not covered
b7d44a2 Merge PR #2409: fix install detect WSL/MINGW
5a454ae fix: suppress pre-existing gosec false positives
a9a9c7a fix: clean formatting drift
9d2e364 Merge PR #2424: db/ed-4 (embedded dolt)
37dfcd5 Merge PR #2420: db/ed-3 (embedded dolt)
b515661 fix: skip sleep in CI/non-interactive environments
15a9d16 fix(list): --json flag ignored when --tree defaults to true
2df714f feat: bd done <id> <message> treats last arg as reason
e6019d9 fix: hide --allow-stale no-op flag from help output
bdbbb4b fix: restore --allow-stale as no-op flag for gt compatibility
fbbaa5e fix: detect WSL and MINGW in install.sh
7ba3a05 Follow redirect target when cleaning stale lock files
dbbe212 fix(worktree): skip gitignore append when parent dir pattern covers
1ea190e Follow redirect target when cleaning stale lock files
dc0b180 fix(deps): update dolthub/driver digest
```

### Gas Town (v0.11.0 release + 7 commits on main)

#### In v0.11.0 release
```
38bc447 feat(close): add --cascade flag to close parent and all children
3b9b0f0 feat(dashboard): enrich convoy panel with progress %
dafcd24 feat(polecat): set POLECAT_SLOT env var for test isolation
86e3b89 feat: Add docker-compose and Dockerfile
86e3b89 feat: add Cursor hooks support for polecat agent integration
330aec8 feat: add context-budget guard as external script
3f533d9 feat: add schema evolution support to gt wl sync
5a5deaa fix: move stuck agent detection from daemon to Dog plugin
774eec9 fix(polecat): reconcile JSON list state with session liveness
f3d47a9 Fix serial killer bug: remove hung session detection for witnesses/refineries
7084e37 fix: refinery PostMerge uses ForceCloseWithReason for source issue
6bc898c fix(nudge): change default delivery mode from immediate to wait-idle
0516f68 fix(sling): add TTL to sling contexts to prevent permanent scheduling blocks
```

#### Post-v0.11.0 (on main, unreleased)
```
a4117e9 fix: agent-beads-exist check now verifies polecat beads
3627f03 fix: sweep orphaned wisp_dependencies after compact
dd4f810 fix: sweep orphaned wisp_dependencies after compact
f587f7a fix: agent-beads-exist check now verifies polecat beads
bf260dc docs: correct homebrew turnaround time
3fc2014 fix: update stale TODO comment on polecat spawn cap
e9c2929 fix: remove dead homebrew tap job in release workflow
```

### Notable open PRs

**Beads:**
- **#2440**: Replace stale 'bd sync' references in docs
- **#2439**: Stabilize BEADS_DIR paths from detached commit worktrees
- **#2437**: Fix config package shadow in compact command
- **#2436**: Support `ai.api_key` config for Anthropic API key
- **#2432**: Fix `--json` flag in PersistentPreRun (**CRITICAL for mg**)
- **#2431**: Remove daemon infrastructure remnants
- **#2428**: Use interface for init store (embedded dolt)
- **#2423**: Fix `--metadata` merge (merge instead of replace)

**Gas Town:**
- **#2481**: Honor sling's `BD_DOLT_AUTO_COMMIT=off`
- **#2480**: `gt prime --hook` blocks on non-Claude runtimes
- **#2479**: EnsureMetadata repairs stale dolt_server_port
- **#2478**: Reap idle polecat sessions to prevent API slot burn
- **#2477**: Include `beads_prefix` in `gt rig list --json`
- **#2476**: `gt dolt sync` pushes via SQL to avoid stopping server
- **#2474**: Honor `--base-branch` in formula rendering
- **#2473**: Namespace tmux sockets by town path hash
- **#2472**: Propagate `BEADS_DOLT_PORT` to agent sessions
- **#2471**: `gt mq list` returns empty for merge-request wisps
