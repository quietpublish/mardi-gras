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
- **Rich fields** — notes, design, and acceptance criteria fetched on demand via `bd show --long`
- **Dependencies** — nine types (blocks, conditional-blocks, blocked-by, related, duplicates, supersedes, parent-child, discovered-from, depends-on) grouped by status: waiting, missing, resolved, and non-blocking
- **Comments & Timeline** — full conversation history with timestamps
- **Agent Output** — live tail of the active agent's tmux pane (last 15 lines, ANSI stripped)
- **Molecule DAG** — multi-step workflows rendered as a visual flow graph with parallel branching (`┌─ ├─ └─`) and connector lines between tiers
- **HOP Quality** — reputation stars, crystal/ephemeral badges, and validator verdicts for agent-produced work

Press `m` in the detail pane to mark the active molecule step as done.

## Filtering

Press `/` and the bottom bar becomes a query input.

- `enter`: keep the query applied and return to list navigation.
- `esc`: clear the query and exit filter mode.
- Multiple terms use `AND` semantics (all terms must match).

Supported query forms:

- Free text: `deploy auth` (matches ID, title, description, assignee, owner, notes, and labels)
- Type token: `type:bug`, `type:feature`, `type:task`, `type:chore`, `type:epic`
- Priority shorthand: `p0` to `p4`
- Priority token: `priority:0` to `priority:4`, or `priority:critical|high|medium|low|backlog`

Examples:

```text
type:feature p1 deploy
priority:high auth
type:feature p0 auth deploy     ← matches P0 features containing "auth" AND "deploy"
vv-006
```

## Command Palette

Press `:` or `Ctrl+K` to open a fuzzy-match command palette. Type to filter available actions, then press `enter` to execute. The palette provides access to the same actions available through keybindings, plus palette-only actions like **Cascade close** (close an issue and all its children, requires Gas Town v0.11+).
