# Architecture Overview

Mardi Gras is a terminal UI (TUI) that visualizes [Beads](https://github.com/matt-wright86/beads) issues as a parade — a motion-based metaphor where issues flow through four stages rather than sitting in static columns.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) (Elm architecture for Go), it reads a `.beads/issues.jsonl` file, groups issues by parade status, and renders a two-pane interface with live file watching. When [Gas Town](https://github.com/steveyegge/gastown) is available, it becomes a full agent control surface with convoy management, mail, cost analytics, and operational intelligence.

## Package Layout

```
cmd/mg/
  main.go                 Entry point: flags, path resolution, bootstrap

internal/
  app/
    app.go                Root BubbleTea model (lifecycle, routing, layout)
    confetti.go           Confetti celebration animation on issue close

  data/
    issue.go              Domain types: Issue, Status, Priority, Dependency, DepEval
    loader.go             JSONL parsing, sorting, parade grouping
    filter.go             Query filtering (type:, priority:, free-text)
    watcher.go            File polling (1.2s interval, change detection)
    focus.go              Focus mode filtering (my work + top priority)
    mutate.go             Issue mutations via bd CLI (status, priority, create)
    crossrig.go           Cross-rig dependency detection and rendering
    hop.go                HOP (Hierarchy of Proof) quality score types

  views/
    parade.go             Left pane: grouped issue list with cursor navigation
    detail.go             Right pane: scrollable issue detail, deps, molecule DAG
    gastown.go            Gas Town control surface (agents, convoys, mail, costs)
    problems.go           Problems view overlay (stalled agents, backoff, zombies)

  components/
    header.go             Title bar with parade counts and progress bar
    footer.go             Keybinding hints and source metadata
    help.go               Modal overlay (keybinding reference, 8 sections)
    float.go              Float/overlay rendering utility
    palette.go            Command palette (fuzzy-match action search)
    toast.go              Toast notification system (timed dismissal)
    create_form.go        Issue creation form

  agent/
    launch.go             Claude Code prompt builder and CLI invocation
    tmux.go               tmux window integration (launch, discover, kill)

  gastown/
    detect.go             Environment detection (GT_ROLE, GT_RIG, gt on PATH)
    status.go             gt status --json parsing, TownStatus/AgentRuntime types
    sling.go              Issue dispatch: sling, unsling, multi-sling, nudge
    convoy.go             Convoy CRUD: list, create, land, close
    mail.go               Mail inbox, reply, compose, archive, mark-read
    molecule.go           Molecule/DAG types, gt mol integration
    dagrender.go          DAG layout engine: LayoutDAG(), critical path
    problems.go           Problem detection heuristics (stalled, backoff, zombie)
    costs.go              Cost parsing from gt costs
    activity.go           Activity feed event parsing
    velocity.go           Workflow velocity metrics computation
    scorecard.go          HOP agent scorecards (quality aggregates)
    predict.go            Convoy ETA prediction from historical throughput
    recommend.go          Formula recommendation heuristics
    comments.go           Issue comment/timeline fetching

  tmux/
    status.go             tmux status line widget formatter (--status mode)

  ui/
    theme.go              Color palette, RoleColor(), AgentStateColor()
    styles.go             Pre-built lipgloss styles (parade, detail, Gas Town, DAG)
    symbols.go            Unicode symbols (status, deps, borders, DAG connectors)
    gradient.go           Gradient text rendering
    hop.go                HOP badge rendering (stars, crystal/ephemeral indicators)
```

### Dependency direction

```
main.go
  --> data     (load issues)
  --> app      (create root model, run TUI)
  --> tmux     (--status mode)

app.Model
  --> views    (Parade, Detail, GasTown, Problems)
  --> components (Header, Footer, Help, Palette, Toast, CreateForm)
  --> data     (types, watcher, filter, grouping, mutations)
  --> gastown  (detection, status, sling, convoy, mail, costs, ...)
  --> agent    (Claude Code launch/tracking)
  --> ui       (theme, styles, symbols, hop)

views
  --> data     (Issue, DepEval types)
  --> gastown  (TownStatus, AgentRuntime, ConvoyDetail, MailMessage, ...)
  --> ui       (styles, symbols)

components
  --> data     (Issue types for create form)
  --> ui       (styles, symbols)

gastown
  --> (stdlib + encoding/json only, no internal deps)

data
  --> (stdlib only, no internal deps)

ui
  --> (lipgloss only, no internal deps)
```

No package imports `app` — it is the root. `data`, `gastown`, and `ui` have no internal dependencies beyond the standard library and lipgloss.

## BubbleTea Model Structure

### Root model (`app.Model`)

The root model owns all state and delegates rendering to sub-models:

```go
type Model struct {
    // Data
    issues        []data.Issue
    groups        map[data.ParadeStatus][]data.Issue

    // Sub-models (views)
    parade        views.Parade       // left pane
    detail        views.Detail       // right pane
    gasTown       views.GasTown      // Gas Town control surface
    problems      views.Problems     // problems overlay
    header        components.Header  // top bar
    toast         components.Toast   // notification system
    palette       components.Palette // command palette
    createForm    components.CreateForm // issue creation

    // Confetti animation
    confetti      Confetti

    // UI state
    activPane     Pane               // PaneParade | PaneDetail
    width, height int                // terminal dimensions
    filterInput   textinput.Model    // search/filter bar
    filtering     bool               // filter mode active?
    showHelp      bool               // help overlay visible?
    showGasTown   bool               // Gas Town panel replaces detail?
    showProblems  bool               // problems view visible?
    showPalette   bool               // command palette visible?
    creating      bool               // issue creation form visible?
    focusMode     bool               // focus mode active?

    // File watching
    watchPath     string
    lastFileMod   time.Time

    // Agent integration
    claudeAvail   bool
    activeAgents  map[string]string  // issueID -> tmux window name
    inTmux        bool
    projectDir    string

    // Gas Town integration
    gtEnv         gastown.Env         // read once at startup
    townStatus    *gastown.TownStatus // latest gt status, nil until fetched

    // Change detection
    changedIDs    map[string]bool     // recently changed issue IDs
    prevIssueMap  map[string]data.Status // for diffing

    blockingTypes map[string]bool    // dep types that count as blockers

    // ... plus transient state for nudge, convoy create, mail reply/compose,
    //     formula picker, and molecule step operations
}
```

### Lifecycle

**Init()** starts two concurrent commands:
- `data.WatchFile(path, lastMod)` — polls the JSONL file every 1.2s
- `pollAgentState(gtEnv, inTmux)` — queries tmux or `gt status --json` for agent state

**Update(msg)** routes messages. The full message set:

| Message | Handler |
|---|---|
| `tea.KeyMsg` | Route to help/filter/palette/create/navigation/agent/gastown handlers |
| `tea.WindowSizeMsg` | Recalculate layout, resize sub-models |
| **File watching** | |
| `data.FileChangedMsg` | Reload issues, rebuild parade groups, diff for change indicators |
| `data.FileUnchangedMsg` | Reschedule watcher |
| `data.FileWatchErrorMsg` | Log and reschedule |
| **Agent** | |
| `agentLaunchedMsg` | Track new agent window |
| `agentLaunchErrorMsg` | Show toast with error |
| `agentStatusMsg` | Update active agents map |
| `agentFinishedMsg` | Force file reload |
| **Gas Town status** | |
| `townStatusMsg` | Update townStatus, refresh Gas Town panel and header |
| **Sling/dispatch** | |
| `slingResultMsg` | Show toast, reload file |
| `formulaListMsg` | Populate formula picker |
| `unslingResultMsg` | Show toast, reload file |
| `multiSlingResultMsg` | Show toast, reload file |
| `nudgeResultMsg` | Show toast |
| `handoffResultMsg` | Show toast, refresh status |
| `decommissionResultMsg` | Show toast, refresh status |
| **Convoys** | |
| `convoyListMsg` | Update Gas Town panel convoy data |
| `convoyCreateResultMsg` | Show toast, refresh convoys |
| `convoyLandResultMsg` | Show toast, refresh convoys |
| `convoyCloseResultMsg` | Show toast, refresh convoys |
| **Mail** | |
| `mailInboxMsg` | Update Gas Town panel mail data |
| `mailReplyResultMsg` | Show toast, refresh inbox |
| `mailArchiveResultMsg` | Show toast, refresh inbox |
| `mailSendResultMsg` | Show toast, refresh inbox |
| `mailMarkReadResultMsg` | Refresh inbox |
| **Molecule** | |
| `moleculeDAGMsg` | Update detail panel DAG rendering |
| `moleculeStepDoneMsg` | Show toast, refresh molecule |
| **Data enrichment** | |
| `commentsMsg` | Update detail panel comments |
| `costsMsg` | Update Gas Town panel cost data |
| `activityMsg` | Update Gas Town panel activity feed |
| **UI feedback** | |
| `views.GasTownActionMsg` | Dispatch Gas Town panel actions (nudge, handoff, etc.) |
| `mutateResultMsg` | Handle status/priority change results, trigger confetti on close |
| `confettiTickMsg` | Advance confetti animation frame |
| `components.ToastDismissMsg` | Clear toast notification |
| `changeIndicatorExpiredMsg` | Clear change indicator badges |

**View()** composes the full screen:

```
+--------------------------------------+
| Header (counts, progress, agents)    |  2 lines
+------------------+-------------------+
| Parade (40%)     | Detail (60%)      |  remaining height - 2
| grouped list     | or Gas Town panel |
|                  | or Problems view  |
+------------------+-------------------+
| Footer (keybindings, source path)    |  1 line
+--------------------------------------+
| [Toast notification, if active]      |  overlaid
| [Help overlay, if visible]           |  overlaid
| [Command palette, if open]           |  overlaid
| [Create form, if creating]           |  overlaid
| [Confetti, if celebrating]           |  overlaid
```

### Sub-models

**`views.Parade`** — Maintains a flat `[]ParadeItem` list (headers + issue rows + footers), a cursor position, and scroll offset. Renders each parade group with decorated borders. Navigation methods (`MoveUp`, `MoveDown`) skip non-selectable items. Supports multi-select (`selectedIDs` set) for bulk operations.

**`views.Detail`** — Wraps a `viewport.Model` (from bubbles) for scrollable content. Renders the selected issue's metadata, description, notes, due dates, HOP quality badges, full dependency breakdown (blocking/resolved/missing/non-blocking/reverse), comments/timeline, and molecule DAG visualization.

**`views.GasTown`** — Three-section control surface (agents/convoys/mail) that replaces the detail pane when active. Navigable with `tab` between sections and `j/k` within. Renders agent roster with role badges and state colors, convoy progress bars with expand/collapse, mail inbox with unread counts, cost dashboard, activity feed, velocity metrics, scorecards, and predictions. Emits `GasTownActionMsg` for user actions.

**`views.Problems`** — Overlay showing operational issues detected from Gas Town status: stalled agents, backoff loops, zombie sessions.

**`components.Header`** — Parade group counts, progress bar, active agent count, Gas Town role badge, problem warning indicator, and the decorative bead string.

**`components.Footer`** — Context-sensitive keybinding hints and source file path with freshness indicator.

**`components.Palette`** — Command palette with fuzzy matching over available actions.

**`components.Toast`** — Timed notification overlay (4s auto-dismiss) for operation feedback.

**`components.CreateForm`** — Multi-field issue creation form with type, priority, title, and description inputs.

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

Eight dependency types are supported: blocks, blocked-by, related, duplicates, supersedes, parent-child, discovered-from, depends-on. The `--block-types` flag controls which types are treated as blockers (default: `blocks` only).

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

On `FileChangedMsg`, the app reloads issues, rebuilds parade groups, diffs against `prevIssueMap` to detect status changes (for change indicator badges), and syncs the selected issue — preserving cursor position and scroll state.

### 5. Filtering (data/filter.go)

`FilterIssues(issues, query)` tokenizes the query and ANDs all predicates:

- `type:bug` — match issue type
- `priority:high` / `p0` — match priority level
- Free text — substring match on ID or title

### 6. Focus mode (data/focus.go)

`FocusFilter(issues)` returns the subset relevant to the current user: their in-progress work plus the top ready and blocked issues. Activated with `f`.

## Gas Town Integration

The `internal/gastown` package handles all Gas Town interaction. It has no internal dependencies — only stdlib and `encoding/json`.

### Environment Detection (detect.go)

`Detect()` reads environment variables and checks PATH at startup:

```go
type Env struct {
    Available bool   // gt binary on PATH
    Active    bool   // running inside a Gas Town-managed session
    Role      string // GT_ROLE: mayor, polecat, crew, witness, refinery, deacon
    Rig       string // GT_RIG: rig name
    Scope     string // GT_SCOPE: town or rig
    Worker    string // GT_POLECAT or GT_CREW: worker name
}
```

Features activate progressively: Beads-only (no gt) → Gas Town available (gt on PATH) → Inside Gas Town (GT_ROLE set).

### Status Polling (status.go)

`FetchStatus()` runs `gt status --json` and parses the result. Key gotcha: `gt status --json` takes **~9 seconds** to complete. The JSON nests agents under `rigs[].agents`; `normalizeStatus()` flattens them into a single `Agents` slice for the UI. Top-level agents are HQ-level (mayor, deacon); rig agents include polecats, crew, witness, refinery.

If `AgentRuntime.State` is empty, state is inferred from `Running`: true → "working", false → "idle".

### Sling & Dispatch (sling.go)

- `Sling(issue, rig)` — dispatch to polecat via `gt sling`
- `SlingWithFormula(issue, rig, formula)` — sling with specific formula
- `MultiSling(issues, rig)` — batch dispatch
- `Unsling(issue)` — remove assignment
- `Nudge(address, message)` — send nudge to agent
- `Handoff(address)` — handoff work from agent
- `Decommission(address)` — decommission polecat

### Convoy Management (convoy.go)

- `ListConvoys()` — fetch all convoys
- `CreateConvoy(name, issueIDs)` — create from issue selection
- `LandConvoy(id)` — land (close + cleanup)
- `CloseConvoy(id)` — close without landing

### Mail (mail.go)

- `FetchInbox()` — get all messages
- `MarkRead(id)` — mark message read
- `Reply(id, body)` — reply to message
- `Archive(id)` — archive message
- `Send(address, subject, body)` — compose new message

### DAG Rendering (dagrender.go)

`LayoutDAG(dag)` converts a `DAGInfo` (tiers of molecule steps) into `[]DAGRow` for visual rendering:

- `RowSingle` — one node per tier (linear chain)
- `RowParallel` — multiple nodes per tier (branching)
- `RowConnector` — flow connector line (`│`) between tiers

`CriticalPathSet()`, `CriticalPathTitles()`, and `CriticalPathString()` identify and render the critical path through the molecule using human-readable step titles.

### Analytics (costs.go, activity.go, velocity.go, scorecard.go, predict.go, recommend.go)

Each file handles one data domain:
- **costs.go** — Parse `gt costs` output for per-agent token/cost breakdown
- **activity.go** — Parse event streams for the activity feed
- **velocity.go** — Compute issue flow rates and agent utilization
- **scorecard.go** — Aggregate HOP quality scores per agent
- **predict.go** — Convoy ETA estimation from historical throughput
- **recommend.go** — Formula recommendation based on issue characteristics

## Key Domain Types

```
Issue
  ID, Title, Description, Status, Priority, IssueType
  Owner, Assignee, CreatedAt, UpdatedAt, ClosedAt
  Dependencies []Dependency
  Notes, Design, AcceptanceCriteria, CloseReason
  DueDate, Comments []Comment
  QualityScore, Crystallizes, Creator, Validations  (HOP fields)

Status:        open | in_progress | closed
IssueType:     task | bug | feature | chore | epic
Priority:      0 (critical) .. 4 (backlog)
ParadeStatus:  Rolling | LinedUp | Stalled | PastTheStand

Dependency
  IssueID      (source -- the issue that has this dep)
  DependsOnID  (target -- the issue being depended on)
  Type         ("blocks", "blocked-by", "related", "duplicates",
                "supersedes", "parent-child", "discovered-from", "depends-on")

DepEval        (computed from EvaluateDependencies)
  BlockingIDs, ResolvedIDs, MissingIDs, NonBlocking
  IsBlocked, NextBlockerID

AgentRuntime   (from gastown/status.go)
  Name, Address, Role, State, Running
  HasWork, WorkTitle, HookBead, UnreadMail

TownStatus     (from gastown/status.go)
  Agents []AgentRuntime  (flattened from all rigs)
  Rigs   []RigStatus

ConvoyDetail   (from gastown/convoy.go)
  ID, Title, Status, Issues, Progress

MailMessage    (from gastown/mail.go)
  ID, From, Subject, Body, Timestamp, Read
```

## Agent Integration

Pressing `a` on a selected issue launches Claude Code with a context-rich prompt. Behavior depends on environment:

- **In Gas Town**: dispatches via `gt sling` to assign the issue to a polecat
- **In tmux (no Gas Town)**: opens a new tmux window tagged with `@mg_agent=mg-<issueID>` for discovery
- **Outside tmux**: suspends the TUI via `tea.ExecProcess`, resumes on exit

The app polls for agent state: tmux windows (when in tmux) or `gt status --json` (when Gas Town available). Status badges appear in the header, parade list, and detail view.

Additional agent operations from the Gas Town panel:
- `n` — nudge agent with a message
- `h` — handoff work from an agent to another
- `K` — decommission a polecat

## UI Architecture

### Theme System (ui/)

All visual constants live in `internal/ui/`:

- **theme.go** — Color palette (Mardi Gras purple, gold, green), plus `RoleColor()` for Gas Town agent roles and `AgentStateColor()` for working/idle/backoff states
- **styles.go** — Pre-built lipgloss styles for every context: parade items, detail sections, Gas Town panel, DAG connectors, toast notifications, command palette
- **symbols.go** — Unicode symbols: status indicators (●, ♪, ⊘, ✓), dependency arrows, DAG flow connectors (│, ┌, ├, └), progress bars
- **hop.go** — HOP badge rendering (star ratings, crystal/ephemeral indicators)

Convention: views and components import `ui` for all visual constants. No raw colors or symbols in view code.

### Receiver Conventions

- **Value receivers** on BubbleTea models (`Update`, `View`) — required by the Elm architecture
- **Pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`) — internal state updates

## External Dependencies

All TUI dependencies are from the [Charmbracelet](https://charm.sh/) toolkit:

| Package | Purpose |
|---|---|
| `bubbletea` | Elm-architecture TUI framework |
| `bubbles` | Reusable components (viewport, textinput) |
| `lipgloss` | Terminal styling and layout |
| `x/ansi` | ANSI string width and truncation |

Plus:
| Package | Purpose |
|---|---|
| `atotto/clipboard` | Cross-platform clipboard access (branch name copy) |
