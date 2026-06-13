---
name: upstream-scout
description: Research recent changes in beads and gastown upstream repos. Identifies new features to leverage, breaking changes to handle, and creates a research document with actionable items.
argument-hint: "[beads|gastown|both (default: both)]"
---

# Upstream Scout

Research recent upstream changes in Beads and/or Gas Town. Produce a research document with feature opportunities and compatibility actions.

## Scope

- `$ARGUMENTS` controls which repos to check: `beads`, `gastown`, or `both` (default)
- Beads repo: `gastownhall/beads`
- Gas Town repo: `gastownhall/gastown`

> Note: both repos moved org from `steveyegge/*` → `gastownhall/*` (observed 2026-06-13). GitHub still 301-redirects the old `steveyegge/*` paths, so older commands keep working, but `gastownhall/*` is canonical.

## Step 1: Gather upstream state

For each repo in scope:

### Recent commits on main

```bash
gh api 'repos/gastownhall/<repo>/commits?per_page=30' --jq '.[] | "\(.sha[0:7]) \(.commit.committer.date[:10]) \(.commit.message | split("\n")[0])"'
```

### Recent releases

```bash
gh release list --repo gastownhall/<repo> --limit 5
```

### Open PRs with significant changes

```bash
gh pr list --repo gastownhall/<repo> --state open --limit 10
```

## Step 2: Check our current compatibility baseline

Read these sources for the previous check state:

- **Journal**: `docs/internal/upstream-check/UPSTREAM_JOURNAL.md` — chronological log of all checks
- **Memory**: `~/.claude/projects/-Users-matthewwright-Work-mardi-gras/memory/MEMORY.md` — "Upstream Status" section
- **Previous docs**: `docs/internal/upstream-check/` — full research docs from prior checks
- **go.mod** — current dependency versions

## Step 3: Analyze each upstream change

For each significant commit or release, classify it:

### Breaking changes (MUST handle)
Changes that will cause mg to fail or behave incorrectly:
- Removed fields/APIs we depend on
- Changed JSON output formats we parse
- Renamed CLI flags we call
- New required dependencies

### Feature opportunities (COULD leverage)
New capabilities we could expose in the TUI:
- New data fields in `bd list --json` output
- New `gt` subcommands or flags
- New agent states, roles, or lifecycle events
- New UI-relevant metadata (labels, comments, etc.)

### Informational (GOOD to know)
Changes that don't directly affect mg but provide context:
- Internal refactors
- Bug fixes in areas we don't touch
- Documentation updates

## Step 4: Create research document

Write a research document to `docs/internal/upstream-check/upstream-check-<date>.md` with:

```markdown
# Upstream Check — <date>

## TL;DR
<2-3 sentence summary of key findings>

## Current baseline
- mg version: <latest tag>
- <repo> version in go.mod or last checked: <version>
- Previous check: <link to last doc>

## Breaking changes
<For each: what changed, which mg files affected, suggested fix>

## Feature opportunities
<For each: what's new, how mg could use it, effort estimate (small/medium/large)>

## Recommended actions
| # | Action | Priority | Effort | Files |
|---|--------|----------|--------|-------|
| 1 | ... | critical/high/medium/low | small/medium/large | ... |

## Raw commit log
<Filtered list of relevant commits>
```

## Step 5: Update journal and memory

### Journal

Prepend an entry to `docs/internal/upstream-check/UPSTREAM_JOURNAL.md` (newest first, below header):

```markdown
## <date>

**Scope**: <beads|gastown|both> | **Doc**: [upstream-check-<date>.md](upstream-check-<date>.md)

<2-3 sentence summary>

**Breaking**: <summary or "None for mg">. **Action items**: <numbered list>.
```

### Memory

Update the "Upstream Status" section in MEMORY.md with:
- Date of this check
- Current versions
- Key findings summary
- Link to the full research doc

## Reference: mg's upstream touchpoints

### Beads (`internal/data/`)
- `source.go`: `bd list --json` output parsing
- `loader.go`: JSONL format parsing
- `mutate.go`: `bd update`, `bd close` CLI calls
- `focus.go`: `bd update --claim`

### Gas Town (`internal/gastown/`)
- `status.go`: `gt status --json` output parsing (AgentRuntime struct)
- `sling.go`: `gt sling`, `gt nudge`, `gt handoff` CLI calls
- `convoy.go`: `gt convoy` CLI calls
- `mail.go`: `gt mail` CLI calls
- `molecule.go`: `gt mol dag` output parsing
- `costs.go`: `gt costs --json` output parsing
- `comments.go`: `bd comments` output parsing
- `detect.go`: GT_* environment variable names
