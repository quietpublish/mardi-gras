---
name: gt-test
description: Guide testing mardi-gras in a real Gas Town workspace. Sets up the environment, runs mg against live gt data, and verifies Gas Town panel features.
disable-model-invocation: true
---

# Gas Town Integration Test

Test mardi-gras against a real Gas Town workspace to verify the Gas Town panel, agent roster, convoys, and sling dispatch.

## Prerequisites

Check that the environment is ready:

```bash
which gt          # Gas Town CLI on PATH
which bd          # Beads CLI on PATH
ls ~/gt/          # HQ directory exists
```

If `gt` is not installed, stop and tell the user. Gas Town integration testing requires a running Gas Town installation.

## Step 1: Find or create a workspace

Check for existing Gas Town workspaces:

```bash
ls ~/gt/*/crew/ 2>/dev/null
```

### If a rig exists (e.g., `~/gt/mardi_gras/`)

```bash
# Use the crew workspace
cd ~/gt/mardi_gras/crew/matt
```

### If no rig exists

Guide the user through setup:

```bash
# Rig names MUST use underscores, not hyphens
gt rig create mardi_gras --git <repo-url>
cd ~/gt/mardi_gras/crew/matt
```

## Step 2: Verify beads data is accessible

The crew workspace may have `.beads/redirect` instead of `issues.jsonl`. mg walks up the directory tree to find beads data at the rig root.

```bash
ls .beads/
# If redirect file exists, check where it points
cat .beads/redirect 2>/dev/null
# Verify issues exist
bd list --status=all --json | head -5
```

## Step 3: Run mg from the workspace

```bash
# Build fresh from source
cd ~/Work/mardi-gras && make build

# Run from the Gas Town workspace
cd ~/gt/mardi_gras/crew/matt
~/Work/mardi-gras/mg
```

## Step 4: Verify Gas Town features

Test each feature interactively. Report pass/fail for each:

### Environment detection
- [ ] Footer shows Gas Town indicators (role, rig name)
- [ ] `ctrl+g` opens Gas Town panel

### Agent roster
- [ ] Agent list shows with roles and states
- [ ] State badges render correctly (working, idle, spawning, backoff, stuck, paused, awaiting-gate)
- [ ] Breathing dot animates for working agents
- [ ] Work duration shows next to working agents
- [ ] Heat indicators show for active agents

### Status polling
- [ ] Status loads (may take ~9 seconds for `gt status --json`)
- [ ] Loading state shown while waiting
- [ ] `ctrl+g` triggers on-demand fetch if status is nil
- [ ] No process pile-up (single-flight gate working)

### Sling dispatch (if polecats available)
- [ ] `a` on an issue shows formula picker
- [ ] Sling dispatches via `gt sling`
- [ ] Toast confirms dispatch

### Convoy panel
- [ ] `c` shows convoy list (if any exist)
- [ ] Convoy pipeline visualization renders

### Problems detection
- [ ] `P` shows problems overlay
- [ ] Stalled/backoff/zombie detection works (if applicable)

## Step 5: Verify edge cases

- [ ] mg handles nil status gracefully (kill gt, reopen mg)
- [ ] mg handles missing beads data (rename issues.jsonl temporarily)
- [ ] Exiting mg cleanly (q or ctrl+c) — no orphan processes

## Common issues

- **"gt status --json" hangs**: The command takes ~9 seconds. Wait for it. If it hangs beyond 30s, the exec timeout should kill it.
- **Rig name has hyphens**: Rig names cannot contain hyphens. Use underscores (`mardi_gras`, not `mardi-gras`).
- **No polecats**: Polecats are created by the mayor agent. If none exist, sling features won't work but the roster should still render.
- **Crew workspace has no issues**: `.beads/redirect` points to rig root. mg should find issues there.

## Environment variables

When inside a Gas Town session, these are set:
- `GT_ROLE`: mayor, polecat, crew, witness, refinery, deacon
- `GT_RIG`: rig name
- `GT_SCOPE`: town or rig
- `GT_POLECAT` / `GT_CREW`: worker name

These are NOT set in normal shells — only in `gt`-launched agent sessions.
