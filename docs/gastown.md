# Gas Town Integration

[Gas Town](https://github.com/steveyegge/gastown) is a multi-agent orchestrator for Claude Code. When `gt` is on your PATH, Mardi Gras lights up with a full agent control surface.

## Control Surface (`ctrl+g`)

Press `ctrl+g` to replace the detail pane with the Gas Town dashboard. It has three navigable sections (switch with `tab`):

**Agent Roster** — all agents across rigs with role badges, state (working/idle/backoff), current work assignment, and unread mail count. From here you can nudge (`n`), handoff (`h`), or decommission (`K`) agents.

**Convoys** — delivery batches shown as progress bars with status badges, progress percentage, ready/active counts, and assignees. Expand a convoy with `enter` to see its issues, then land (`l`) or close (`x`) it. Create new convoys from multi-selected issues with `C`, or press `C` on an epic to auto-populate a convoy from its child issues.

**Mail** — inbox showing messages between agents. Expand a message with `enter`, reply with `r`, compose a new message with `w`, or archive with `d`.

See [keybindings](keybindings.md) for the full Gas Town Panel and Problems View shortcut reference.

## Sling & Nudge

When running inside a Gas Town workspace, the `a` key dispatches issues to polecats via `gt sling` instead of launching raw Claude sessions. Additional commands:

- `s` — choose a formula (workflow template) before slinging
- `n` — send a nudge message to the agent working on the selected issue
- `A` — unsling an issue from its polecat

Multi-select (`space` to mark, then `a` or `s`) slings multiple issues in one batch.

## Assign to Crew

When Gas Town is available, the issue create form (`N`) includes a **Crew** field. If you enter a crew member name, Mardi Gras uses `gt assign` instead of `bd create` — this creates the issue, hooks it to the crew member, and nudges the agent in one step.

The crew field is optional. Leave it empty to create a normal Beads issue.

`gt assign` accepts the same type and priority options as the rest of the create form. Under the hood it runs:

```
gt assign --nudge -t <type> -p <priority> -- <crew-member> <title>
```

The crew member must be a valid crew directory in the rig. If you're not in a Gas Town workspace, use `--rig` to specify the rig name. Rig inference works automatically if crew names are unique across rigs.

> **Note**: `gt assign` is for crew members. To dispatch work to polecats, use sling (`a`).

## Operational Intelligence

The Gas Town panel includes several data views below the interactive sections:

- **Cost Dashboard** — session counts, token usage, and cost breakdown per agent and time window
- **Vitals** — Dolt server health (port, PID, disk, connections, latency) and backup freshness from `gt vitals`
- **Activity Feed** — real-time event ticker showing slings, nudges, handoffs, session starts/deaths, and spawns
- **Velocity** — issue flow rates (created/closed today and this week), agent utilization percentage, cost summary, and a 7-day dual sparkline showing created vs closed trends
- **Scorecards** — HOP-powered agent quality ratings aggregated across recent work
- **Predictions** — convoy completion ETAs based on historical throughput

## Problems View (`p`)

Press `p` to toggle the problems view overlay. It combines two sources of diagnostics:

**Agent problems** — detected from Gas Town status:
- **Dead rigs** — rigs with 0 polecats and orphaned work, shown with orphan list. Press `R` to recover (release + re-sling orphaned issues)
- **Stuck agents** — agents explicitly requesting help
- **Stalled agents** — agents with assigned work but sitting idle
- **Backoff loops** — agents stuck in retry cycles
- **Zombie sessions** — agents not running but with hooked work (suppressed on dead rigs)

Dead-rig detection groups all orphaned agents under a single problem instead of emitting individual zombie alerts, reducing alarm fatigue when an entire rig is down.

**Doctor diagnostics** — from `bd doctor --agent` at startup (also available on-demand via `D`):
- Core system health (Dolt server, config, hooks)
- Git integration issues
- Suggested fix commands for each finding

## Environment

Gas Town features activate automatically when `gt` is on your PATH. Inside a Gas Town-managed session (polecat, crew, etc.), additional context from `GT_ROLE`, `GT_RIG`, and `GT_SCOPE` env vars appears in the header and Gas Town panel.
