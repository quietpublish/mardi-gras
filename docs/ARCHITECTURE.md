# Architecture Overview

Mardi Gras is a terminal UI (TUI) that visualizes [Beads](https://github.com/matt-wright86/beads) issues as a parade -- a motion-based metaphor where issues flow through four stages rather than sitting in static columns.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) (Elm architecture for Go), it reads a `.beads/issues.jsonl` file, groups issues by parade status, and renders a two-pane interface with live file watching.

## Package Layout

```
cmd/mg/
  main.go                 Entry point: flags, path resolution, bootstrap

internal/
  app/
    app.go                Root BubbleTea model (lifecycle, routing, layout)

  data/
    issue.go              Domain types: Issue, Status, Priority, Dependency, DepEval
    loader.go             JSONL parsing, sorting, parade grouping
    filter.go             Query filtering (type:, priority:, free-text)
    watcher.go            File polling (1.2s interval, change detection)

  views/
    parade.go             Left pane: grouped issue list with cursor navigation
    detail.go             Right pane: scrollable issue detail with dependency tree

  components/
    header.go             Title bar with parade counts and progress bar
    footer.go             Keybinding hints and source metadata
    help.go               Modal overlay (keybinding reference)
    float.go              Float/overlay rendering utility

  agent/
    launch.go             Claude Code prompt builder and CLI invocation
    tmux.go               tmux pane integration (launch, discover, kill)

  tmux/
    status.go             tmux status line widget formatter (--status mode)

  ui/
    theme.go              Mardi Gras color palette (purple, gold, green)
    styles.go             Pre-built lipgloss styles
    symbols.go            Unicode symbols for status, deps, borders
    gradient.go           Gradient text rendering
```

### Dependency direction

```
main.go
  --> data     (load issues)
  --> app      (create root model, run TUI)
  --> tmux     (--status mode)

app.Model
  --> views    (Parade, Detail)
  --> components (Header, Footer, Help)
  --> data     (types, watcher, filter, grouping)
  --> agent    (Claude Code launch/tracking)
  --> ui       (theme, styles, symbols)

views, components
  --> data     (Issue, DepEval types)
  --> ui       (styles, symbols)
```

No package imports `app` -- it is the root. `data` and `ui` have no internal dependencies beyond the standard library and lipgloss.

## BubbleTea Model Structure

### Root model (`app.Model`)

The root model owns all state and delegates rendering to sub-models:

```go
type Model struct {
    // Data
    issues        []data.Issue
    groups        map[data.ParadeStatus][]data.Issue

    // Sub-models
    parade        views.Parade       // left pane
    detail        views.Detail       // right pane
    header        components.Header  // top bar

    // UI state
    activPane     Pane               // PaneParade | PaneDetail
    width, height int                // terminal dimensions
    filterInput   textinput.Model    // search/filter bar
    filtering     bool               // filter mode active?
    showHelp      bool               // help overlay visible?

    // File watching
    watchPath     string
    lastFileMod   time.Time

    // Agent integration
    claudeAvail   bool
    activeAgents  map[string]string  // issueID -> tmux paneID
    inTmux        bool
    projectDir    string

    blockingTypes map[string]bool    // dep types that count as blockers
}
```

### Lifecycle

**Init()** starts two concurrent commands:
- `data.WatchFile(path, lastMod)` -- polls the JSONL file every 1.2s
- `pollAgents(inTmux)` -- queries tmux for active Claude panes

**Update(msg)** routes messages:

| Message | Handler |
|---|---|
| `tea.KeyMsg` | Route to help/filter/navigation/agent handlers |
| `tea.WindowSizeMsg` | Recalculate layout, resize sub-models |
| `data.FileChangedMsg` | Reload issues, rebuild parade groups |
| `data.FileUnchangedMsg` | Reschedule watcher |
| `data.FileWatchErrorMsg` | Log and reschedule |
| `agentLaunchedMsg` | Track new agent pane |
| `agentStatusMsg` | Update active agents map |
| `agentFinishedMsg` | Force file reload |

**View()** composes the full screen:

```
+--------------------------------------+
| Header (counts, progress, agents)    |  2 lines
+------------------+-------------------+
| Parade (40%)     | Detail (60%)      |  remaining height - 2
| grouped list     | scrollable        |
+------------------+-------------------+
| Footer (keybindings, source path)    |  1 line
+--------------------------------------+
```

### Sub-models

**`views.Parade`** -- Maintains a flat `[]ParadeItem` list (headers + issue rows + footers), a cursor position, and scroll offset. Renders each parade group with decorated borders. Navigation methods (`MoveUp`, `MoveDown`) skip non-selectable items.

**`views.Detail`** -- Wraps a `viewport.Model` (from bubbles) for scrollable content. Renders the selected issue's metadata, description, notes, and a full dependency breakdown (blocking/resolved/missing/non-blocking/reverse blocks).

**`components.Header`** -- Static render of parade group counts, a progress bar, active agent count, and the decorative bead string.

**`components.Footer`** -- Context-sensitive keybinding hints and source file path.

## Data Flow: JSONL to Parade View

### 1. Bootstrap (cmd/mg/main.go)

```
Parse flags (--path, --block-types, --status, --version)
    |
    v
Resolve JSONL path:
  --path flag given?  -->  use it
  otherwise           -->  walk up from cwd until .beads/issues.jsonl found
    |
    v
data.LoadIssues(path)
  Open file, scan line by line, JSON unmarshal each into data.Issue
  SortIssues: active first, then by priority (asc), then by recency
    |
    v
--status mode?
  yes --> data.GroupByParade() --> tmux.StatusLine() --> print and exit
  no  --> app.New(issues, path, ...) --> tea.NewProgram(model).Run()
```

### 2. Parade grouping (data/loader.go)

`GroupByParade` builds an issue lookup map, then classifies each issue:

```
issue.ParadeGroup(issueMap, blockingTypes):

  closed?                    --> Past the Stand
  in_progress + not blocked? --> Rolling
  in_progress + blocked?     --> Stalled
  open + not blocked?        --> Lined Up
  open + blocked?            --> Stalled
```

"Blocked" is determined by `EvaluateDependencies`: an issue is blocked if it has any dependency where the type is in `blockingTypes` (default: `"blocks"`) and the target issue is either missing or still open.

### 3. Dependency evaluation (data/issue.go)

```
issue.EvaluateDependencies(issueMap, blockingTypes) --> DepEval

For each dependency edge (deduped by type|dependsOnID):
  type not in blockingTypes?       --> NonBlocking
  target not found in issueMap?    --> Missing (counts as blocked)
  target exists and closed?        --> Resolved
  target exists and open?          --> Blocking (counts as blocked)

DepEval.IsBlocked = len(BlockingIDs) > 0 || len(MissingIDs) > 0
```

### 4. Live updates (data/watcher.go)

The watcher is a BubbleTea `Cmd` that polls via `tea.Tick`:

```
WatchFile(path, lastMod):
  sleep 1.2s
  stat the file
    modtime changed? --> LoadIssues, emit FileChangedMsg
    unchanged?       --> emit FileUnchangedMsg
    error?           --> emit FileWatchErrorMsg

app.Update handles the msg, reschedules WatchFile for the next cycle
```

On `FileChangedMsg`, the app reloads issues, rebuilds parade groups, and syncs the selected issue -- preserving cursor position and scroll state.

### 5. Filtering (data/filter.go)

`FilterIssues(issues, query)` tokenizes the query and ANDs all predicates:

- `type:bug` -- match issue type
- `priority:high` / `p0` -- match priority level
- Free text -- substring match on ID or title

## Key Domain Types

```
Issue
  ID, Title, Description, Status, Priority, IssueType
  Owner, Assignee, CreatedAt, UpdatedAt, ClosedAt
  Dependencies []Dependency
  Notes, Design, AcceptanceCriteria, CloseReason

Status:        open | in_progress | closed
IssueType:     task | bug | feature | chore | epic
Priority:      0 (critical) .. 4 (backlog)
ParadeStatus:  Rolling | LinedUp | Stalled | PastTheStand

Dependency
  IssueID      (source -- the issue that has this dep)
  DependsOnID  (target -- the issue being depended on)
  Type         ("blocks", "discovered-from", etc.)

DepEval        (computed from EvaluateDependencies)
  BlockingIDs, ResolvedIDs, MissingIDs, NonBlocking
  IsBlocked, NextBlockerID
```

## Agent Integration

Pressing `a` on a selected issue launches Claude Code with a context-rich prompt:

- **In tmux**: opens a split pane tagged with `@mg_agent=mg-<issueID>` for discovery
- **Outside tmux**: suspends the TUI via `tea.ExecProcess`, resumes on exit

The app polls tmux for active agent panes and displays status badges in the header, parade list, and detail view.

## External Dependencies

All dependencies are from the [Charmbracelet](https://charm.sh/) toolkit:

| Package | Purpose |
|---|---|
| `bubbletea` | Elm-architecture TUI framework |
| `bubbles` | Reusable components (viewport, textinput) |
| `lipgloss` | Terminal styling and layout |
