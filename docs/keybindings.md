# Keybindings

Press `?` from anywhere to open the full help overlay. Inside it, `h` / `l` (or `←` / `→`) page through the sections and `esc`, `q`, or `?` closes it.

Keys marked **(orch)** need a live orchestrator — Gas Town (`gt`) or Gas City. Without one they are inert no-ops rather than errors. A few are narrower still and say so.

## Global

| Key          | Action                     |
| ------------ | -------------------------- |
| `q`          | Quit application           |
| `ctrl+c`     | Quit from anywhere, including forms, dialogs and input bars |
| `tab`        | Switch active pane         |
| `esc`        | Exit focus mode if on, otherwise return to the parade pane |
| `?`          | Toggle help overlay        |
| `: / Ctrl+K` | Open command palette      |
| `/`          | Enter filter mode          |
| `f`          | Toggle focus mode (my work + top priority) |
| `c`          | Toggle closed issues       |
| `ctrl+g`     | Toggle Gas Town panel **(orch)** |
| `p`          | Toggle problems view **(orch)** |
| `D`          | Toggle doctor diagnostics overlay |
| `M`          | Toggle Codex (MCP) live transcript |

`ctrl+g`, `p`, `D` and `M` are mutually exclusive — opening one closes the other three.

## Parade

| Key          | Action                                    |
| ------------ | ----------------------------------------- |
| `j` / `k`    | Navigate down/up                         |
| `g` / `G`    | Jump to top / bottom                     |
| `enter`      | Focus detail pane                         |
| `c`          | Toggle closed issues                      |
| `/`          | Enter filter mode                         |
| `f`          | Toggle focus mode (my work + top priority)|
| `a`          | Launch agent (tmux: new pane; orchestrator: sling) |
| `A`          | Stop the active agent on the issue         |

`a` picks its dispatch path from the environment: with an orchestrator it slings, on the Gas City backend it first prompts for a target agent, in tmux without an orchestrator it opens an agent pane, and outside tmux it suspends the TUI. Pressing `a` on an issue that already has a tmux agent switches to that pane instead of launching a second one. `A` asks the orchestrator to unsling when one is present and only falls back to killing the tmux pane when there is none.

## Quick Actions

| Key           | Action                                   |
| ------------- | ---------------------------------------- |
| `1` / `2` / `3` | Set status: in_progress / open / closed |
| `!` / `@` / `#` / `$` | Set priority: P1 / P2 / P3 / P4 |
| `b`           | Copy branch name to clipboard            |
| `B`           | Create + checkout git branch             |
| `N`           | Create new issue                         |
| `e`           | Edit selected issue (title, priority)    |
| `r`           | Add comment to selected issue            |
| `y`           | Assign selected issue                    |
| `t`           | Add label to selected issue              |
| `l`           | Add dependency link                      |
| `s`           | Pick a formula and sling the issue **(orch)** |
| `n`           | Nudge the agent working the issue **(orch)** |
| `C`           | Create a convoy from the selection **(orch)** |

`3` on a single issue closes it and atomically claims the next ready bead. `n` only fires when an agent is actually active on the selected issue. `C` on an epic builds the convoy from that epic's tree; on any other issue (or a multi-selection) it builds from the selected IDs.

## Multi-select

| Key           | Action                              |
| ------------- | ----------------------------------- |
| `space` / `x` | Toggle select on cursor issue      |
| `Shift+J/K`   | Select and move down/up            |
| `X`           | Clear all selections                |
| `1/2/3`       | Bulk set status on selected         |
| `!/@/#/$`     | Bulk set priority on selected       |
| `a`           | Sling all selected issues           |
| `s`           | Pick formula and sling all selected |
| `C`           | Create a convoy from all selected   |

## Detail Pane

| Key          | Action                     |
| ------------ | -------------------------- |
| `j` / `k`    | Scroll down/up            |
| `esc`        | Back to parade pane        |
| `/`          | Enter filter mode          |
| `a`          | Launch agent               |
| `A`          | Stop the active agent      |
| `m`          | Mark active molecule step done |

Any other navigation key (`pgup`, `pgdn`, `home`, `end`, …) is passed straight to the viewport.

## Gas Town Panel (`ctrl+g`)

The panel takes over the detail pane; give it focus with `tab` (or `enter` from the parade) and these keys route to it instead of the global handlers.

| Key          | Action                          |
| ------------ | ------------------------------- |
| `j` / `k`    | Navigate agents/convoys/mail   |
| `g` / `G`    | Jump to first/last             |
| `tab`        | Switch section (agents/convoys/mail) |
| `n`          | Nudge selected agent (agents)   |
| `h`          | Handoff work from agent (agents) |
| `K`          | Decommission polecat (agents; polecat role only) |
| `enter`      | Expand/collapse convoy or message |
| `l`          | Land convoy (convoys)           |
| `x`          | Close convoy (convoys)          |
| `w`          | Watch convoy (convoys) / compose message (agents, mail) |
| `W`          | Unwatch convoy (convoys)        |
| `r`          | Reply to selected message (mail) |
| `d`          | Archive selected message (mail) |
| `R`          | Mark all mail read (mail)       |

`tab` skips sections with nothing in them, so a town with no convoys cycles agents → mail → agents. Expanding an unread message also marks it read. Note that `l`, `x`, `r`, `d` and `w` mean something different here than they do in the parade — the panel claims them while it has focus.

## Problems View (`p`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `j` / `k`    | Navigate problems              |
| `g` / `G`    | Jump to first/last             |
| `n`          | Nudge agent on selected problem |
| `h`          | Handoff from agent              |
| `K`          | Decommission polecat (polecat role only) |
| `R`          | Recover dead rig — opens a confirmation dialog, then releases + re-slings orphans |

`R` only applies to a `dead_rig` problem; on any other row it does nothing. Recovery shells out to `gt` directly, so it is offered on the Gas Town backend only.

## Doctor Overlay (`D`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `j` / `k`    | Scroll diagnostics             |
| `g` / `G`    | Jump to first/last             |
| `R`          | Re-run `bd doctor`              |

## Codex Transcript (`M`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `M`          | Close the transcript            |
| `r`          | Reply to the live Codex session |
| `esc`        | Dismiss the reply input         |

Approval requests from Codex surface as a modal dialog: `j`/`k` to choose, `enter` to confirm, `esc` to cancel.

## Command Palette (`:` / `Ctrl+K`)

| Key                | Action                     |
| ------------------ | -------------------------- |
| type               | Fuzzy-filter the actions   |
| `up` / `ctrl+p`    | Previous match             |
| `down` / `ctrl+n`  | Next match                 |
| `enter`            | Run selected action        |
| `esc`              | Close the palette          |

The palette also carries actions with no key of their own — add note, claim next ready, prune preview/prune closed, create & assign to crew, cascade close, cycle layout, resume last Codex session, recover dead rigs. See [filtering.md](filtering.md#command-palette).

## Filter Mode (`/`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `enter`      | Keep the query applied and return to list navigation |
| `esc`        | Clear the query and exit        |

Every other printable key is a literal, so `q` and `?` type rather than quit or open help. `ctrl+c` still quits.
