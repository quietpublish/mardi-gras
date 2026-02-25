# Upstream Check — 2026-02-25

## TL;DR

Both repos are stabilizing post-migration. Beads got **30 commits** in 2 days — mostly Dolt doctor fixes, test infrastructure, and a new `--database` init flag (fixes our GH#2098 question). Gas Town got **30 commits** — multi-town tmux socket fixes, parked rig guards on sling/convoy, and sling rollback for stale molecules. No breaking changes to mg. Two feature opportunities: **config-driven metadata schema** and **MCP claim tool** in beads; **parked rig awareness** in Gas Town.

## Current baseline

- mg version: v0.2.0
- beads installed: v0.56.1 (latest release)
- gastown installed: v0.7.0 (latest release)
- beads main: 30 commits ahead of v0.56.1
- gastown main: 30 commits ahead of v0.7.0
- Previous check: [2026-02-23](beads-upstream-check-2026-02-23.md) / [gastown](gastown-upstream-check-2026-02-23.md)

---

## Beads (30 commits since 2026-02-23)

### Breaking changes

**None.** All changes are additive or internal fixes. `bd list --json` output format is unchanged.

### Feature opportunities

#### 1. `bd init --database` flag (a30e02a)

New `--database` flag lets callers specify a pre-existing server database name, avoiding orphan DB creation when Gas Town has already set up the database. This directly answers GH#2098 (multiple beads projects on one Dolt server).

**How mg could use it**: Not directly relevant to mg's runtime, but useful for our beads setup docs and the `/gt-test` skill. Update setup instructions to recommend `--database` when initializing inside a Gas Town rig.

**Effort**: Small (docs only)

#### 2. MCP `claim` tool (3a86e9f)

New dedicated MCP tool for atomic issue claiming (equivalent to `bd update <id> --claim`). Separates start-work semantics from generic update. Docs standardized around `--claim` for start-work (#2070).

**How mg could use it**: We already use `bd update <id> --claim` via CLI. No immediate code change, but validates our approach. If we ever add MCP integration, use `claim` instead of `update`.

**Effort**: None (validates existing approach)

#### 3. Config-driven metadata schema enforcement (00aa5dc)

New `validation.metadata` section in `.beads/config.yaml` lets projects define field types, required fields, and constraints. Modes: `none` (default), `warn`, `error`.

**How mg could use it**: If a project enforces metadata schema, mg's issue create form could read the schema and present validated fields. Could also show metadata in the detail panel.

**Effort**: Medium (read config, update create form, update detail view)

#### 4. Auto-update stale hooks (7dafce0)

`bd init` now auto-updates outdated hooks. Previously required manual intervention.

**How mg could use it**: Not directly relevant to mg, but good to know — reduces setup friction for new users.

**Effort**: None

### Notable fixes

| Commit | Fix | Relevance |
|--------|-----|-----------|
| 76e01b2 | Auto-clean stale Dolt noms LOCK files on startup | Fixes a common failure mode we've seen |
| 56f4c98 | Doctor phantom detection + INFORMATION_SCHEMA crash fix | Addresses GH#2091 we found in triage |
| 362d82b | Doctor suppresses expected .beads/ deletions in redirect worktrees | Fixes noise in Gas Town crew workspaces |
| 6b47996 | Reopen clears defer_until | Correctness fix |
| 376122b | Test suite no longer leaks databases to production Dolt | Improves upstream CI reliability |

### Open PRs worth watching

| PR | Title | Impact |
|----|-------|--------|
| #2108 | Batch SQL IN-clause queries (15s → 0.16s) | Huge perf win for `bd list --json` on large projects. Our SourceCLI mode directly benefits. |
| #2106 | Check local store before prefix routing | Fixes silent data divergence in multi-rig setups. Important for our Gas Town testing. |
| #2114 | Use doltserver.DefaultConfig for port resolution | Fixes GH#2073 (global config port not consulted). |
| #2117 | Auto-commit beads files during `bd init` | Addresses GH#1989. |
| #2103 | Auto-publish Homebrew tap on release | Better install experience. |

---

## Gas Town (30 commits since 2026-02-23)

### Breaking changes

**None.** `gt status --json` format unchanged. CLI flags we use (`gt sling`, `gt nudge`, `gt handoff`, `gt convoy`, `gt mail`) unchanged.

### Feature opportunities

#### 1. Parked rig awareness (01fdc51)

`gt sling` and `gt convoy` now check `IsRigParked()` before dispatching. Parked rigs are blocked from receiving work (unless `--force`). Error messages include unpark command.

**How mg could use it**: Our sling dispatch (`a` key) calls `gt sling` and shows a toast. If the target rig is parked, we'd get an error. We should:
- Parse the "rig is parked" error from gt sling stderr
- Show a specific toast: "Rig is parked — run `gt rig unpark <name>` to resume"
- Potentially show a "parked" indicator in the agent roster

**Effort**: Small (error parsing + toast) to Medium (roster indicator)

#### 2. Sling rollback burns stale molecules (7cd3155)

When sling fails, stale molecules are now cleaned up. Previously they could linger and confuse the molecule DAG.

**How mg could use it**: Our molecule DAG rendering should see fewer phantom nodes. No code change needed, but validates our nil-safety in `dagrender.go`.

**Effort**: None

#### 3. Deterministic git state in handoff (6acfb08)

Handoff now captures deterministic git state (branch, commit, dirty status) for the receiving agent.

**How mg could use it**: If we ever show handoff context in the detail panel, we could display the git state. Currently we just show handoff as a lifecycle event.

**Effort**: Small (if we want to display it)

### Notable fixes

| Commit | Fix | Relevance |
|--------|-----|-----------|
| 18c5b34 | Auto-dismiss Claude Code workspace trust dialog | Critical for agent reliability — trust dialog was blocking all automated sessions |
| 8d5264f | Migrate remaining tmux callsites to socket-aware API | Multi-town support. Our tmux dispatch may need to use socket-aware calls. |
| 209b47b | Parse tmux socket from TMUX env var | Fixes cross-socket cycling. Relevant to our `internal/tmux/` code. |
| 37441ee | Stop daemon from creating separate socket server | Fixes tmux socket conflicts |
| 8a54de6 | Include pinned agents in mail recipient validation | Fixes mail compose failures |
| ece3eda | Make `gt dolt start` idempotent | Safer Dolt server management |

### Open PRs worth watching

| PR | Title | Impact |
|----|-------|--------|
| #2021 | Update nix package for v0.8.0 | Signals v0.8.0 release approaching |
| #2020 | Update patrol formulas to use `gt patrol new` | New patrol command we don't use yet |
| #2019 | Make env-vars check auto-fixable via `gt doctor --fix` | Better onboarding |
| #2018 | Add `--upstream-url` to `gt rig add` | Remote rig plumbing |

---

## Recommended actions

| # | Action | Priority | Effort | Source | Files |
|---|--------|----------|--------|--------|-------|
| 1 | Parse "rig is parked" error from `gt sling` stderr, show specific toast | medium | small | GT 01fdc51 | `internal/gastown/sling.go`, `internal/views/gastown.go` |
| 2 | Review tmux socket handling against upstream multi-town fixes | medium | small | GT 209b47b, 8d5264f | `internal/tmux/`, `internal/agent/` |
| 3 | Add metadata schema display to detail panel (when config present) | low | medium | BD 00aa5dc | `internal/data/`, `internal/views/detail.go` |
| 4 | Update `/gt-test` skill with `bd init --database` recommendation | low | small | BD a30e02a | `.claude/skills/gt-test/SKILL.md` |
| 5 | Track PR #2108 (batch queries) — will speed up SourceCLI mode significantly | info | none | BD PR #2108 | — |
| 6 | Track v0.8.0 release signals (nix package PR, env-vars doctor) | info | none | GT PRs | — |

## Raw commit log (filtered, significant only)

### Beads
```
a30e02a feat(init): add --database flag to configure existing server database (#2102)
00aa5dc feat: config-driven metadata schema enforcement (GH#1416 Phase 2) (#2027)
3a86e9f feat(mcp): add dedicated claim tool for atomic start-work (#2071)
7dafce0 feat: auto-update stale hooks during bd init (GH#1466) (#2008)
76e01b2 fix: auto-clean stale Dolt noms LOCK files on bd startup (#2059)
56f4c98 fix: doctor phantom detection and INFORMATION_SCHEMA crash (#2091) (#2093)
362d82b fix(doctor): suppress expected .beads/ deletions in redirect worktrees (#2080)
6b47996 fix(reopen): clear defer_until when reopening an issue (#2000)
ef57293 fix(dolt): stop wisp ID double-prefixing and add wisp fallback to ResolvePartialID
```

### Gas Town
```
01fdc51 feat: add parked rig checks to sling and convoy commands (gt-4owfd.1)
70743bc feat: add best-effort push before --force nuke (gt-4vr)
18c5b34 fix(tmux): auto-dismiss workspace trust dialog blocking all agent sessions
8d5264f fix(multi-town): migrate remaining tmux callsites to socket-aware API
7cd3155 Fix sling rollback to burn stale molecules (#2009)
209b47b fix: parse tmux socket from TMUX env var instead of hardcoding town name
6acfb08 fix: add deterministic git state to handoff context (GH#1996)
8a54de6 fix(mail): include pinned agents in recipient validation (#2002)
ece3eda fix(dolt): make gt dolt start idempotent and fix server detection
```
