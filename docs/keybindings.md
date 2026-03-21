# Keybindings

Press `?` from anywhere to open the full help overlay.

## Global

| Key          | Action                     |
| ------------ | -------------------------- |
| `q`          | Quit application           |
| `tab`        | Switch active pane         |
| `?`          | Toggle help overlay        |
| `: / Ctrl+K` | Open command palette      |
| `p`          | Toggle problems view (gt)  |
| `D`          | Toggle doctor diagnostics overlay |

## Parade

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

## Multi-select

| Key           | Action                              |
| ------------- | ----------------------------------- |
| `space` / `x` | Toggle select on cursor issue      |
| `Shift+J/K`   | Select and move down/up            |
| `X`           | Clear all selections                |
| `1/2/3`       | Bulk set status on selected         |
| `a`           | Sling all selected issues           |
| `s`           | Pick formula and sling all selected |

## Detail Pane

| Key          | Action                     |
| ------------ | -------------------------- |
| `j` / `k`    | Scroll up/down            |
| `esc`        | Back to parade pane        |
| `/`          | Enter filter mode          |
| `a`          | Launch agent               |
| `A`          | Kill active agent          |
| `m`          | Mark active molecule step done |

## Gas Town Panel (`ctrl+g`)

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

## Problems View (`p`)

| Key          | Action                          |
| ------------ | ------------------------------- |
| `j` / `k`    | Navigate problems              |
| `g` / `G`    | Jump to first/last             |
| `n`          | Nudge agent on selected problem |
| `h`          | Handoff from agent              |
| `K`          | Decommission polecat            |
| `R`          | Recover dead rig (release + re-sling orphans) |
