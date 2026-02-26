# Upstream Check — 2026-02-25

## TL;DR

**Beads**: ~25 commits since last check (2026-02-23), focused on Dolt hardening and migration fidelity. One behavioral change: `conditional-blocks` dependency type is now evaluated in readiness computation. No new release; v0.56.1 remains latest.

**Gas Town**: 169 commits since v0.8.0 (tagged 2026-02-23). Massive churn around tmux socket isolation (multi-town), sling rollback, Dolt stability, and Claude Code workspace trust mitigation. No breaking changes to mg's integration points, but `AgentRuntime` now has 3 new fields (`FirstSubject`, `AgentAlias`, `AgentInfo`) that mg should surface. The `Running` field was NOT removed — it's present upstream despite our earlier removal. No new release.

## Current Baseline

| Item | Value |
|------|-------|
| mg version | v0.2.0 (main @ `4364f5c`) |
| Beads installed | v0.56.1 |
| Beads main | ~25 commits ahead of v0.56.1 |
| Gas Town in go.mod | v0.8.0 |
| Gas Town main | 169 commits ahead of v0.8.0 |
| Previous check | [2026-02-23](beads-upstream-check-2026-02-23.md) / [gastown](gastown-upstream-check-2026-02-23.md) |

---

## Beads

### Breaking Changes

#### `conditional-blocks` dependency type now evaluated (PR #2128)

Previously `conditional-blocks` deps were defined but not evaluated when computing blocked status. Now they count as blockers while the precondition issue is open. If mg computes readiness independently of `bd ready`, results may diverge.

**Affected files**: `internal/data/issue.go`, `internal/data/focus.go`
**Severity**: Low-medium. Worth verifying mg's `blockingTypes` set handles this.

#### Auto-migrate on CLI upgrade (`1d3c9a7`)

`bd` now auto-migrates the database schema on every CLI invocation after upgrade. Transparent to mg but could cause one-time latency on first `bd list --json` after upgrade.

### Feature Opportunities

| Opportunity | Source | Effort | Notes |
|------------|--------|--------|-------|
| `has_metadata_key` query predicate | PR #1996 | Small | New DSL predicate for `bd query`. Relevant if mg adds query bar. |
| `bd ready --json` enriched | d81badd (pre-check) | Medium | Labels, deps, parent in output. Could replace client-side readiness. |
| Dolt auto-push for replication | PR #2132 (open) | Track | New `dolt.auto-push` config. No mg action needed. |

### Open PRs to Watch

| PR | Title | Relevance |
|----|-------|-----------|
| #2132 | feat: `dolt.auto-push` for Dolt replication | Could affect server-mode under Gas Town |
| #2135 | fix: reset AUTO_INCREMENT after DOLT_PULL/MERGE | Fewer duplicate key errors |
| #2131 | fix: "no store available" with `bd dolt pull` | Fixes exact error we hit |
| #2001 | Reparented child appears under BOTH parents | If landed, changes parent/child semantics |

---

## Gas Town

### Breaking Changes

**None.** All changes are additive or internal. `gt status --json`, `gt sling`, `gt convoy`, `gt mail` interfaces are stable.

### Important: `Running` field is NOT removed

MEMORY.md says v0.8.0 removed `Running` (citing commit `67cdc25`). However, the current main branch has `Running bool json:"running"` present in `AgentRuntime`. Either it was re-added or the removal was reverted. `gt status --json` is emitting `running: true/false` and mg is silently discarding it. Adding it back improves zombie detection.

### New `AgentRuntime` Fields

```go
type AgentRuntime struct {
    // ... existing fields ...
    Running      bool   `json:"running"`           // PRESENT upstream (mg removed it)
    FirstSubject string `json:"first_subject"`     // NEW: first unread mail subject
    AgentAlias   string `json:"agent_alias"`       // NEW: e.g., "opus-46", "pi"
    AgentInfo    string `json:"agent_info"`        // NEW: e.g., "claude/opus", "pi/kimi-k2p5"
}
```

