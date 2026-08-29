# Gas Town Integration

[Gas Town](https://github.com/gastownhall/gastown) is a multi-agent orchestrator for Claude Code. When `gt` is on your PATH, Mardi Gras lights up with a full agent control surface.

Gas Town is one of two orchestrators mg can drive; the other is [Gas City](gascity.md). mg picks one at startup from evidence about the machine, and **any** Gas Town evidence — `gt` on PATH, or a `GT_*` env var — selects Gas Town ahead of any Gas City evidence. Only an explicit `MG_GC_API` overrides that. See [Which driver mg picks](gascity.md#which-driver-mg-picks).

## Control Surface (`ctrl+g`)

Press `ctrl+g` to replace the detail pane with the Gas Town dashboard. It has three navigable sections (switch with `tab`):

**Agent Roster** — all agents across rigs with role badges, state (working/idle/backoff), current work assignment, and unread mail count. From here you can nudge (`n`), handoff (`h`), or decommission (`K`) agents.

**Convoys** — delivery batches shown as progress bars with status badges, progress percentage, ready/active counts, and assignees. Expand a convoy with `enter` to see its issues, then land (`l`), close (`x`), watch (`w`), or unwatch (`W`) it. Create new convoys from multi-selected issues with `C`, or press `C` on an epic to auto-populate a convoy from its child issues.

**Mail** — inbox showing messages between agents. Expand a message with `enter`, reply with `r`, compose a new message with `w`, archive with `d`, or mark the whole inbox read with `R`.

See [keybindings](keybindings.md) for the full Gas Town Panel and Problems View shortcut reference.

**When `gt` doesn't answer.** `gt status --json` latency is highly variable, so the panel shows a loading line while it waits. If the fetch actually *fails* and there is no earlier status to fall back on, the panel reports the error and what to check — that `gt` is installed and on PATH, and that this directory is inside a Gas Town workspace — rather than spinning forever. Press `ctrl+g` again to retry; a fresh poll clears the error, and a poll that already succeeded keeps its data.

## Sling & Nudge

When running inside a Gas Town workspace, the `a` key dispatches issues to polecats via `gt sling` instead of launching raw Claude sessions. Additional commands:

- `s` — choose a formula (workflow template) before slinging
- `n` — send a nudge message to the agent working on the selected issue
- `A` — unsling an issue from its polecat

Multi-select (`space` to mark, then `a` or `s`) slings multiple issues in one batch.

## Assign to Crew

When an orchestrator is available, the issue create form (`N`) includes a **Crew** field. If you enter a crew member name, Mardi Gras goes through `Driver.Assign` instead of `bd create` — this creates the issue, hooks it to the crew member, and nudges the agent in one step. On Gas Town that is `gt assign`; on [Gas City](gascity.md) it is a single bead create carrying the assignee inline.

The crew field is optional. Leave it empty to create a normal Beads issue.

`gt assign` accepts the same type and priority options as the rest of the create form. Under the hood it runs:

```
gt assign -t <type> -p <priority> [-l <label>] --nudge -- <crew-member> <title>
```

Each flag is omitted when its value is empty, and the create form has no label field, so `-l` is never sent from it.

The crew member must be a valid crew directory in the rig. mg does not pass `--rig`, so it relies on gt's rig inference — which works automatically when crew names are unique across rigs. If a name is ambiguous, run `gt assign --rig <rig>` from the shell instead.

> **Note**: `gt assign` is for crew members. To dispatch work to polecats, use sling (`a`).

## Operational Intelligence

The Gas Town panel includes several data views below the interactive sections:

- **Cost Dashboard** — session counts, token usage, and cost breakdown per agent and time window
- **Vitals** — Dolt server health (port, PID, disk, connections, latency) and backup freshness from `gt vitals`
- **Activity Feed** — event ticker showing slings, nudges, handoffs, session starts/deaths, and spawns, read from `~/gt/.events.jsonl` on local disk
- **Velocity** — issue flow rates (created/closed today and this week), agent utilization percentage, cost summary, and a 7-day dual sparkline showing created vs closed trends
- **Scorecards** — agent quality ratings aggregated across recent work
- **Predictions** — convoy completion ETAs based on historical throughput

The Cost Dashboard, Vitals, and Activity Feed are Gas Town-only — they come from `gt costs`, `gt vitals`, and `~/gt/.events.jsonl`, none of which Gas City provides — so those sections stay empty on a Gas City backend. Velocity, Scorecards, and Predictions are computed from data mg already holds and render on either backend, minus velocity's cost summary.

Note that none of these views stream. mg polls; the `FeatureSSE` capability on the `Driver` seam is declared but unimplemented on **both** backends.

## Problems View (`p`)

Press `p` to toggle the problems view overlay. It combines two sources of diagnostics:

**Agent problems** — detected from Gas Town status:
- **Dead rigs** — rigs with 0 polecats and orphaned work, shown with orphan list. Press `R` to recover (release + re-sling orphaned issues)
- **Stuck agents** — agents explicitly requesting help
- **Stalled agents** — agents with assigned work but sitting idle
- **Backoff loops** — agents stuck in retry cycles
- **Zombie sessions** — agents not running but with hooked work (suppressed on dead rigs)

Dead-rig detection groups all orphaned agents under a single problem instead of emitting individual zombie alerts, reducing alarm fatigue when an entire rig is down.

**Patrol scan** — background polling via `gt patrol scan --json` (requires Gas Town v0.13.0+):
- **Patrol zombies** — agents detected as dead by the patrol system
- **Patrol stalls** — agents detected as stalled by the patrol system
- Augments the agent-level heuristics with patrol-specific diagnostics. Polled every 60 seconds.

**Doctor diagnostics** — from `bd doctor --agent` at startup (also available on-demand via `D`):
- Core system health (Dolt server, config, hooks)
- Git integration issues
- Suggested fix commands for each finding

## Environment

Gas Town features activate automatically when `gt` is on your PATH. Inside a Gas Town-managed session (polecat, crew, etc.), additional context from `GT_ROLE`, `GT_RIG`, and `GT_SCOPE` env vars appears in the header and Gas Town panel.
