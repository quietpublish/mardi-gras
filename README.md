# ⚜ Mardi Gras

[![CI](https://github.com/matt-wright86/mardi-gras/actions/workflows/ci.yml/badge.svg)](https://github.com/matt-wright86/mardi-gras/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/matt-wright86/mardi-gras)](https://github.com/matt-wright86/mardi-gras/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/matt-wright86/mardi-gras)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Your Beads issues deserve a parade — not a spreadsheet.**

Mardi Gras is a terminal UI for [Beads](https://github.com/steveyegge/beads) that turns your issue list into a living parade: what's moving, what's waiting, what's blocked, and what's already behind you.

It's fast, visual, and joyful.
One binary. No config. Just `mg`.

![Mardi Gras TUI](docs/screenshots/mardi-gras.png)

Think of your project as a parade route:

```
Rolling      →  work in progress
Lined Up     →  open & ready
Stalled      →  blocked
Past Stand   →  done
```

Same data. Better vibe.

## Why this exists

Beads solves agent context beautifully.
But `bd list` wasn't built for humans doing daily visual triage.

People have tried to fix this: web dashboards, desktop apps, alternate TUIs. Most recreate a kanban board.

Mardi Gras doesn't.

It treats your work like motion. Because work _is_ motion. Things move. Things wait. Things get stuck. Things pass.

If you're going to stare at your tasks every day, they should at least make you smile.

## Install

### Homebrew (macOS / Linux)

```bash
brew install matt-wright86/homebrew-tap/mardi-gras
```

### Go

```bash
go install github.com/matt-wright86/mardi-gras/cmd/mg@latest
```

> **Note**: Make sure `~/go/bin` is on your `PATH`. macOS ships a `/usr/bin/mg` (micro-emacs) that will shadow the binary otherwise.

### From source

```bash
git clone https://github.com/matt-wright86/mardi-gras.git
cd mardi-gras
make build
```

### GitHub Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/matt-wright86/mardi-gras/releases) page.

## Usage

```bash
# Auto-detect .beads/issues.jsonl in current directory
mg

# Point at a specific project
mg --path /path/to/.beads/issues.jsonl

# Treat additional dependency types as blockers
mg --block-types blocks,discovered-from
# or via environment variable
MG_BLOCK_TYPES=blocks,parent-child mg

# Check version
mg --version
```

Mardi Gras reads your `.beads/issues.jsonl` directly — no daemon, no database, no config file. It polls for changes automatically, so if an agent updates an issue while you're watching, the parade reshuffles in real time.

## Live Updates

Mardi Gras polls your JSONL file on a short interval. No OS-specific file watchers. No daemons. No background services.

- External edits (agents, scripts, `bd` commands) are picked up automatically.
- Current view state is preserved on refresh where possible (selection, closed section toggle, active filter query).
- The footer shows your data source and how fresh it is.

## Keybindings

Press `?` from anywhere to open the full help overlay.

### Global

| Key          | Action                     |
| ------------ | -------------------------- |
| `q`          | Quit application           |
| `tab`        | Switch active pane         |
| `?`          | Toggle help overlay        |
| `: / Ctrl+K` | Open command palette      |
| `p`          | Toggle problems view (gt)  |

### Parade

| Key          | Action                                    |
| ------------ | ----------------------------------------- |
| `j` / `k`    | Navigate up/down                         |
| `g` / `G`    | Jump to top / bottom                     |
| `enter`      | Focus detail pane                         |
| `c`          | Toggle closed issues                      |
| `/`          | Enter filter mode                         |
| `f`          | Toggle focus mode (my work + top priority)|
| `a`          | Launch agent (tmux: new window)           |
| `A`          | Kill active agent on issue                |

### Quick Actions

| Key           | Action                                   |
| ------------- | ---------------------------------------- |
| `1` / `2` / `3` | Set status: in_progress / open / closed |
| `!` / `@` / `#` / `$` | Set priority: P1 / P2 / P3 / P4 |
| `b`           | Copy branch name to clipboard            |
| `B`           | Create + checkout git branch             |
| `N`           | Create new issue                         |

### Multi-select

| Key           | Action                              |
| ------------- | ----------------------------------- |
| `space` / `x` | Toggle select on cursor issue      |
| `Shift+J/K`   | Select and move down/up            |
| `X`           | Clear all selections                |
| `1/2/3`       | Bulk set status on selected         |
| `a`           | Sling all selected issues           |
| `s`           | Pick formula and sling all selected |

### Detail Pane

| Key          | Action                     |
| ------------ | -------------------------- |
| `j` / `k`    | Scroll up/down            |
| `esc`        | Back to parade pane        |
| `/`          | Enter filter mode          |
| `a`          | Launch agent               |
| `A`          | Kill active agent          |
| `m`          | Mark active molecule step done |

### Gas Town Panel (`ctrl+g`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `j` / `k`    | Navigate agents/convoys/mail   |
| `g` / `G`    | Jump to first/last             |
| `tab`        | Switch section (agents/convoys/mail) |
| `n`          | Nudge selected agent            |
| `h`          | Handoff work from agent         |
| `K`          | Decommission polecat            |
| `enter`      | Expand/collapse convoy or message |
| `l`          | Land convoy                     |
| `x`          | Close convoy                    |
| `r`          | Reply to selected message       |
| `w`          | Compose new message to agent    |
| `d`          | Archive selected message        |
| `C`          | Create convoy from selection    |

## Filtering

Press `/` and the bottom bar becomes a query input.

- `enter`: keep the query applied and return to list navigation.
- `esc`: clear the query and exit filter mode.
- Multiple terms use `AND` semantics (all terms must match).

Supported query forms:

- Free text: `deploy auth` (matches issue ID and title)
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

## The Parade

Every Beads issue maps to a spot on the parade route:

| On the Route         | What It Means                         |
| -------------------- | ------------------------------------- |
| **Rolling** ●        | In progress — the float is moving     |
| **Lined Up** ♪       | Open and unblocked — waiting its turn |
| **Stalled** ⊘        | Blocked by a dependency               |
| **Past the Stand** ✓ | Done — beads have been thrown         |

Closed issues are collapsed by default (because in any real project, 90%+ of your issues are closed). Press `c` to expand them.

Stalled issues show a "next blocker" hint so you can see at a glance what's holding things up. The detail panel breaks dependencies into four categories: waiting on (active blockers), missing (dangling references), resolved (closed blockers), and related (non-blocking dependency types).

## Detail Panel

Press `enter` on any issue to focus the detail pane. It shows everything about the selected issue:

- **Metadata** — type, priority, assignee, due dates with overdue/due-soon badges
- **Dependencies** — eight types (blocks, blocked-by, related, duplicates, supersedes, parent-child, discovered-from, depends-on) grouped by status: waiting, missing, resolved, and non-blocking
- **Comments & Timeline** — full conversation history with timestamps
- **Molecule DAG** — multi-step workflows rendered as a visual flow graph with parallel branching (`┌─ ├─ └─`) and connector lines between tiers
- **HOP Quality** — reputation stars, crystal/ephemeral badges, and validator verdicts for agent-produced work

Press `m` in the detail pane to mark the active molecule step as done.

## Command Palette

Press `:` or `Ctrl+K` to open a fuzzy-match command palette. Type to filter available actions, then press `enter` to execute. The palette provides access to the same actions available through keybindings, useful when you forget a shortcut.

## tmux Integration

### Status Line Widget

Show parade counts directly in your tmux status bar:

```bash
set -g status-right "#(mg --status)"
```

This outputs a compact, color-coded summary: rolling, lined up, stalled, and closed counts. The `--path` and `--block-types` flags work here too, so you can point at a specific project:

```bash
set -g status-right "#(mg --status --path ~/myproject/.beads/issues.jsonl)"
```

### Popup Dashboard

Launch the full TUI in a tmux popup with a single keybinding:

```bash
bind m display-popup -E -w 80% -h 75% -d "#{pane_current_path}" "mg"
```

- `-E` closes the popup when `mg` exits
- `-w 80% -h 75%` sizes the popup relative to the terminal
- `-d "#{pane_current_path}"` preserves the working directory so `mg` auto-detects the right `.beads/issues.jsonl`

## Claude Code Integration

Press `a` on any selected issue to launch an interactive [Claude Code](https://claude.com/claude-code) session pre-loaded with the full issue context: title, description, notes, acceptance criteria, and dependency status.

### Tmux-native dispatch (multi-agent)

When running inside tmux, agents launch in **new tmux windows** instead of suspending the TUI. This means:

- The parade stays visible while agents work
- Multiple agents can run simultaneously on different issues
- Active agents show a `⚡` badge next to their issue in the parade
- The header displays the total active agent count
- Press `a` on an issue with an active agent to **switch** to its tmux window
- Press `A` to **kill** the active agent on the selected issue
- Agent status is polled automatically alongside the file watcher

### Fallback (non-tmux)

Outside tmux, the TUI suspends while Claude runs (using BubbleTea's `tea.ExecProcess`), giving Claude the full terminal. When you exit the Claude session, Mardi Gras resumes and reloads the JSONL to pick up any changes Claude made.

### Requirements

- Requires `claude` on your `PATH`
- If `claude` is not installed, the `a` key silently does nothing
- Tmux dispatch requires both the `TMUX` env var and `tmux` binary on PATH
- The prompt includes `bd update` and `bd close` hints so Claude knows how to manage the issue lifecycle

## Gas Town Integration

[Gas Town](https://github.com/steveyegge/gastown) is a multi-agent orchestrator for Claude Code. When `gt` is on your PATH, Mardi Gras lights up with a full agent control surface.

### Control Surface (`ctrl+g`)

Press `ctrl+g` to replace the detail pane with the Gas Town dashboard. It has three navigable sections (switch with `tab`):

**Agent Roster** — all agents across rigs with role badges, state (working/idle/backoff), current work assignment, and unread mail count. From here you can nudge (`n`), handoff (`h`), or decommission (`K`) agents.

**Convoys** — delivery batches shown as progress bars with status badges. Expand a convoy with `enter` to see its issues, then land (`l`) or close (`x`) it. Create new convoys from multi-selected issues with `C`.

**Mail** — inbox showing messages between agents. Expand a message with `enter`, reply with `r`, compose a new message with `w`, or archive with `d`.

### Sling & Nudge

When running inside a Gas Town workspace, the `a` key dispatches issues to polecats via `gt sling` instead of launching raw Claude sessions. Additional commands:

- `s` — choose a formula (workflow template) before slinging
- `n` — send a nudge message to the agent working on the selected issue
- `A` — unsling an issue from its polecat

Multi-select (`space` to mark, then `a` or `s`) slings multiple issues in one batch.

### Operational Intelligence

The Gas Town panel includes several data views below the interactive sections:

- **Cost Dashboard** — session counts, token usage, and cost breakdown per agent and time window
- **Activity Feed** — real-time event ticker showing slings, nudges, handoffs, session starts/deaths, and spawns
- **Velocity** — issue flow rates (created/closed today and this week), agent utilization percentage, and cost summary
- **Scorecards** — HOP-powered agent quality ratings aggregated across recent work
- **Predictions** — convoy completion ETAs based on historical throughput

### Problems View (`p`)

Press `p` to toggle the problems view overlay. It scans the current Gas Town status for operational issues:

- Stalled agents (working but no progress)
- Backoff loops (repeated retry failures)
- Zombie sessions (agents that stopped reporting)

### Environment

Gas Town features activate automatically when `gt` is on your PATH. Inside a Gas Town-managed session (polecat, crew, etc.), additional context from `GT_ROLE`, `GT_RIG`, and `GT_SCOPE` env vars appears in the header and Gas Town panel.

## Built with

- [BubbleTea](https://github.com/charmbracelet/bubbletea) — Elm Architecture for the terminal
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — CSS-like styling (the purple, gold, and green)
- [Bubbles](https://github.com/charmbracelet/bubbles) — viewport scrolling

Single binary, no runtime dependencies. Cross-compiles to Linux, macOS, and Windows via [GoReleaser](https://goreleaser.com).

## Design Principles

- Joy over minimalism
- Motion over columns
- Zero configuration
- Human-first visuals
- Beads remains the brain

## What Mardi Gras is not

- Not a project management system
- Not a kanban replacement
- Not a sync layer

It is a visual lens on top of Beads. Beads remains the source of truth.

## Possible Future Ideas

- Color themes (Catppuccin, Dracula)
- Bead throw celebration animation on task close
- Real-time streaming updates (SSE)

No promises. Just dreams. PRs welcome.

## Contributing

Mardi Gras is early. The parade route is laid, the floats are rolling, but there's plenty of room for more krewes. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and guidelines.

## License

[MIT](LICENSE)

---

_Let the good tasks roll._ ⚜
