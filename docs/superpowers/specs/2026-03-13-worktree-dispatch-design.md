# Worktree Creation & Agent Dispatch — Design Spec

## Overview

Add the ability to create a git worktree for a bead and store its path as bead metadata. When an agent is dispatched for that bead, it launches in the worktree's directory instead of the project root. This gives each bead an isolated working copy, avoiding branch-switching conflicts when multiple agents work in parallel.

## Keybinding

`W` — create a git worktree for the selected bead and store the path on the bead.

Also available as a command palette action (`ActionCreateWorktree`).

Existing `B` (create branch) and `b` (copy branch name) are unchanged.

## Worktree Creation Flow

### New Function: `CreateWorktree`

Location: `internal/data/mutate.go`

```go
func CreateWorktree(issue Issue, projectDir string) (string, error)
```

Steps:
1. Generate branch name using existing `BranchName(issue)` logic.
2. Compute worktree base dir: `filepath.Join(filepath.Dir(projectDir), filepath.Base(projectDir)+"-worktrees")`. This namespaces worktrees per project (e.g., `mardi-gras-worktrees/`).
3. Create the full worktree parent dir with `os.MkdirAll` — needed because branch names contain slashes (e.g., `feat/mg-d65-...` creates a `feat/` subdirectory). This nested structure is intentional and mirrors the branch prefix grouping.
4. Compute worktree path: `filepath.Join(baseDir, branchName)`.
5. Run `git worktree add <worktree-path> -b <branch-name>` with `projectDir` as cwd. Build `exec.Cmd` manually and set `cmd.Dir = projectDir` (same pattern as `createBranchCmd` in `app.go`).
6. Store absolute worktree path on the bead: `bd update <id> --set-metadata worktree=<abs-path>`.
7. Return the absolute worktree path.

**Branch already exists**: If `-b` fails because the branch exists, retry with `git worktree add <path> <branch>` (without `-b`).

**Branch checked out in main worktree**: If the user previously ran `B` to create+checkout the branch in the main worktree, `git worktree add` will fail because a branch can't be checked out in two places. In this case, show an error toast: "Branch already checked out in main worktree — switch to a different branch first, or use a new bead."

**Partial failure recovery**: If the worktree directory exists on disk but metadata is empty (e.g., previous `bd update` failed), detect this via `os.Stat` on the expected path before calling `git worktree add`. If the directory exists, skip git and retry the metadata write only.

### New Helper: `WorktreePath`

Location: `internal/data/mutate.go`

```go
func WorktreePath(issue Issue) string
```

Reads `issue.Metadata["worktree"]` and returns it as a string. Returns `""` if not set or not a string.

## App Integration

### Keybinding Handler (`internal/app/app.go`)

In the key handler switch, add `W`:

```go
case "W":
    issue := m.parade.SelectedIssue
    if issue == nil {
        return m, nil
    }
    return m, createWorktreeCmd(*issue, m.projectDir)
```

This follows the same pattern as the existing `B` handler (accessing `m.parade.SelectedIssue` directly).

The `createWorktreeCmd` returns a `tea.Cmd` that calls `data.CreateWorktree()` and emits a `worktreeCreatedMsg` on success or `worktreeErrorMsg` on failure.

Message types:

```go
type worktreeCreatedMsg struct {
    issueID string
    path    string
}

type worktreeErrorMsg struct {
    issueID string
    err     error
}
```

On `worktreeCreatedMsg`: show a success toast with the worktree path, and trigger a data refresh so the parade picks up the new metadata.
On `worktreeErrorMsg`: show an error toast.

### Agent Dispatch Modification

In the agent dispatch block (the `"a"` key handler), before building the launch command:

1. Read `data.WorktreePath(issue)`.
2. If non-empty, check that the directory exists on disk (`os.Stat`). This is a local filesystem check and expected to be fast.
3. If it exists, use it as `cwd` instead of `m.projectDir`.
4. If it's set but the directory is missing, fall back to `m.projectDir` and show a warning toast ("Worktree directory missing, using project root").

This applies only to the non-Gas Town path (raw Claude launch and tmux dispatch). The Gas Town `gt sling` path is unchanged — it manages its own workspaces.

```go
cwd := m.projectDir
if wt := data.WorktreePath(*sel); wt != "" {
    if info, err := os.Stat(wt); err == nil && info.IsDir() {
        cwd = wt
    } else {
        // show warning toast
    }
}
```

## Worktree Status Indicators

### Parade View (`internal/views/parade.go`)

When rendering a list item, check `WorktreePath(issue)`. If non-empty, display a worktree symbol after the issue ID. Use `ui.Muted` color if the worktree directory no longer exists on disk.

### Detail View (`internal/views/detail.go`)

Add a line below the existing header fields showing the worktree path when present:

```
Worktree  ../worktrees/feat/mg-d65-worktree-dispatch
```

Rendered with `ui.Muted` color. If the path doesn't exist on disk, render with a warning indicator.

### Symbols (`internal/ui/symbols.go`)

Add a new symbol constant:

```go
Worktree = "⌥"
```

## Help Text

Add `W` to the QUICK ACTIONS section of the help overlay (adjacent to `B`), with description: "Create worktree".

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `git` not on PATH | Error toast: "git not found" |
| Branch already exists, no worktree | Try `git worktree add <path> <branch>` (without `-b`) |
| Branch checked out in main worktree | Error toast: "Branch already checked out — switch branches first" |
| Worktree dir exists but metadata empty | Skip git, retry metadata write only |
| Worktree already exists for this bead | Toast: "Worktree already exists" (read from metadata, confirmed on disk) |
| `bd update --set-metadata` fails | Error toast, but worktree is already created on disk |
| Worktree path missing at dispatch time | Warning toast, fall back to `projectDir` |

## Not In Scope

- **Worktree cleanup/removal** — tracked separately as mg-3gf.
- **Changes to Gas Town sling path** — `gt sling` manages its own workspaces.
- **Worktree creation from the create form** — this is a post-creation action on existing beads.

## Files Changed

| File | Change |
|------|--------|
| `internal/data/mutate.go` | `CreateWorktree()`, `WorktreePath()` |
| `internal/app/app.go` | `W` keybinding, `worktreeCreatedMsg`/`worktreeErrorMsg`, modified agent dispatch cwd logic, `ActionCreateWorktree` palette action |
| `internal/views/parade.go` | Worktree indicator in list item |
| `internal/views/detail.go` | Worktree path line in header |
| `internal/ui/symbols.go` | `Worktree` symbol constant |
| `internal/components/help.go` | `W` entry in QUICK ACTIONS section |

## Testing

- `TestCreateWorktree` — happy path: creates worktree, returns path, sets metadata.
- `TestCreateWorktreeExistingBranch` — branch exists but no worktree yet.
- `TestCreateWorktreeBranchCheckedOut` — branch checked out in main worktree, shows error.
- `TestCreateWorktreePartialFailure` — dir exists but metadata empty, retries metadata only.
- `TestCreateWorktreeAlreadyExists` — metadata already has worktree path.
- `TestWorktreePath` — reads from metadata, handles nil/missing/wrong-type.
- `TestAgentDispatchWithWorktree` — cwd is worktree path when set and valid.
- `TestAgentDispatchMissingWorktree` — falls back to projectDir when path gone.