### Feature Opportunities

| Opportunity | Source | Effort | Notes |
|------------|--------|--------|-------|
| Surface `AgentInfo` in roster | `agent_info` field | Small | Shows runtime/model in Gas Town panel |
| Surface `AgentAlias` | `agent_alias` field | Small | Short name next to agent |
| Surface `FirstSubject` | `first_subject` field | Small | Mail preview in roster row |
| Re-add `Running` field | Present upstream | Small | Improves zombie detection accuracy |
| Parked rig error handling | `01fdc51` | Small | `gt sling`/`gt convoy` reject parked rigs — surface in toast |
| Acceptance criteria display | `5d690f1` | Medium | New `AcceptanceCriteria` gate in `gt done` |
| Multi-town tmux socket audit | 5 commits | Medium | mg tmux calls use bare `tmux` without socket flag |

### Open PRs to Watch

| PR | Title | Impact |
|----|-------|--------|
| #2046 | Prevent double-spawn from stale convoy feed | Fixes ghost agents in roster |
| #2044 | Prevent premature polecat nuke before refinery merge | Fewer false "completed" states |
| #2043 | Use handoff bead for mol attach auto-detect | May affect `gt mol dag` output |
| #2042 | Session reliability + socket migration | More tmux socket changes |
| #2035 | Pi-rust agent runtime support | New runtime type |

---

## Recommended Actions

| # | Action | Priority | Effort | Files |
|---|--------|----------|--------|-------|
| 1 | Add `Running`, `AgentAlias`, `AgentInfo`, `FirstSubject` to mg's `AgentRuntime` | high | small | `gastown/status.go`, `views/gastown.go` |
| 2 | Update zombie detection to use `Running` field | high | small | `gastown/problems.go` |
| 3 | Verify `conditional-blocks` dep type passes through mg blocking logic | high | small | `data/issue.go` |
| 4 | Display agent runtime info in Gas Town panel | medium | small | `views/gastown.go`, `ui/styles.go` |
| 5 | Show first mail subject in agent roster | medium | small | `views/gastown.go` |
| 6 | Surface parked rig errors in toast | medium | small | `gastown/sling.go` |
| 7 | Audit tmux socket usage for multi-town compat | low | medium | `agent/tmux.go`, `gastown/sling.go` |
| 8 | Acceptance criteria display in detail panel | low | medium | `data/issue.go`, `views/detail.go` |
| 9 | `bd ready --json` enriched output | low | medium | `data/source.go` |

---

## Raw Commit Logs (mg-relevant only)

### Beads (since 2026-02-23)

```
535ba5e fix(readiness): conditional-blocks deps in computeBlockedIDs()
1d3c9a7 fix: auto-migrate database version on CLI upgrade
f6b04f7 has_metadata_key in bd query DSL
94b699c per-worktree .beads/redirect override
4fd1f6e feat: auto-push to git remote after Dolt commits
b3809f4 test: add messaging system tests
```

### Gas Town (since 2026-02-23)

```
bcca5d8 fix: stop hardcoding 'origin' as dolt remote name
18c5b34 fix(tmux): auto-dismiss workspace trust dialog
8d5264f fix(multi-town): migrate remaining tmux callsites to socket-aware API
7cd3155 fix sling rollback to burn stale molecules
37441ee fix(tmux): stop daemon from creating separate socket server
70743bc feat: add best-effort push before --force nuke
209b47b fix: parse tmux socket from TMUX env var instead of hardcoding
01fdc51 feat: add parked rig checks to sling and convoy commands
6acfb08 fix: add deterministic git state to handoff context
5d690f1 feat: acceptance criteria gate in gt done
c3c9c7d check bead status before zombie flagging and redispatch
a6df5fa fix: add in_progress guard to sling dispatch
3e690b7 fix: emit session_death event on polecat crash
fe0ac2b feat: add dog health-check command
```
