---
name: upstream-scout
description: Research recent changes in beads and gastown upstream repos. Identifies new features to leverage, breaking changes to handle, and creates a research document with actionable items.
disable-model-invocation: true
argument-hint: "[beads|gastown|both (default: both)]"
---

# Upstream Scout

Research recent upstream changes in Beads and/or Gas Town. Produce a research document with feature opportunities and compatibility actions.

## Scope

- `$ARGUMENTS` controls which repos to check: `beads`, `gastown`, or `both` (default)
- Beads repo: `steveyegge/beads`
- Gas Town repo: `steveyegge/gastown`

## Step 1: Gather upstream state

For each repo in scope:

### Recent commits on main

```bash
gh api repos/steveyegge/<repo>/commits?per_page=30 --jq '.[] | "\(.sha[0:7]) \(.commit.message | split("\n")[0])"'
```

### Recent releases

```bash
gh release list --repo steveyegge/<repo> --limit 5
```

### Open PRs with significant changes

```bash
gh pr list --repo steveyegge/<repo> --state open --limit 10
```

## Step 2: Check our current compatibility baseline

Read the memory file for current upstream tracking state:

- Check `~/.claude/projects/-Users-matthewwright-Work-mardi-gras/memory/MEMORY.md` for the "Upstream Status" section
- Check `docs/internal/` for previous upstream research docs
- Check `go.mod` for current dependency versions

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

Write a research document to `docs/internal/<repo>-upstream-check-<date>.md` with:

```markdown
# <Repo> Upstream Check — <date>

## TL;DR
<2-3 sentence summary of key findings>

## Current baseline
- mg version: <latest tag>
- <repo> version in go.mod or last checked: <version>
- Previous check: <link to last doc if exists>

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

## Step 5: Update memory

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
