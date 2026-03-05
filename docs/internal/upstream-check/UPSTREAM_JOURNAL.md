# Upstream Journal

Chronological log of upstream checks for Beads and Gas Town. Each entry links to the full research doc.

---

## 2026-03-05

**Scope**: both | **Doc**: [upstream-check-2026-03-05.md](upstream-check-2026-03-05.md)

Quiet day — no new releases. Beads main has ~11 post-v0.58.0 commits (docs, Nix flake, doctor warning suppression, idle-monitor fix). Gas Town main has ~14 non-backup commits (hook_bead removal confirmed, cursor hooks merged, enriched convoy dashboard, context-budget guard). Several interesting open PRs: GitHub Issues integration (#2373 on Beads), polecat JSON state reconciliation (#2379 on GT), daemon pressure checks (#2370 on GT).

**Breaking**: None for mg. **Action items**: All monitoring/tracking — no code changes needed. Carry forward: upgrade bd to 0.58.0, upgrade gt to 0.10.0, dogs in roster, `bd show --current` in header.

---

## 2026-03-04

**Scope**: both | **Doc**: [upstream-check-2026-03-04.md](upstream-check-2026-03-04.md)

New releases: Beads v0.58.0 (76 features, 200+ fixes — consolidation release) and Gas Town v0.10.0 (150 commits — reliability, telemetry, mTLS proxy, cursor hooks, circuit breakers). One potential risk: Gas Town removed `hook_bead` agent bead slot (`fa9dc28`), but mg enriches from hook data so no immediate break. Post-release: cursor hooks support merged, doctor warning suppression, idle-monitor fix.

**Breaking**: `hook_bead` slot removal — low risk, mg uses hook data fallback. **Action items**: 1) Monitor hook_bead after gt v0.10.0 upgrade. 2) Upgrade bd to 0.58.0. 3) Upgrade gt to 0.10.0. 4) Surface `bd doctor --agent` in problems overlay. Carry forward: dogs in roster, `bd show --current` in header.

---

## 2026-03-02

**Scope**: both | **Doc**: [upstream-check-2026-03-02.md](upstream-check-2026-03-02.md)

Heavy post-release bug-fix day on both repos. Two new features: `bd remember/memories/forget/recall` for persistent bead-backed agent memory (replaces MEMORY.md pattern), and `bd show --current` to resolve the active issue. Gas Town passes through memory commands. Charm TUI v2 PR open on Gas Town (BubbleTea/bubbles/lipgloss all v2) — mg will need to track this migration.

**Breaking**: None for mg. **Action items**: 1-3 carry forward from 3/1. 4) `gt vitals` done in v0.5.0. 5) Surface `bd show --current` in header/footer. 6) Plan Charm v2 migration epic.

---

## 2026-03-01

**Scope**: both | **Doc**: [upstream-check-2026-03-01.md](upstream-check-2026-03-01.md)

Major release day: Beads v0.57.0 and Gas Town v0.9.0 both released. No breaking changes for mg. Beads lands self-managing Dolt server, SSH push/pull fallback, per-worktree redirect, metadata mutations, and hook migration phase 1. Gas Town lands Compactor Dog, `gt vitals`, `gt upgrade`, merge-blocks convoy deps, per-worker agent selection, typed ZombieClassification (ZFC), and polecat IDLE-on-completion lifecycle.

**Breaking**: None for mg. **Action items**: 1) Upgrade bd to 0.57.0. 2) Upgrade gt to 0.9.0. 3) Update CLAUDE.md re: self-managing Dolt. 4) Surface `gt vitals` in Gas Town panel. 5) Render dogs in agent roster.

---

## 2026-02-28

**Scope**: both | **Doc**: [upstream-check-2026-02-28.md](upstream-check-2026-02-28.md)

