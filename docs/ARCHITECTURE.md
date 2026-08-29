# Architecture Overview

Mardi Gras is a terminal UI (TUI) that visualizes [Beads](https://github.com/gastownhall/beads) issues as a parade — a motion-based metaphor where issues flow through four stages rather than sitting in static columns.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) v2 (Elm architecture for Go), it supports two data sources: direct `.beads/issues.jsonl` reading and `bd list --json` CLI fallback (for Beads v0.56+ with Dolt). It groups issues by parade status and renders a two-pane interface with live polling. When an orchestrator — [Gas Town](https://github.com/gastownhall/gastown) or [Gas City](https://github.com/gastownhall/gascity) — is available, it becomes a full agent control surface with convoy management, mail, cost analytics, and operational intelligence.

## Package Layout

```
cmd/mg/
  main.go                 Entry point: flags, path resolution, bootstrap

internal/
  app/
    app.go                Root BubbleTea model (lifecycle, routing, layout)
    codex.go              Codex MCP session lifecycle, transcript + approval routing
    confetti.go           Confetti celebration animation on issue close
    deferred_keys.go      Short-delay key staging so the OSC guard can spot fragments
    oscguard.go           Filters terminal capability-reply traffic out of the key stream
    debug.go              Opt-in action/route/state logging

  data/
    issue.go              Domain types: Issue, Status, Priority, Dependency, DepEval
    loader.go             JSONL parsing, sorting, parade grouping
    filter.go             Query filtering (type:, label:, priority:, free-text)
    watcher.go            File polling (1.2s JSONL / 5s CLI interval, change detection)
    source.go             Source config (JSONL vs CLI), bd list/show fetchers, bd version warning
    source_health.go      Consecutive-failure tracking for the CLI source
    focus.go              Focus mode filtering (my work + top priority)
    mutate.go             Issue mutations via bd CLI (status, priority, create, claim, note, prune)
    metadata.go           Beads config parsing, metadata schema, ResolveBeadsDir
    exec.go               Timeout helpers for bd/git commands, bd doctor, SchemaSkewHint
    validate.go           Input validation for user-supplied mutation arguments
    crossrig.go           Cross-rig dependency detection and rendering


  views/
    parade.go             Left pane: grouped issue list with cursor navigation
    detail.go             Right pane: scrollable issue detail, deps, molecule DAG
    gastown.go            Orchestrator control surface (agents, convoys, mail, costs)
    problems.go           Problems view overlay (stalled agents, backoff, zombies)
    doctor.go             bd doctor diagnostics overlay
    codex_transcript.go   Live Codex (MCP) event transcript panel

  components/
    header.go             Title bar with parade counts and progress bar
    footer.go             Keybinding hints, source metadata, divider, bulk-mode bar
    help.go               Modal overlay (paginated keybinding reference, 9 sections)
    float.go              Float/overlay rendering utility
    palette.go            Command palette (fuzzy-match action search)
    toast.go              Toast notification system (timed dismissal, single-row fit)
    create_form.go        Issue creation form
    edit_form.go          Issue edit form (title, priority)
    approval_dialog.go    Codex exec/apply-patch approval modal
    recovery_dialog.go    Dead-rig recovery confirmation modal

  agent/
    launch.go             Runtime detection (claude/cursor-agent/codex), prompt builder, CLI invocation
    tmux.go               tmux pane integration (launch, discover, capture, focus, kill)
    codex.go              Codex session-file discovery and `codex resume --last`
    codex_mcp.go          Codex MCP subprocess handle (spawn, events, reply, close)

  gastown/
    driver.go             Driver interface (the orchestrator seam) + Feature/ErrUnsupported/SlingRequest
    gt_driver.go          GTDriver: Gas Town impl, delegates to the gt CLI wrappers below
    gc.go                 Evidence-based driver selection + MG_GC_API base-URL discovery
    gc_driver.go          GCDriver: Gas City impl over the Supervisor HTTP API
    gcclient/             Generated (oapi-codegen) Gas City Supervisor API client
    detect.go             Environment detection (GT_ROLE, GT_RIG, gt/gc on PATH, city.toml)
    assign.go             Create + hook a bead to a crew member (gt assign)
    validate.go           Argument validation for values passed to gt
    exec.go               Timeout helpers for gt commands (short/medium/long tiers)
    status.go             gt status --json parsing, TownStatus/AgentRuntime types
    sling.go              Issue dispatch: sling, unsling, multi-sling, nudge
    convoy.go             Convoy CRUD: list, create, land, close
    mail.go               Mail inbox, reply, compose, archive, mark-read
    molecule.go           Molecule/DAG types, gt mol integration
    dagrender.go          DAG layout engine: LayoutDAG(), critical path
    problems.go           Problem detection heuristics (stalled, stuck, backoff, zombie, dead_rig)
    patrol.go             Patrol scan integration: gt patrol scan --json parsing, patrol-sourced problems
    recovery.go           Dead-rig recovery: orphan detection, release + re-sling
    costs.go              Cost parsing from gt costs
    vitals.go             Server health + backup freshness from gt vitals
    activity.go           Activity feed event parsing
    velocity.go           Workflow velocity metrics computation
    scorecard.go          Agent scorecards (quality aggregates)
    predict.go            Convoy ETA prediction from historical throughput
    recommend.go          Formula recommendation heuristics
    comments.go           Issue comment/timeline fetching

  codexmcp/
    proto.go              JSON-RPC + codex/event wire types
    transport.go          stdio transport over the `codex mcp-server` subprocess
    client.go             Handshake, request/response, notification fan-out
    session.go            One Codex session: events, replies, server requests

  tmux/
    status.go             tmux status line widget formatter (--status mode)

  ui/
    theme.go              Switchable color palette (SetTheme), RoleColor(), AgentStateColor()
    styles.go             Pre-built lipgloss styles (parade, detail, Gas Town, DAG)
    symbols.go            Unicode symbols (status, deps, borders, DAG connectors)
    gradient.go           Gradient text rendering
    glamour.go            Markdown rendering style derived from the active theme
    sparkline.go          Inline sparkline rendering for metrics
```

### Dependency direction

```
main.go
  --> data     (load issues)
  --> app      (create root model, run TUI)
  --> tmux     (--status mode)

app.Model
  --> views    (Parade, Detail, GasTown, Problems, Doctor, CodexTranscript)
  --> components (Header, Footer, Help, Palette, Toast, forms, dialogs)
  --> data     (types, watcher, filter, grouping, mutations)
  --> gastown  (Driver, detection, status, sling, convoy, mail, costs, ...)
  --> agent    (runtime detection, launch/tracking, Codex MCP handle)
  --> codexmcp (Codex event + session types)
  --> ui       (theme, styles, symbols)

views
  --> data     (Issue, DepEval types)
  --> gastown  (TownStatus, AgentRuntime, ConvoyDetail, MailMessage, ...)
  --> codexmcp (Codex event types for the transcript panel)
  --> ui       (styles, symbols)

components
  --> data     (Issue types for create form)
  --> gastown  (TownStatus for the header, OrphanedIssue for the recovery dialog)
  --> ui       (styles, symbols)

agent
  --> data     (Issue/DepEval for the prompt builder)
  --> codexmcp (MCP client for the in-app Codex session)

gastown (core: status, sling, convoy, mail, molecule, problems, recovery, detect)
  --> gcclient (generated Gas City client, used by gc_driver.go)

gastown (analytics: velocity, predict, scorecard, recommend)
  --> data     (Issue types for metrics computation)

codexmcp
  --> (stdlib only, no internal deps)

data
  --> (stdlib only, no internal deps)

ui
  --> (lipgloss + glamour only, no internal deps)
```

No package imports `app` — it is the root. `data`, `ui` and `codexmcp` have no internal dependencies. Core `gastown` files (status, sling, convoy, mail, molecule, problems, recovery, detect) still import nothing from `internal/data`; only the analytics files (velocity, predict, scorecard, recommend) do, for issue types. The generated `gcclient` is the one internal import the core `gastown` package carries, and it is confined to `gc_driver.go`.

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
    gasTown       views.GasTown      // orchestrator control surface
    problems      views.Problems     // problems overlay
    doctor        views.Doctor       // bd doctor diagnostics overlay
    codexTranscript views.CodexTranscript // live Codex (MCP) transcript
    header        components.Header  // top bar
    toast         components.Toast   // notification system
    palette       components.Palette // command palette
    createForm    components.CreateForm // issue creation
    editForm      components.EditForm   // issue edit

    // Confetti animation
    confetti      Confetti

    // UI state
    activPane     Pane               // PaneParade | PaneDetail
    width, height int                // terminal dimensions
    layoutPreset  LayoutPreset       // LayoutDefault | LayoutGasTown | LayoutWide
    filterInput   textinput.Model    // search/filter bar
    filtering     bool               // filter mode active?
    showHelp      bool               // help overlay visible?
    showGasTown   bool               // orchestrator panel replaces detail?
    showProblems  bool               // problems view visible?
    showDoctor    bool               // doctor overlay visible?
    showCodex     bool               // codex transcript visible?
    showPalette   bool               // command palette visible?
    creating      bool               // issue creation form visible?
    editing       bool               // issue edit form visible?
    focusMode     bool               // focus mode active?

    // Data source
    sourceMode    data.SourceMode    // JSONL file watching or CLI polling
    watchPath     string             // JSONL path (SourceJSONL only)
    lastFileMod   time.Time          // last known modtime (JSONL only)
    sourceHealth  data.SourceHealth  // consecutive-failure tracking (CLI mode)

    // Agent integration
    agentAvail    bool
    agentRuntime  agent.Runtime
    activeAgents  map[string]string  // issueID -> tmux pane ID
    codexSessions map[string]*codexSession // issueID -> live Codex MCP session
    inTmux        bool
    projectDir    string

    // Orchestrator integration
    driver        gastown.Driver      // GTDriver or GCDriver, chosen at startup
    gtEnv         gastown.Env         // read once at startup
    townStatus    *gastown.TownStatus // latest status, nil until fetched
    townStatusErr error               // last status failure; stops the panel
                                      // spinning forever on a dead backend

    // Filters from flags
    excludeTypes  map[string]bool     // --exclude-type
    excludeLabels map[string]bool     // --exclude-label

    // Change detection
    changedIDs    map[string]bool     // recently changed issue IDs
    prevIssueMap  map[string]data.Status // for diffing

    blockingTypes map[string]bool    // dep types that count as blockers

    // ... plus transient state for nudge, convoy create, mail reply/compose,
    //     formula picker, and molecule step operations
}
```

### Lifecycle

**Init()** batches the startup commands:
- `m.startPoll()` — JSONL mode: `data.WatchFile(path, lastMod)` polls every 1.2s; CLI mode: `data.PollCLI(projectDir)` runs `bd list --json` every 5s
- Agent state poll — `pollGTStatus` when an orchestrator is available, otherwise `pollTmuxAgentState` when running in tmux, otherwise nothing. Subsequent polls use a single-flight gate (`gtPollInFlight`) to prevent overlapping status calls, which are slow; Init bypasses the gate for the first poll, and calls from the watcher and user actions go through `gatedPollAgentState()`.
- Header shimmer + spinner ticks, unless `--no-animations`
- In CLI mode only: `fetchCurrentIssue`, `fetchDoctorDiagnostics`, and `fetchBeadsContext` (the last also carries the `bd` version, which feeds `data.BdVersionWarning`)

**Update(msg)** routes messages. The main ones:

| Message | Handler |
|---|---|
| `tea.KeyPressMsg` | Route to help/filter/palette/form/dialog/input-bar handlers, then `handleKey` |
| `tea.WindowSizeMsg` | Recalculate layout, resize sub-models |
| **File watching** | |
| `data.FileChangedMsg` | Reload issues, rebuild parade groups, diff for change indicators |
| `data.FileUnchangedMsg` | Reschedule watcher |
| `data.FileWatchErrorMsg` | JSONL: log and reschedule. CLI: show error toast and reschedule |
| **Agent** | |
| `agentLaunchedMsg` | Track new agent pane |
| `agentLaunchErrorMsg` | Show toast with error |
| `agentStatusMsg` | Update active agents map |
| `agentFinishedMsg` | Force file reload |
| **Orchestrator status** | |
| `townStatusMsg` | Update `townStatus` on success; on failure record `townStatusErr` and hand the panel an error state instead of leaving it loading forever |
| `patrolScanMsg` | Merge `gt patrol scan` findings into the problems list |
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
| `convoyWatchResultMsg` / `convoyUnwatchResultMsg` | Show toast, refresh convoys |
| **Mail** | |
| `mailInboxMsg` | Update Gas Town panel mail data |
| `mailReplyResultMsg` | Show toast, refresh inbox |
| `mailArchiveResultMsg` | Show toast, refresh inbox |
| `mailSendResultMsg` | Show toast, refresh inbox |
| `mailMarkReadResultMsg` / `mailMarkAllReadResultMsg` | Refresh inbox |
| **Molecule** | |
| `moleculeDAGMsg` | Update detail panel DAG rendering |
| `moleculeStepDoneMsg` | Show toast, refresh molecule |
| **Data enrichment** | |
| `commentsMsg` | Update detail panel comments |
| `issueDetailMsg` | Merge notes/design/acceptance criteria from `bd show` into the selected issue |
| `doctorResultMsg` | Populate the doctor overlay and the problems list |
| `beadsContextMsg` | Store the Beads context; warn via toast on a known-bad `bd` version |
| `currentIssueMsg` | Highlight bd's active issue in the header |
| `costsMsg` | Update Gas Town panel cost data |
| `vitalsMsg` | Update Gas Town panel server health + backups |
| `activityMsg` | Update Gas Town panel activity feed |
| **UI feedback** | |
| `views.GasTownActionMsg` | Dispatch Gas Town panel actions (nudge, handoff, etc.) |
| `views.RecoveryActionMsg` | Initiate dead-rig recovery (release + re-sling orphans) |
| `recoveryResultMsg` | Show recovery result toast, refresh Gas Town status |
| `mutateResultMsg` | Handle status/priority change results, trigger confetti on close |
| `confettiTickMsg` | Advance confetti animation frame |
| `components.ToastDismissMsg` | Clear toast notification |
| `changeIndicatorExpiredMsg` | Clear change indicator badges |
| `pruneResultMsg` / `claimNextReadyMsg` | Show toast; claim also selects the claimed issue |
| **Codex (MCP)** | |
| `codexLaunchedMsg` / `codexLaunchErrorMsg` | Attach or fail the in-app Codex session |
| `codexEventMsg` | Append one Codex event to the transcript |
| `codexApprovalRequestMsg` / `codexApprovalResolvedMsg` | Raise and resolve the approval modal |
| `codexReplyDispatchedMsg` / `codexReplyErrorMsg` | Confirm or report a follow-up prompt |
| `codexDoneMsg` | Finalize the session (status, duration, token usage) |
| **Internal** | |
| `deferredKeyMsg` | Deliver a printable key held for OSC-fragment detection |

**View()** composes the full screen:

```
+--------------------------------------+
| Header (counts, progress, agents)    |  2 lines
+------------------+-------------------+
| Parade (2/5)     | Detail (3/5)      |  height - 4
| grouped list     | or Gas Town panel |
|                  | or Problems view  |
|                  | or Doctor overlay |
|                  | or Codex script   |
+------------------+-------------------+
| Divider, OR the active toast         |  1 line
+--------------------------------------+
| Footer, OR the active input bar      |  1 line
+--------------------------------------+
| [Help overlay, if visible]           |  full-screen replacement
| [Command palette, if open]           |  full-screen replacement
| [Create/edit form, dialogs]          |  full-screen replacement
| [Confetti, if celebrating]           |  overlaid
```

Two details are easy to get wrong. First, the toast is **not** an overlay — it takes over the one-row divider between the body and the footer, so key hints stay visible while a toast shows. `Toast.View` therefore collapses the message to a single line and truncates it with `fitToastLine`; it does not wrap, which is what used to push the layout down a row on long messages. Second, the bottom row is a switch, not a stack: a bulk-selection bar, the nudge / sling-target / quick-action / mail / convoy / codex-reply input, the filter bar (with a right-aligned `N/M match` count), or the footer — whichever applies, in that order.

The `LayoutWide` preset drops the right panel and gives the parade the full width; `LayoutGasTown` opens the orchestrator panel alongside the parade.

### Sub-models

**`views.Parade`** — Maintains a flat `[]ParadeItem` list (headers + issue rows + footers), a cursor position, and scroll offset. Renders each parade group with decorated borders. Navigation methods (`MoveUp`, `MoveDown`) skip non-selectable items. Supports multi-select (`selectedIDs` set) for bulk operations.

**`views.Detail`** — Wraps a `viewport.Model` (from bubbles) for scrollable content. Renders the selected issue's metadata, description, notes, due dates, full dependency breakdown (blocking/resolved/missing/non-blocking/reverse), comments/timeline, and molecule DAG visualization.

**`views.GasTown`** — Three-section control surface (agents/convoys/mail) that replaces the detail pane when active. Navigable with `tab` between sections and `j/k` within. Renders agent roster with role badges and state colors, convoy progress bars with expand/collapse, mail inbox with unread counts, cost dashboard, vitals (server health + backup freshness), activity feed, velocity metrics, scorecards, and predictions. Emits `GasTownActionMsg` for user actions. `SetStatusError(err, backend)` gives it a third state alongside "loading" and "populated": when a status fetch fails it explains what is unreachable and what to check, rather than animating a spinner at a backend that will never answer. Sections whose backend reports `ErrUnsupported` (vitals, costs, patrol on Gas City) are hidden, not shown as errors.

**`views.Problems`** — Overlay showing operational issues detected from orchestrator status: dead rigs (with orphan list and `R` recovery action), stuck agents, stalled agents, backoff loops, zombie sessions, plus `patrol_zombie` / `patrol_stall` findings from `gt patrol scan`. Dead-rig detection groups orphaned agents under a single problem instead of emitting individual zombie alerts. Also shows `bd doctor` diagnostics with suggested fix commands.

**`views.Doctor`** — Scrollable `bd doctor --agent --json` overlay (`D`), listing error and warning diagnostics with their suggested fix commands. `R` re-runs the check.

**`views.CodexTranscript`** — Renders one in-app Codex (MCP) session as a running transcript: agent messages, exec commands, tool calls, searches, patches and errors, each with an icon, plus a meta line and a terminal status (done / errored / canceled) with duration and token usage.

**`components.Header`** — Parade group counts, progress bar, active agent count, Gas Town role badge, problem warning indicator, and the decorative bead string.

**`components.Footer`** — Context-sensitive keybinding hints and source file path with freshness indicator.

**`components.Palette`** — Command palette with fuzzy matching over available actions.

**`components.Toast`** — Timed notification in the divider row (4s auto-dismiss; 15s for critical warnings such as a known-bad `bd` version). The message is collapsed to one line and truncated to the terminal width by `fitToastLine`, so no caller can push the layout by being verbose.

**`components.CreateForm` / `components.EditForm`** — Multi-field issue creation (type, priority, title, description, and an optional crew field when an orchestrator is present) and a smaller edit form for title and priority.

**`components.ApprovalDialog` / `components.RecoveryDialog`** — Modal confirmations for Codex exec/apply-patch approvals and for dead-rig recovery (release-only vs release + re-sling).

## Data Flow: JSONL to Parade View

### 1. Bootstrap (cmd/mg/main.go)

```
Parse flags (--path, --block-types, --exclude-type, --exclude-label,
             --status, --version, --agent, --theme,
             --no-animations, --cmd-timeout)
    |
    v
