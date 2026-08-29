# Navigating the Parade

## The Parade

Every Beads issue maps to a spot on the parade route:

| On the Route         | What It Means                         |
| -------------------- | ------------------------------------- |
| **Rolling** ●        | In progress — the float is moving     |
| **Lined Up** ♪       | Open and unblocked — waiting its turn |
| **Stalled** ⊘        | Blocked by a dependency               |
| **Past the Stand** ✓ | Done — beads have been thrown         |

Closed issues are collapsed by default (because in any real project, 90%+ of your issues are closed). Press `c` to expand them.

Stalled issues show a "next blocker" hint so you can see at a glance what's holding things up. Issues with dead agent sessions show a ☠ zombie indicator. Issues on dead rigs show a 💀 orphan indicator. The detail panel breaks dependencies into four categories: waiting on (active blockers), missing (dangling references), resolved (closed blockers), and related (non-blocking dependency types).

## Detail Panel

Press `enter` on any issue to focus the detail pane. It shows everything about the selected issue:

- **Metadata** — type, priority, assignee, due dates with overdue/due-soon badges
- **Rich fields** — notes, design, and acceptance criteria fetched on demand via `bd show --long --json`. mg adds `--brief-deps` (bd v1.3.0+) to trim a large dependency payload it never reads; a bd that rejects the flag is detected once per process and the call falls back to the plain form
- **Dependencies** — grouped by status: waiting, missing, resolved, and non-blocking. mg does not hard-code a list of dependency types; whatever `bd` emits is rendered. The types named by `--block-types` (default `blocks` and `conditional-blocks`) count as blockers, and the rest render as non-blocking, with friendly wording for `related`, `duplicates`, `supersedes`, `parent-child`, `discovered-from`, `waits-for`, and `replies-to`
- **Comments & Timeline** — full conversation history with timestamps
- **Agent Output** — live tail of the active agent's tmux pane (last 15 lines, ANSI stripped)
- **Molecule DAG** — multi-step workflows rendered as a visual flow graph with parallel branching (`┌─ ├─ └─`) and connector lines between tiers

Press `m` in the detail pane to mark the active molecule step as done.

## Filtering

Press `/` and the bottom bar becomes a query input.

- `enter`: keep the query applied and return to list navigation.
- `esc`: clear the query and exit filter mode.
- Structured tokens use `AND` semantics — every one of them must match.
- The right of the bar shows a live `N/M match` count as the query narrows.

Supported query forms:

- Free text: `deploy auth` — a fuzzy, case-insensitive **subsequence** match over ID + title + description + assignee + owner + notes + labels, joined into one haystack per issue. Results are ranked by match quality.
- Type token: `type:bug`, `type:feature`, `type:task`, `type:chore`, `type:epic`, `type:spike`, `type:story`, `type:milestone`
- Label token: `label:gt:agent` (case-insensitive, exact match on one of the issue's labels)
- Priority shorthand: `p0` to `p4`
- Priority token: `priority:0` to `priority:4`, or `priority:critical|high|medium|low|backlog`

Anything that isn't one of those prefixes (or a `p0`–`p4` shorthand) is free text. Free-text words are **not** ANDed independently — they are joined back into a single fuzzy pattern, so word order matters: `deploy auth` and `auth deploy` are different queries.

Examples:

```text
type:feature p1 deploy
priority:high auth
label:gt:agent p0
type:feature p0 auth deploy     ← P0 features fuzzy-matching "auth deploy", in that order
vv-006
```

## Excluding Issue Types

Use `--exclude-type` to hide specific issue types from the parade and status output. Excluded issues are still available in the detail panel's dependency graph — they just don't appear in the parade list or header counts.

```bash
mg --exclude-type=epic          # hide epics
mg --exclude-type=epic,chore    # hide epics and chores
```

## Excluding Labels

Use `--exclude-label` to hide issues carrying specific labels. Match is case-insensitive. Issues without any labels are always kept. Useful for suppressing bot-tracked beads (`gt:agent`) or any workflow tag you don't want cluttering the parade.

```bash
mg --exclude-label=gt:agent            # hide agent-tracked beads
mg --exclude-label=gt:agent,wip        # hide agent + wip-labeled issues
```

## Command Palette

Press `:` or `Ctrl+K` to open a fuzzy-match command palette. Type to filter available actions, then press `enter` to execute. The palette includes:

- **Add note** — append a note to the selected issue via `bd note`
- **Claim next ready** — atomically claim the top-priority ready bead via `bd ready --claim --json` (requires bd v1.0.4+)
- **Prune preview / Prune closed > 30d** — dry-run or force-delete closed non-ephemeral beads older than 30 days via `bd prune` (requires bd v1.1+)
- **Create & assign to crew** — open the issue create form with the crew field (requires an orchestrator; works on both Gas Town and Gas City)
- **Cascade close** — close an issue and all its children (Gas Town only — Gas City reports the operation as unsupported)
- **Cycle layout** — rotate through the Default / Gas Town / Wide panel arrangements
- **Recover dead rigs** — release orphans from a dead rig (Gas Town only, and only listed while a dead rig is actually detected)
- **Resume last Codex session** — `codex resume --last` in a new tmux pane (only when Codex is the active runtime inside tmux)
- All keybinding actions (close, set priority, sling, nudge, etc.)