Beads: hardening day — centralized Dolt error handling, config-driven infra types and compaction thresholds, `bd export` command, `bd backup init/sync` for Dolt-native backups, label inheritance on child creation. Gas Town: new Compactor Dog (dolt gc split from Doctor), auto six-stage lifecycle for new installs, role registry with TOML-driven properties, formula parser extended (extends/compose/advice/presets/squash/gate). Heavy test parallelization on both repos. ZFC initiative PRs on Gas Town (typed enums replacing hardcoded strings).

**Breaking**: None for mg. **Action items**: None new — all feature opportunities still blocked on upstream releases.

---

## 2026-02-27

**Scope**: both | **Doc**: [upstream-check-2026-02-27.md](upstream-check-2026-02-27.md)

Beads: full backup/replication story (`bd backup`, `bd dolt remote`, auto-push with 5m debounce). Gas Town: massive operational maturity push — root-only wisps cut wisp creation ~15x, 4 new Dog formulas (doctor, reaper, janitor, JSONL backup), `gt vitals` unified health dashboard, sleepwalking polecat prevention (zero-commit hard gate).

**Breaking**: None for mg. **Action items**: Surface `gt vitals` in Gas Town panel (#1), backup freshness indicator (#2), plan Dog patrol panel (#3).

---

## 2026-02-26

**Scope**: both | **Doc**: [upstream-check-2026-02-26.md](upstream-check-2026-02-26.md)

Massive beads merge day (~30 commits). Infra beads auto-route to wisps table, `BEADS_DOLT_PORT` renamed to `BEADS_DOLT_SERVER_PORT`, hyphen sanitization in db names, hash-derived port in doctor, embedded mode docs removed. Gas Town: convoy IDs shortened (5-char base36), `RoleDog` first-class constant with `DogName`, handoff preserves context on PreCompact.

**Breaking**: None for mg. **Action items**: Surface new AgentRuntime fields (#1), re-add Running for zombie detection (#2), plan SourceCLI-only future (#3).

---

## 2026-02-25

**Scope**: both | **Doc**: [upstream-check-2026-02-25.md](upstream-check-2026-02-25.md)

Beads: ~25 commits, Dolt hardening, `conditional-blocks` dep type now evaluated in readiness. Gas Town: 169 commits since v0.8.0 — tmux socket isolation, sling rollback, Dolt stability. 3 new `AgentRuntime` fields discovered: `FirstSubject`, `AgentAlias`, `AgentInfo`. `Running` field confirmed NOT removed upstream.

**Breaking**: `conditional-blocks` dep type (low-medium). **Action items**: Add new AgentRuntime fields, re-add Running field, verify conditional-blocks handling.

---

## 2026-02-23

**Scope**: both (separate docs) | **Docs**: [beads](beads-upstream-check-2026-02-23.md), [gastown](gastown-upstream-check-2026-02-23.md)

Major version jump: beads v0.52 → v0.56.1 (JSONL fully removed, Dolt-only), Gas Town v0.7.0 → v0.8.0 (persistent polecats, convoy ownership, new AI runtimes). Built SourceCLI fallback for mg to handle JSONL removal. Bumped gastown in go.mod to v0.8.0.

**Breaking**: JSONL removed from beads (critical). **Action items**: SourceCLI fallback (done), go.mod bump (done), gt sling syntax update (done).

---

## Triage Log

Issues we've commented on upstream:

| Date | Issue | Our comment |
|------|-------|-------------|
| 2026-02-24 | [#2016](https://github.com/steveyegge/beads/issues/2016) | Self-managing Dolt server info, single-server question answered |

Issues triaged but not commented on (2026-02-26):

| Issue | Title | Category |
|-------|-------|----------|
| #2029 | bd crashes with >1 dolt instance | Can help (drafted response) |
| #2030 | How to init new dolt database | Can help (drafted response) |
| #2098 | Multiple beads projects with Dolt | Can help (drafted response) |
| #2061 | Post-merge hook duplicate key | Can help (drafted response) |
| #2007 | bd prime references removed --status | Can help (drafted response) |