Env-var equivalents fold into the same values:
  MG_BLOCK_TYPES, MG_AGENT_RUNTIME, MG_THEME,
  MG_NO_ANIMATIONS, MG_CMD_TIMEOUT
  (an explicit flag wins over the env var)
    |
    v
resolveSource(cwd, pathFlag):
  --path flag given?          --> SourceJSONL (explicit)
  .beads/ dir + bd on PATH?   --> SourceCLI (preferred)
  .beads/issues.jsonl found?  --> SourceJSONL (legacy fallback)
  none of the above           --> exit with error
    |
    v
Initial load based on source.Mode:
  SourceJSONL: data.LoadIssues(path)
  SourceCLI:   data.FetchIssuesCLI(projectDir)  (bd list --json --limit 0 --all)
    |
    v
--status mode?
  yes --> data.GroupByParade() --> tmux.StatusLine() --> print and exit
  no  --> app.New(issues, source, ...) --> tea.NewProgram(model).Run()
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

"Blocked" is determined by `EvaluateDependencies`: an issue is blocked if it has any dependency where the type is in `blockingTypes` (default: `"blocks"` and `"conditional-blocks"`) and the target issue is either missing or still open.

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

mg does not enforce a fixed vocabulary of dependency types — whatever string `bd` puts in `type` is carried through. The `--block-types` flag (or `MG_BLOCK_TYPES`) names the types treated as blockers; everything else is non-blocking. The default set is `data.DefaultBlockingTypes` = `blocks` and `conditional-blocks`. The detail pane has friendly wording for `related`, `duplicates`, `supersedes`, `parent-child`, `discovered-from`, `waits-for` and `replies-to`, and falls back to printing the raw type for anything else.

### 4. Live updates (data/watcher.go, source.go)

Two polling strategies, selected by `sourceMode`:

**JSONL mode** (`data.WatchFile`): polls file modtime every 1.2s, emits `FileChangedMsg` on change, `FileUnchangedMsg` when unchanged.

**CLI mode** (`data.PollCLI`): runs `bd list --json --limit 0 --all` every 5s, always emits `FileChangedMsg` (the app's `diffIssues()` detects no-ops). Errors emit `FileWatchErrorMsg` and show a toast.

Both use `startPoll()` and `startPollImmediate()` helpers so message handlers are mode-agnostic. After mutations (status change, issue create), `startPollImmediate()` triggers an instant re-fetch regardless of mode.

On `FileChangedMsg`, the app reloads issues, rebuilds parade groups, diffs against `prevIssueMap` to detect status changes (for change indicator badges), and syncs the selected issue — preserving cursor position and scroll state.

### 5. Filtering (data/filter.go)

`FilterIssues(issues, query)` tokenizes the query and ANDs all predicates:

- `type:bug` — match issue type
- `label:foo` — match one of the issue's labels (case-insensitive, exact)
- `priority:high` / `p0` — match priority level
- Free text — fuzzy (subsequence) match, ranked by score, against `issueSearchSource.String`: ID + Title + Description + Assignee + Owner + Notes + Labels joined into one haystack per issue

Structured tokens are ANDed. Free-text words are **not** — they are rejoined into a single fuzzy pattern before matching, so `deploy auth` and `auth deploy` are different queries. `FilterIssuesWithHighlights` is the variant the parade uses; it also returns the matched character offsets, projected back onto the title, for highlight rendering.

### 6. Focus mode (data/focus.go)

`FocusFilter(issues, blockingTypes)` returns the subset relevant to the current user: their in-progress work plus the top ready and blocked issues. Activated with `f`.

## Gas Town Integration

The `internal/gastown` package handles all orchestrator interaction. Core files (status, sling, convoy, mail, molecule, problems, recovery, detect) import nothing from `internal/data` — only stdlib, plus the generated `gcclient` in `gc_driver.go`. Analytics files (velocity, predict, scorecard, recommend) import `internal/data` for issue types.

### Driver seam (driver.go, gt_driver.go, gc.go, gc_driver.go)

The orchestrator is abstracted behind a `Driver` interface so the rest of the app never calls a specific backend directly. `app.Model` holds one `gastown.Driver`, chosen at startup by `SelectDriver()`:

- **`GTDriver`** (default) — wraps the existing `gt` CLI helpers 1:1; behavior is unchanged from before the seam existed.
- **`GCDriver`** — speaks the [Gas City](gascity.md) Supervisor HTTP API via the generated `gcclient` package. Operations with no Gas City mapping (and gt-only features like vitals, costs, patrol, recovery, handoff, the activity feed) return `ErrUnsupported`, which callers treat as "feature absent", not an error.

`SelectDriver()` chooses **by evidence about the machine**, not by `MG_GC_API` alone. In precedence order:

1. `MG_GC_API` set — the operator named Gas City explicitly.
2. Any Gas Town evidence (`GT_*` env vars, or `gt` on PATH) — Gas Town.
3. No Gas Town evidence at all, but Gas City evidence (`gc` on PATH, or a `city.toml` in an ancestor directory) — Gas City.
4. Nothing conclusive — Gas Town, the historical default.

Rule 3 is deliberately narrow: it fires only when there is no Gas Town evidence whatsoever, so anyone with `gt` installed or inside a Gas Town session keeps the backend they had. It exists because mg previously handed back a Gas Town driver that shelled out to a `gt` which was not installed. If constructing the Gas City driver fails, selection falls back to `GTDriver` rather than leaving the app driverless. `MG_GC_API=auto` (or any non-URL value) discovers the supervisor's dynamically-assigned port from `~/.gc/supervisor.log`; `MG_GC_CITY` optionally pins the city.

`app.Model.orchestratorAvailable()` is the gate the UI actually consults: `gtEnv.Available || driver.Backend() == "gascity"`.

Pure analytics, recovery helpers, local event-log reads, and the tmux handoff stay as free functions — they're driver-agnostic and not on the interface.

### Environment Detection (detect.go)

`Detect()` reads environment variables and checks PATH at startup:

```go
type Env struct {
    Available bool   // gt binary on PATH
    Active    bool   // running inside a Gas Town-managed session
    Role      string // GT_ROLE: mayor, polecat, crew, witness, refinery, deacon, dog
    Rig       string // GT_RIG: rig name
    Scope     string // GT_SCOPE: town or rig
    Worker    string // GT_POLECAT or GT_CREW: worker name

    GCAvailable bool   // gc binary on PATH (Gas City)
    GCCityPath  string // nearest ancestor directory containing city.toml, or ""
}
```

`Active` is true when either `GT_ROLE` or `GT_RIG` is set. The two `GC*` fields are the Gas City evidence `SelectDriver()` consults; mg does not otherwise drive `gc` from `Detect()`.

Features activate progressively: Beads-only (no orchestrator) → orchestrator available → inside a Gas Town session (GT_ROLE set).

### Status Polling (status.go)

`FetchStatus()` runs `gt status --json` and parses the result. A single-flight gate (`gtPollInFlight` in the app model) prevents overlapping polls. Key gotcha: the call is **slow and highly variable** — measured from seconds to tens of seconds depending on rig count, agent count, and whether backing services (dolt, the daemon) are running, which is why it sits in the 30s `timeoutLong` tier. Always render a nil `TownStatus` gracefully. The JSON nests agents under `rigs[].agents`; `normalizeStatus()` flattens them into a single `Agents` slice for the UI. Top-level agents are HQ-level (mayor, deacon); rig agents include polecats, crew, witness, refinery.

When a fetch fails, `app.Model` records the error in `townStatusErr` and hands it to the panel via `GasTown.SetStatusError`. `gasTownLoading()` is `showGasTown && townStatus == nil && townStatusErr == nil && orchestratorAvailable()` — the error term is what stops the panel from animating a loading spinner forever against a backend that is never going to answer.

`AgentRuntime` includes `Running`, `State`, `AgentInfo` (runtime/model), `AgentAlias` (short name), and `FirstSubject` (first unread mail subject). If `State` is empty, it defaults to "idle".

### Sling & Dispatch (sling.go)

These are the `gt` CLI wrappers `GTDriver` delegates to. Callers in `app` go through `Driver` (whose `Sling` takes a single `SlingRequest` collapsing all the variants below), not these directly.

- `Sling(issueID)` — dispatch to a polecat via `gt sling`
- `SlingWithAgent(issueID, agentName)` — sling with an `--agent` runtime override
- `SlingWithFormula(issueID, formula)` — sling with a specific formula
- `SlingMultiple(issueIDs)` / `SlingMultipleWithAgent` / `SlingMultipleWithFormula` — batch variants
- `Unsling(issueID)` — remove assignment
- `Nudge(target, message)` — send a nudge to an agent
- `HandoffInTmux(target, projectDir)` — hand a live session to the user in a tmux pane (local, so not on the `Driver` interface)
- `Decommission(address)` — decommission a polecat
- `CascadeClose(issueID)` — `gt close --cascade`

### Convoy Management (convoy.go)

- `ConvoyList()` — fetch all convoys
- `ConvoyStatus(convoyID)` — fetch one convoy's detail
- `ConvoyCreate(name, issueIDs)` — create from an issue selection
- `ConvoyCreateFromEpic(name, epicID)` — create from an epic's tree
- `ConvoyAdd(convoyID, issueIDs)` — add issues to an existing convoy
- `ConvoyLand(convoyID)` — land (close + cleanup)
- `ConvoyClose(convoyID)` — close without landing
- `ConvoyWatch(convoyID)` / `ConvoyUnwatch(convoyID)` — subscribe/unsubscribe

### Mail (mail.go)

- `MailInbox(unreadOnly)` — get messages
- `MailRead(messageID)` — fetch one message
- `MailReply(messageID, body)` — reply to a message
- `MailSend(address, subject, body)` — compose a new message
- `MailArchive(messageID)` — archive a message
- `MailMarkRead(messageID)` / `MailMarkAllRead()` — mark read

### DAG Rendering (dagrender.go)

`LayoutDAG(dag)` converts a `DAGInfo` (tiers of molecule steps) into `[]DAGRow` for visual rendering:

- `RowSingle` — one node per tier (linear chain)
- `RowParallel` — multiple nodes per tier (branching)
- `RowConnector` — flow connector line (`│`) between tiers

`CriticalPathSet()`, `CriticalPathTitles()`, and `CriticalPathString()` identify and render the critical path through the molecule using human-readable step titles.

### Analytics (costs.go, vitals.go, activity.go, velocity.go, scorecard.go, predict.go, recommend.go)

Each file handles one data domain:
- **costs.go** — Parse `gt costs` output for per-agent token/cost breakdown
- **vitals.go** — Parse `gt vitals` text output for Dolt server health and backup freshness
- **activity.go** — Parse event streams for the activity feed
- **velocity.go** — Compute issue flow rates and agent utilization
- **scorecard.go** — Aggregate quality scores per agent
- **predict.go** — Convoy ETA estimation from historical throughput
- **recommend.go** — Formula recommendation based on issue characteristics

## Key Domain Types

```
Issue
  ID, Title, Description, Status, Priority, IssueType
  Owner, Assignee, CreatedAt, CreatedBy, UpdatedAt, StartedAt, ClosedAt
  Dependencies []Dependency
  Notes, Design, AcceptanceCriteria, CloseReason
  Labels []string, DueAt, DeferUntil, Metadata map[string]any
  CommentCount  (from bd list --json; the detail panel counts its own
                 fetched comments instead)

  Note: comments are NOT a field on Issue. They are fetched separately
  (Driver.Comments -> gastown.Comment) and held on views.Detail.

Status:        open | in_progress | closed
IssueType:     task | bug | feature | chore | epic | spike | story | milestone
Priority:      0 (critical) .. 4 (backlog)
ParadeStatus:  Rolling | LinedUp | Stalled | PastTheStand

Dependency
  IssueID      (source -- the issue that has this dep)
  DependsOnID  (target -- the issue being depended on)
  Type         (free-form; blockers are whichever types --block-types names,
                default "blocks" and "conditional-blocks")
  CreatedAt, CreatedBy

DepEval        (computed from EvaluateDependencies)
  Edges []DepEdge
  BlockingIDs, ResolvedIDs, MissingIDs, NonBlocking
  IsBlocked, NextBlockerID

AgentRuntime   (from gastown/status.go)
  Name, Address, Role, Rig, Running, State
  HasWork, WorkTitle, HookBead, Mail (unread count)
  AgentInfo, AgentAlias, FirstSubject

TownStatus     (from gastown/status.go)
  Agents []AgentRuntime  (flattened from all rigs)
  Rigs   []RigStatus

ConvoyDetail   (from gastown/convoy.go)
  ID, Title, Status, Completed, Total, ProgressPct
  ReadyCount, ActiveCount, Assignees
  Tracked []TrackedIssue (expanded issue details)

MailMessage    (from gastown/mail.go)
  ID, From, Subject, Body, Time, Read, Priority, Type

OrphanedIssue  (from gastown/recovery.go)
  IssueID, Title, AgentName, AgentRole

Problem        (from gastown/problems.go)
  Type, Agent, Detail, Severity, Category, Fix
  RigName, Orphans (for dead_rig problems)
  Types: stalled, stuck, backoff, zombie, dead_rig, doctor, patrol_zombie, patrol_stall

PatrolScanResult (from gastown/patrol.go)
  Rig, Timestamp, Zombies, Stalls, Completions (each: Checked, Found)
  Details []PatrolDetail (Agent, Rig, Role, HookBead, Detail)
```

## Agent Integration

Pressing `a` on a selected issue launches an agent with a context-rich prompt (title, metadata, description, notes, acceptance criteria, and the evaluated dependency list, plus `bd update` / `bd close` lifecycle hints). Behavior depends on environment:

- **With an orchestrator**: dispatches through `Driver.Sling`. On Gas Town that is `gt sling`; on Gas City mg first prompts for a target agent, because that API is addressed by target rather than auto-picking a polecat.
- **In tmux (no orchestrator)**: opens a new tmux **pane** (`split-window -h -l 60% -d`) tagged with the pane option `@mg_agent=mg-<issueID>` for discovery, focus, capture and kill. Pressing `a` again on an issue that already has a pane switches to it.
- **Outside tmux**: suspends the TUI via `tea.ExecProcess`, resumes on exit.

`A` stops the agent: with an orchestrator it calls `Driver.Unsling` (a backend that cannot returns `ErrUnsupported`, which is a truthful message); only with no orchestrator at all does it kill the tmux pane directly.

`agent.DetectRuntime()` picks the runtime at startup: `MG_AGENT_RUNTIME` / `--agent` wins if the named binary is on PATH, otherwise the order is `claude` → `cursor-agent` → `codex`. Each gets its own launch flags (`claude --teammate-mode tmux`, `cursor-agent -f -p`, `codex --sandbox workspace-write -a on-request -C <dir>`, plus `--no-alt-screen` in tmux). When Codex is the runtime, mg propagates `--agent codex` into `gt sling`. The app polls for agent state: tmux panes (when in tmux) or orchestrator status (when available). Status badges appear in the header, parade list, and detail view, and the detail pane tails the agent's pane via `agent.CapturePane(id, 15)`.

`M` is a separate dispatch path entirely: it runs a Codex session **inside** mg over MCP (`internal/codexmcp` + `agent.LaunchCodexMCP`), streaming events into `views.CodexTranscript` and routing exec/apply-patch approvals to a modal. It uses approval policy `on-request` because a human is watching; the tmux and orchestrator paths use `never`.

Additional agent operations from the Gas Town panel:
- `n` — nudge agent with a message
- `h` — handoff work from an agent to another
- `K` — decommission a polecat

## UI Architecture

### Theme System (ui/)

All visual constants live in `internal/ui/`:

- **theme.go** — Color palette (Mardi Gras purple, gold, green), plus `RoleColor()` for all 7 Gas Town agent roles (mayor/coordinator, deacon/health-check, polecat, crew, witness, refinery, dog) and `AgentStateColor()` for working/idle/spawning/backoff-degraded/stuck/awaiting-gate/fix_needed/propelled/patrolling/paused-muted states. The palette is **switchable**: `ui.SetTheme(ThemeDark|ThemeLight)` re-bakes it at startup from `--theme` / `MG_THEME` (`auto` sniffs the terminal background). Because of that, no package outside `internal/ui` may capture palette vars, styles, or pre-rendered strings in package-level variables — they would freeze the dark values before the theme is applied
- **styles.go** — Pre-built lipgloss styles for every context: parade items, detail sections, Gas Town panel, DAG connectors, toast notifications, command palette
- **symbols.go** — Unicode symbols: status indicators (●, ♪, ⊘, ✓), dependency arrows, DAG flow connectors (│, ┌, ├, └), progress bars
Convention: views and components import `ui` for all visual constants. No raw colors or symbols in view code.

### Receiver Conventions

- **Value receivers** on BubbleTea models (`Update`, `View`) — required by the Elm architecture
- **Pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`) — internal state updates

## External Dependencies

The TUI core is the [Charmbracelet](https://charm.sh/) toolkit, on the v2 `charm.land/*` module paths:

| Package | Purpose |
|---|---|
| `charm.land/bubbletea/v2` | Elm-architecture TUI framework |
| `charm.land/bubbles/v2` | Reusable components (viewport, textinput, spinner) |
| `charm.land/lipgloss/v2` | Terminal styling and layout |
| `charmbracelet/x/ansi` | ANSI string width, stripping and truncation |
| `charmbracelet/glamour` + `yuin/goldmark` | Markdown rendering in the detail pane |
| `charmbracelet/ultraviolet` | Terminal primitives used by bubbletea v2 |
| `muesli/termenv`, `lucasb-eyer/go-colorful` | Color profile detection and gradient/sparkline math |

Plus:
| Package | Purpose |
|---|---|
| `atotto/clipboard` | Cross-platform clipboard access (branch name copy) |
| `sahilm/fuzzy` | Fuzzy matching for the filter bar and the command palette |
| `oapi-codegen/runtime` | Runtime support for the generated Gas City `gcclient` |
| `gopkg.in/yaml.v3` | Beads config / metadata schema parsing |

## Data Source Abstraction

`data/source.go` holds the small config value that tells the app where issues come from. It is worth being precise about what this is and is not: `SourceMode` is an **enum on a plain config struct**, not a pluggable interface. There is no `Source` interface and no registry — the modes are switched on explicitly in `startPoll()`, `startPollImmediate()` and `resolveSource()`. What the shape buys is that every mode emits the same messages, so the app layer stays mode-agnostic; adding a mode still means touching each of those switches.

```go
type SourceMode int

const (
    SourceJSONL SourceMode = iota  // Read from .beads/issues.jsonl
    SourceCLI                       // Shell out to bd list --json
)

type Source struct {
    Mode       SourceMode
    Path       string  // JSONL file path (SourceJSONL) or empty (SourceCLI)
    ProjectDir string  // Project root directory
    Explicit   bool    // True if --path was used
}
```

`Source.Label()` returns a display string for the footer: `"bd list"` in CLI mode, otherwise the base name of the JSONL path.

### Source health and JSONL fallback (source_health.go)

`SourceHealth` is a small immutable state machine — value receivers returning a new value, so it composes with BubbleTea's model pattern — that tracks consecutive `bd list` failures: `Healthy → Degraded → Fallback → Recovering → Healthy`. While `InFallback()` is true, `startPoll()` watches the JSONL file even in CLI mode, so a dead Dolt server degrades to stale-but-readable data instead of an empty parade. A separate 15s `CLIHealthCheck` tick probes whether the CLI has come back, and the footer renders the degraded state.

### Adding a new source mode

To add a new mode (e.g., `SourceDolt` for direct Dolt MySQL connection):

1. Add constant to `SourceMode` in `data/source.go`
2. Add fetch function returning `([]Issue, error)` in `data/source.go`
3. Add poll function returning `tea.Cmd` in `data/watcher.go`
4. Extend `startPoll()` / `startPollImmediate()` in `internal/app/app.go`
5. Add case to `resolveSource()` in `cmd/mg/main.go`
6. Update `Source.Label()` for footer display

All modes emit the same `FileChangedMsg` / `FileWatchErrorMsg`, so the app layer is mode-agnostic.

## Architectural Frontier

### Phase 2: Direct Dolt Connection (`SourceDolt`)

Direct MySQL connection to the Dolt SQL server for sub-second polling, incremental diffs, richer queries (e.g., closed-since, changed-fields), and eliminating the `bd` CLI as a runtime dependency. The extension point already exists via `SourceMode` — add `SourceDolt`, a fetch function using `database/sql`, and a poll Cmd.

### More Agent Runtimes

`agent/launch.go` already detects and launches three runtimes (Claude Code, Cursor, Codex), and `--agent codex` propagates into `gt sling`. What remains is the rest of Gas Town's runtime list (Gemini CLI, Copilot CLI, OpenCode): reading the runtime back out of `AgentRuntime` metadata, offering per-sling runtime selection in the formula picker, and per-runtime prompt formatting.

### Orchestrator Status Latency

`gt status --json` is slow and its latency is highly variable — seconds to tens of seconds depending on rig and agent count and whether dolt/the daemon are up. Current mitigation: background polling behind a single-flight gate, nil-safe rendering, an on-demand fetch when the panel opens, and an explicit error state so a dead backend stops the spinner instead of hanging on it. Future options: an incremental status API, local caching with a TTL, or the Gas City SSE stream (`FeatureSSE`, declared but not yet wired up).
