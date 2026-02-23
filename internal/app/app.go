package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
	"github.com/matt-wright86/mardi-gras/internal/views"
)

// Pane tracks which panel is focused.
type Pane int

const (
	PaneParade Pane = iota
	PaneDetail
)

const (
	toastDuration           = 4 * time.Second
	changeIndicatorDuration = 30 * time.Second
)

// Model is the root BubbleTea model.
type Model struct {
	issues        []data.Issue
	groups        map[data.ParadeStatus][]data.Issue
	parade        views.Parade
	detail        views.Detail
	header        components.Header
	activPane     Pane
	width         int
	height        int
	watchPath     string
	pathExplicit  bool
	lastFileMod   time.Time
	blockingTypes map[string]bool
	filterInput   textinput.Model
	filtering     bool
	showHelp      bool
	ready         bool
	claudeAvail   bool
	projectDir    string
	inTmux        bool
	activeAgents  map[string]string   // issueID -> tmux window name
	gtEnv         gastown.Env         // Gas Town environment, read once at startup
	townStatus    *gastown.TownStatus // Latest gt status, nil when unavailable
	gasTown       views.GasTown       // Gas Town control surface panel
	showGasTown   bool                // Whether the Gas Town panel replaces detail

	// Toast notification
	toast components.Toast

	// Confetti animation
	confetti Confetti

	// Change indicators: track recently changed issue IDs
	changedIDs   map[string]bool
	changedAt    time.Time
	prevIssueMap map[string]data.Status // issueID -> previous status for diffing

	// Focus mode
	focusMode bool

	// Issue creation form
	creating   bool
	createForm components.CreateForm

	// Command palette
	showPalette bool
	palette     components.Palette

	// Formula picker state
	formulaPicking bool
	formulaTarget  string
	formulaMulti   []string

	// Nudge input state
	nudging     bool
	nudgeInput  textinput.Model
	nudgeTarget string
}

// New creates a new app model from loaded issues.
func New(issues []data.Issue, watchPath string, pathExplicit bool, blockingTypes map[string]bool) Model {
	groups := data.GroupByParade(issues, blockingTypes)
	lastFileMod := time.Time{}
	if watchPath != "" {
		if mod, err := data.FileModTime(watchPath); err == nil {
			lastFileMod = mod
		}
	}
	ti := textinput.New()
	ti.Prompt = ui.InputPrompt.Render("/ ")
	ti.Placeholder = "Filter type:bug, p1, or fuzzy text..."
	ti.TextStyle = ui.InputText
	ti.Cursor.Style = ui.InputCursor
	ti.Width = 50

	// Derive project root by stripping .beads/issues.jsonl from the watch path.
	projectDir := ""
	if watchPath != "" {
		projectDir = filepath.Dir(filepath.Dir(watchPath))
	}

	// Build initial status snapshot for change detection
	prevMap := make(map[string]data.Status, len(issues))
	for _, iss := range issues {
		prevMap[iss.ID] = iss.Status
	}

	return Model{
		issues:        issues,
		groups:        groups,
		activPane:     PaneParade,
		watchPath:     watchPath,
		pathExplicit:  pathExplicit,
		lastFileMod:   lastFileMod,
		blockingTypes: blockingTypes,
		filterInput:   ti,
		claudeAvail:   agent.Available(),
		projectDir:    projectDir,
		inTmux:        agent.InTmux() && agent.TmuxAvailable(),
		activeAgents:  make(map[string]string),
		gtEnv:         gastown.Detect(),
		changedIDs:    make(map[string]bool),
		prevIssueMap:  prevMap,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		data.WatchFile(m.watchPath, m.lastFileMod),
		pollAgentState(m.gtEnv, m.inTmux),
	)
}

// agentFinishedMsg is sent when a launched claude session exits.
type agentFinishedMsg struct{ err error }

type agentLaunchedMsg struct {
	issueID    string
	windowName string
}

type agentLaunchErrorMsg struct {
	issueID string
	err     error
}

type agentStatusMsg struct {
	activeAgents map[string]string
}

type townStatusMsg struct {
	status *gastown.TownStatus
	err    error
}

type slingResultMsg struct {
	issueID string
	formula string
	err     error
}

type formulaListMsg struct {
	formulas []string
	err      error
}

type unslingResultMsg struct {
	issueID string
	err     error
}

type multiSlingResultMsg struct {
	count   int
	formula string
	err     error
}

type nudgeResultMsg struct {
	target  string
	message string
	err     error
}

type handoffResultMsg struct {
	target string
	err    error
}

type decommissionResultMsg struct {
	address string
	err     error
}

// mutateResultMsg is sent when a bd CLI mutation completes.
type mutateResultMsg struct {
	issueID string
	action  string
	err     error
}

// changeIndicatorExpiredMsg clears change indicators after timeout.
type changeIndicatorExpiredMsg struct{}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle create form result
	if result, ok := msg.(components.CreateFormResult); ok {
		m.creating = false
		if result.Cancelled || result.Title == "" {
			return m, nil
		}
		title := result.Title
		issueType := data.IssueType(result.Type)
		priority := components.ParsePriority(result.Priority)
		return m, func() tea.Msg {
			_, err := data.CreateIssue(title, issueType, priority)
			return mutateResultMsg{issueID: title, action: "created", err: err}
		}
	}

	// Handle palette result
	if result, ok := msg.(components.PaletteResult); ok {
		m.showPalette = false
		if m.formulaPicking {
			m.formulaPicking = false
			if result.Cancelled {
				m.formulaTarget = ""
				m.formulaMulti = nil
				return m, nil
			}
			formula := m.palette.SelectedName()
			if m.formulaMulti != nil {
				ids := m.formulaMulti
				m.formulaMulti = nil
				m.formulaTarget = ""
				return m, func() tea.Msg {
					err := gastown.SlingMultipleWithFormula(ids, formula)
					return multiSlingResultMsg{count: len(ids), formula: formula, err: err}
				}
			}
			issueID := m.formulaTarget
			m.formulaTarget = ""
			return m, func() tea.Msg {
				err := gastown.SlingWithFormula(issueID, formula)
				return slingResultMsg{issueID: issueID, formula: formula, err: err}
			}
		}
		if !result.Cancelled {
			return m.executePaletteAction(result.Action)
		}
		return m, nil
	}

	// Forward all messages to palette when active
	if m.showPalette {
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "ctrl+c" {
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd
	}

	// Forward all messages to create form when active
	if m.creating {
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "ctrl+c" {
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.createForm, cmd = m.createForm.Update(msg)
		return m, cmd
	}

	// Forward all messages to nudge input when active
	if m.nudging {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.nudging = false
				return m, nil
			case "enter":
				m.nudging = false
				target := m.nudgeTarget
				message := m.nudgeInput.Value()
				return m, func() tea.Msg {
					err := gastown.Nudge(target, message)
					return nudgeResultMsg{target: target, message: message, err: err}
				}
			}
		}
		var cmd tea.Cmd
		m.nudgeInput, cmd = m.nudgeInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
			return m.handleHelpKey(msg)
		}
		if m.filtering {
			return m.handleFilteringKey(msg)
		}
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		m.ready = true
		return m, nil

	case data.FileChangedMsg:
		cmds := []tea.Cmd{
			data.WatchFile(m.watchPath, m.lastFileMod),
			pollAgentState(m.gtEnv, m.inTmux),
		}

		// Diff against previous state for change indicators
		changes := m.diffIssues(msg.Issues)
		if changes > 0 {
			m.changedAt = time.Now()
			toast, toastCmd := components.ShowToast(
				fmt.Sprintf("File reloaded \u2014 %d issue%s changed", changes, plural(changes)),
				components.ToastInfo, toastDuration,
			)
			m.toast = toast
			cmds = append(cmds, toastCmd)
			cmds = append(cmds, tea.Tick(changeIndicatorDuration, func(time.Time) tea.Msg {
				return changeIndicatorExpiredMsg{}
			}))
		}

		// Update snapshot for next diff
		m.prevIssueMap = make(map[string]data.Status, len(msg.Issues))
		for _, iss := range msg.Issues {
			m.prevIssueMap[iss.ID] = iss.Status
		}

		m.issues = msg.Issues
		m.groups = data.GroupByParade(msg.Issues, m.blockingTypes)
		if !msg.LastMod.IsZero() {
			m.lastFileMod = msg.LastMod
		}
		m.rebuildParade()
		return m, tea.Batch(cmds...)

	case data.FileUnchangedMsg:
		if !msg.LastMod.IsZero() {
			m.lastFileMod = msg.LastMod
		}
		return m, tea.Batch(data.WatchFile(m.watchPath, m.lastFileMod), pollAgentState(m.gtEnv, m.inTmux))

	case data.FileWatchErrorMsg:
		return m, tea.Batch(data.WatchFile(m.watchPath, m.lastFileMod), pollAgentState(m.gtEnv, m.inTmux))

	case agentLaunchedMsg:
		m.activeAgents[msg.issueID] = msg.windowName
		m.propagateAgentState()
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Agent launched for %s", msg.issueID),
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		return m, cmd

	case agentLaunchErrorMsg:
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Agent launch failed: %s", msg.err),
			components.ToastError, toastDuration,
		)
		m.toast = toast
		return m, cmd

	case agentStatusMsg:
		m.activeAgents = msg.activeAgents
		m.propagateAgentState()
		return m, nil

	case townStatusMsg:
		if msg.err == nil && msg.status != nil {
			m.townStatus = msg.status
			m.activeAgents = msg.status.ActiveAgentMap()
			m.propagateAgentState()
			if m.showGasTown {
				m.gasTown.SetStatus(m.townStatus, m.gtEnv)
			}
		}
		return m, nil

	case slingResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Sling failed for %s: %s", msg.issueID, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		label := fmt.Sprintf("Slung %s to polecat", msg.issueID)
		if msg.formula != "" {
			label = fmt.Sprintf("Slung %s with %s formula", msg.issueID, msg.formula)
		}
		toast, cmd := components.ShowToast(label, components.ToastSuccess, toastDuration)
		m.toast = toast
		return m, tea.Batch(cmd, pollAgentState(m.gtEnv, m.inTmux))

	case formulaListMsg:
		if msg.err != nil || len(msg.formulas) == 0 {
			var slingCmd tea.Cmd
			if m.formulaMulti != nil {
				ids := m.formulaMulti
				slingCmd = func() tea.Msg {
					err := gastown.SlingMultiple(ids)
					return multiSlingResultMsg{count: len(ids), err: err}
				}
			} else {
				issueID := m.formulaTarget
				slingCmd = func() tea.Msg {
					err := gastown.Sling(issueID)
					return slingResultMsg{issueID: issueID, err: err}
				}
			}
			m.formulaPicking = false
			m.formulaTarget = ""
			m.formulaMulti = nil
			toast, toastCmd := components.ShowToast(
				"No formulas available \u2014 using plain sling",
				components.ToastInfo, toastDuration,
			)
			m.toast = toast
			return m, tea.Batch(toastCmd, slingCmd)
		}
		cmds := make([]components.PaletteCommand, len(msg.formulas))
		for i, f := range msg.formulas {
			cmds[i] = components.PaletteCommand{
				Name:   f,
				Desc:   "Formula",
				Action: components.ActionFormulaSelect,
			}
		}
		m.formulaPicking = true
		m.showPalette = true
		m.palette = components.NewPalette(m.width, m.height, cmds)
		return m, m.palette.Init()

	case unslingResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Unsling failed for %s: %s", msg.issueID, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Unslung %s", msg.issueID),
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		return m, tea.Batch(cmd, pollAgentState(m.gtEnv, m.inTmux))

	case multiSlingResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Multi-sling failed: %s", msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		label := fmt.Sprintf("Slung %d issues", msg.count)
		if msg.formula != "" {
			label = fmt.Sprintf("Slung %d issues with %s formula", msg.count, msg.formula)
		}
		toast, cmd := components.ShowToast(label, components.ToastSuccess, toastDuration)
		m.toast = toast
		return m, tea.Batch(cmd, pollAgentState(m.gtEnv, m.inTmux))

	case nudgeResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Nudge failed for %s: %s", msg.target, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		label := fmt.Sprintf("Nudged %s", msg.target)
		if msg.message != "" {
			display := msg.message
			if len(display) > 30 {
				display = display[:27] + "..."
			}
			label = fmt.Sprintf("Nudged %s: %s", msg.target, display)
		}
		toast, cmd := components.ShowToast(label, components.ToastSuccess, toastDuration)
		m.toast = toast
		return m, cmd

	case handoffResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Handoff failed for %s: %s", msg.target, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Handoff initiated for %s", msg.target),
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		return m, tea.Batch(cmd, pollAgentState(m.gtEnv, m.inTmux))

	case decommissionResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Decommission failed for %s: %s", msg.address, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Decommissioned %s", msg.address),
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		return m, tea.Batch(cmd, pollAgentState(m.gtEnv, m.inTmux))

	case views.GasTownActionMsg:
		return m.handleGasTownAction(msg)

	case mutateResultMsg:
		if msg.err != nil {
			toast, cmd := components.ShowToast(
				fmt.Sprintf("Failed: %s %s \u2014 %s", msg.action, msg.issueID, msg.err),
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		toast, toastCmd := components.ShowToast(
			fmt.Sprintf("%s \u2192 %s", msg.issueID, msg.action),
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		// Force reload on next poll
		m.lastFileMod = time.Time{}
		cmds := []tea.Cmd{toastCmd, data.WatchFile(m.watchPath, m.lastFileMod)}
		// Trigger confetti on close
		if msg.action == "closed" && m.width > 0 && m.height > 0 {
			m.confetti = NewConfetti(m.width, m.height)
			cmds = append(cmds, m.confetti.Tick())
		}
		return m, tea.Batch(cmds...)

	case confettiTickMsg:
		m.confetti.Update()
		if m.confetti.Active() {
			return m, m.confetti.Tick()
		}
		return m, nil

	case components.ToastDismissMsg:
		m.toast = components.Toast{}
		return m, nil

	case changeIndicatorExpiredMsg:
		m.changedIDs = make(map[string]bool)
		m.parade.ChangedIDs = nil
		return m, nil

	case agentFinishedMsg:
		// Reset lastFileMod to force reload on next poll cycle.
		m.lastFileMod = time.Time{}
		return m, tea.Batch(data.WatchFile(m.watchPath, m.lastFileMod), pollAgentState(m.gtEnv, m.inTmux))
	}

	// Forward to detail viewport (or Gas Town viewport) when focused
	if m.activPane == PaneDetail {
		if m.showGasTown {
			var cmd tea.Cmd
			m.gasTown, cmd = m.gasTown.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.detail.Viewport, cmd = m.detail.Viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		m.showHelp = false
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleFilteringKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "esc":
		m.filtering = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.rebuildParade()
		return m, nil
	case "enter":
		m.filtering = false
		m.filterInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	oldVal := m.filterInput.Value()
	m.filterInput, cmd = m.filterInput.Update(msg)
	if m.filterInput.Value() != oldVal {
		m.rebuildParade()
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When Gas Town panel is focused, route its keys before global handlers
	if m.showGasTown && m.activPane == PaneDetail {
		switch msg.String() {
		case "j", "k", "up", "down", "g", "G", "n", "h", "K":
			var cmd tea.Cmd
			m.gasTown, cmd = m.gasTown.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "tab":
		if m.activPane == PaneParade {
			m.activPane = PaneDetail
			m.detail.Focused = true
		} else {
			m.activPane = PaneParade
			m.detail.Focused = false
		}
		return m, nil

	case "esc":
		if m.focusMode {
			m.focusMode = false
			m.rebuildParade()
			return m, nil
		}
		if m.activPane == PaneDetail {
			m.activPane = PaneParade
			m.detail.Focused = false
		}
		return m, nil

	case "f":
		m.focusMode = !m.focusMode
		m.rebuildParade()
		if m.focusMode {
			toast, cmd := components.ShowToast("Focus mode ON", components.ToastInfo, toastDuration)
			m.toast = toast
			return m, cmd
		}
		toast, cmd := components.ShowToast("Focus mode OFF", components.ToastInfo, toastDuration)
		m.toast = toast
		return m, cmd

	case "ctrl+g":
		if !m.gtEnv.Available {
			return m, nil
		}
		m.showGasTown = !m.showGasTown
		if m.showGasTown {
			m.gasTown.SetStatus(m.townStatus, m.gtEnv)
			// If status hasn't arrived yet (slow gt), trigger a fetch
			if m.townStatus == nil {
				return m, pollAgentState(m.gtEnv, m.inTmux)
			}
		}
		return m, nil

	case "c":
		m.parade.ToggleClosed()
		m.syncSelection()
		return m, nil

	// Quick actions: status changes (6.1)
	case "1":
		return m.quickAction(data.StatusInProgress, "in_progress")
	case "2":
		return m.quickAction(data.StatusOpen, "open")
	case "3":
		return m.closeSelectedIssue()

	// Quick actions: priority changes (6.1)
	case "!": // Shift+1
		return m.setPriority(data.PriorityHigh)
	case "@": // Shift+2
		return m.setPriority(data.PriorityMedium)
	case "#": // Shift+3
		return m.setPriority(data.PriorityLow)
	case "$": // Shift+4
		return m.setPriority(data.PriorityBacklog)

	// Git branch name copy (5.5)
	case "b":
		return m.copyBranchName()
	case "B":
		return m.createAndSwitchBranch()

	case "a":
		// Multi-sling with Gas Town
		if selected := m.parade.SelectedIssues(); len(selected) > 0 && m.gtEnv.Available {
			ids := make([]string, len(selected))
			for i, iss := range selected {
				ids[i] = iss.ID
			}
			m.parade.ClearSelection()
			return m, func() tea.Msg {
				err := gastown.SlingMultiple(ids)
				return multiSlingResultMsg{count: len(ids), err: err}
			}
		}

		issue := m.parade.SelectedIssue
		if issue == nil || !m.claudeAvail {
			return m, nil
		}
		if _, active := m.activeAgents[issue.ID]; active && m.inTmux {
			_ = agent.SelectAgentWindow(issue.ID)
			return m, nil
		}

		if m.gtEnv.Available {
			issueID := issue.ID
			return m, func() tea.Msg {
				err := gastown.Sling(issueID)
				return slingResultMsg{issueID: issueID, err: err}
			}
		}

		deps := issue.EvaluateDependencies(m.detail.IssueMap, m.blockingTypes)
		prompt := agent.BuildPrompt(*issue, deps, m.detail.IssueMap)

		if m.inTmux {
			issueID := issue.ID
			return m, func() tea.Msg {
				winName, err := agent.LaunchInTmux(prompt, m.projectDir, issueID)
				if err != nil {
					return agentLaunchErrorMsg{issueID: issueID, err: err}
				}
				return agentLaunchedMsg{issueID: issueID, windowName: winName}
			}
		}
		c := agent.Command(prompt, m.projectDir)
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return agentFinishedMsg{err: err}
		})

	case "A":
		issue := m.parade.SelectedIssue
		if issue == nil {
			return m, nil
		}
		if _, active := m.activeAgents[issue.ID]; !active {
			return m, nil
		}
		issueID := issue.ID
		if m.gtEnv.Available {
			return m, func() tea.Msg {
				err := gastown.Unsling(issueID)
				return unslingResultMsg{issueID: issueID, err: err}
			}
		}
		if m.inTmux {
			return m, func() tea.Msg {
				_ = agent.KillAgentWindow(issueID)
				return agentStatusMsg{activeAgents: make(map[string]string)}
			}
		}
		return m, nil

	case "s":
		if !m.gtEnv.Available {
			return m, nil
		}
		// Multi-select: collect IDs for formula picking
		if selected := m.parade.SelectedIssues(); len(selected) > 0 {
			ids := make([]string, len(selected))
			for i, iss := range selected {
				ids[i] = iss.ID
			}
			m.parade.ClearSelection()
			m.formulaMulti = ids
			m.formulaTarget = ""
			return m, func() tea.Msg {
				formulas, err := gastown.ListFormulas()
				return formulaListMsg{formulas: formulas, err: err}
			}
		}
		// Single issue
		issue := m.parade.SelectedIssue
		if issue == nil {
			return m, nil
		}
		m.formulaTarget = issue.ID
		m.formulaMulti = nil
		return m, func() tea.Msg {
			formulas, err := gastown.ListFormulas()
			return formulaListMsg{formulas: formulas, err: err}
		}

	case "n":
		issue := m.parade.SelectedIssue
		if issue == nil || !m.gtEnv.Available {
			return m, nil
		}
		agentName, active := m.activeAgents[issue.ID]
		if !active {
			return m, nil
		}
		m.nudging = true
		m.nudgeTarget = agentName
		m.nudgeInput = textinput.New()
		m.nudgeInput.Prompt = ui.InputPrompt.Render("nudge> ")
		m.nudgeInput.Placeholder = "Message for " + agentName + "..."
		m.nudgeInput.TextStyle = ui.InputText
		m.nudgeInput.Cursor.Style = ui.InputCursor
		m.nudgeInput.Width = 50
		m.nudgeInput.Focus()
		return m, textinput.Blink

	case "N":
		m.creating = true
		m.createForm = components.NewCreateForm(m.width, m.height)
		return m, m.createForm.Init()

	case ":", "ctrl+k":
		m.showPalette = true
		m.palette = components.NewPalette(m.width, m.height, m.buildPaletteCommands())
		return m, m.palette.Init()
	}

	// Navigation keys depend on active pane
	if m.activPane == PaneParade {
		switch msg.String() {
		case "j", "down":
			m.parade.MoveDown()
			m.syncSelection()
		case "k", "up":
			m.parade.MoveUp()
			m.syncSelection()
		case "J": // Shift+J: select + move down
			m.parade.ToggleSelect()
			m.parade.MoveDown()
			m.syncSelection()
		case "K": // Shift+K: select + move up
			m.parade.ToggleSelect()
			m.parade.MoveUp()
			m.syncSelection()
		case " ", "x": // Toggle multi-select
			m.parade.ToggleSelect()
		case "X": // Clear all selections
			m.parade.ClearSelection()
		case "g":
			m.parade.Cursor = 0
			m.parade.ScrollOffset = 0
			for i, item := range m.parade.Items {
				if !item.IsHeader {
					m.parade.Cursor = i
					break
				}
			}
			m.syncSelection()
		case "G":
			for i := len(m.parade.Items) - 1; i >= 0; i-- {
				if !m.parade.Items[i].IsHeader {
					m.parade.Cursor = i
					break
				}
			}
			m.syncSelection()
		case "enter":
			m.activPane = PaneDetail
			m.detail.Focused = true
		}
		return m, nil
	}

	// Detail pane navigation (or Gas Town panel when active)
	if m.activPane == PaneDetail {
		if m.showGasTown {
			var cmd tea.Cmd
			m.gasTown, cmd = m.gasTown.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		switch msg.String() {
		case "j", "down":
			m.detail.Viewport.ScrollDown(1)
		case "k", "up":
			m.detail.Viewport.ScrollUp(1)
		default:
			m.detail.Viewport, cmd = m.detail.Viewport.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

// quickAction runs bd update to change issue status. Works on multi-selection if active.
func (m Model) quickAction(status data.Status, label string) (tea.Model, tea.Cmd) {
	// Bulk mode: apply to all selected issues
	if selected := m.parade.SelectedIssues(); len(selected) > 0 {
		issues := selected
		count := len(issues)
		m.parade.ClearSelection()
		return m, func() tea.Msg {
			var lastErr error
			for _, iss := range issues {
				if iss.Status != status {
					if err := data.SetStatus(iss.ID, status); err != nil {
						lastErr = err
					}
				}
			}
			return mutateResultMsg{
				issueID: fmt.Sprintf("%d issues", count),
				action:  label,
				err:     lastErr,
			}
		}
	}

	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	if issue.Status == status {
		return m, nil
	}
	issueID := issue.ID
	return m, func() tea.Msg {
		err := data.SetStatus(issueID, status)
		return mutateResultMsg{issueID: issueID, action: label, err: err}
	}
}

// closeSelectedIssue runs bd close on the selected issue(s).
func (m Model) closeSelectedIssue() (tea.Model, tea.Cmd) {
	// Bulk mode
	if selected := m.parade.SelectedIssues(); len(selected) > 0 {
		issues := selected
		count := len(issues)
		m.parade.ClearSelection()
		return m, func() tea.Msg {
			var lastErr error
			for _, iss := range issues {
				if iss.Status != data.StatusClosed {
					if err := data.CloseIssue(iss.ID); err != nil {
						lastErr = err
					}
				}
			}
			return mutateResultMsg{
				issueID: fmt.Sprintf("%d issues", count),
				action:  "closed",
				err:     lastErr,
			}
		}
	}

	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	if issue.Status == data.StatusClosed {
		return m, nil
	}
	issueID := issue.ID
	return m, func() tea.Msg {
		err := data.CloseIssue(issueID)
		return mutateResultMsg{issueID: issueID, action: "closed", err: err}
	}
}

// setPriority runs bd update to change issue priority. Works on multi-selection if active.
func (m Model) setPriority(priority data.Priority) (tea.Model, tea.Cmd) {
	// Bulk mode
	if selected := m.parade.SelectedIssues(); len(selected) > 0 {
		issues := selected
		count := len(issues)
		label := fmt.Sprintf("P%d", priority)
		m.parade.ClearSelection()
		return m, func() tea.Msg {
			var lastErr error
			for _, iss := range issues {
				if iss.Priority != priority {
					if err := data.SetPriority(iss.ID, priority); err != nil {
						lastErr = err
					}
				}
			}
			return mutateResultMsg{
				issueID: fmt.Sprintf("%d issues", count),
				action:  label,
				err:     lastErr,
			}
		}
	}

	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	if issue.Priority == priority {
		return m, nil
	}
	issueID := issue.ID
	label := fmt.Sprintf("P%d", priority)
	return m, func() tea.Msg {
		err := data.SetPriority(issueID, priority)
		return mutateResultMsg{issueID: issueID, action: label, err: err}
	}
}

// copyBranchName copies a slugified branch name to the clipboard.
func (m Model) copyBranchName() (tea.Model, tea.Cmd) {
	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	branch := data.BranchName(*issue)
	err := clipboard.WriteAll(branch)
	if err != nil {
		toast, cmd := components.ShowToast(
			fmt.Sprintf("Clipboard error: %s", err),
			components.ToastError, toastDuration,
		)
		m.toast = toast
		return m, cmd
	}
	toast, cmd := components.ShowToast(
		fmt.Sprintf("Copied: %s", branch),
		components.ToastSuccess, toastDuration,
	)
	m.toast = toast
	return m, cmd
}

// createAndSwitchBranch creates a git branch and switches to it.
func (m Model) createAndSwitchBranch() (tea.Model, tea.Cmd) {
	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	branch := data.BranchName(*issue)
	issueCopy := *issue
	return m, func() tea.Msg {
		err := exec.Command("git", "checkout", "-b", branch).Run()
		action := fmt.Sprintf("branch: %s", branch)
		if err != nil {
			return mutateResultMsg{issueID: issueCopy.ID, action: action, err: err}
		}
		return mutateResultMsg{issueID: issueCopy.ID, action: action}
	}
}

// buildPaletteCommands returns the context-aware list of palette commands.
func (m Model) buildPaletteCommands() []components.PaletteCommand {
	cmds := []components.PaletteCommand{
		{Name: "Set status: in_progress", Desc: "Mark issue as rolling", Key: "1", Action: components.ActionSetInProgress},
		{Name: "Set status: open", Desc: "Mark issue as lined up", Key: "2", Action: components.ActionSetOpen},
		{Name: "Close issue", Desc: "Mark issue as closed", Key: "3", Action: components.ActionCloseIssue},
		{Name: "Set priority: P1 high", Desc: "Urgent work", Key: "!", Action: components.ActionSetPriorityHigh},
		{Name: "Set priority: P2 medium", Desc: "Normal priority", Key: "@", Action: components.ActionSetPriorityMedium},
		{Name: "Set priority: P3 low", Desc: "Can wait", Key: "#", Action: components.ActionSetPriorityLow},
		{Name: "Set priority: P4 backlog", Desc: "Someday maybe", Key: "$", Action: components.ActionSetPriorityBacklog},
		{Name: "Copy branch name", Desc: "Copy git branch to clipboard", Key: "b", Action: components.ActionCopyBranch},
		{Name: "Create git branch", Desc: "Checkout new branch for issue", Key: "B", Action: components.ActionCreateBranch},
		{Name: "New issue", Desc: "Create a new beads issue", Key: "N", Action: components.ActionNewIssue},
		{Name: "Toggle focus mode", Desc: "Show only my work + top priority", Key: "f", Action: components.ActionToggleFocus},
		{Name: "Toggle closed issues", Desc: "Show/hide past the stand", Key: "c", Action: components.ActionToggleClosed},
		{Name: "Filter", Desc: "Fuzzy filter the parade list", Key: "/", Action: components.ActionFilter},
		{Name: "Help", Desc: "Show keybinding help", Key: "?", Action: components.ActionHelp},
		{Name: "Quit", Desc: "Exit Mardi Gras", Key: "q", Action: components.ActionQuit},
	}

	if m.claudeAvail {
		cmds = append(cmds,
			components.PaletteCommand{Name: "Launch agent", Desc: "Start Claude agent on issue", Key: "a", Action: components.ActionLaunchAgent},
			components.PaletteCommand{Name: "Kill agent", Desc: "Stop agent working on issue", Key: "A", Action: components.ActionKillAgent},
		)
	}

	if m.gtEnv.Available {
		cmds = append(cmds,
			components.PaletteCommand{Name: "Toggle Gas Town", Desc: "Show/hide Gas Town panel", Key: "^g", Action: components.ActionToggleGasTown},
			components.PaletteCommand{Name: "Sling with formula", Desc: "Pick formula and sling to polecat", Key: "s", Action: components.ActionSlingFormula},
			components.PaletteCommand{Name: "Nudge agent", Desc: "Nudge agent with message", Key: "n", Action: components.ActionNudgeAgent},
		)
	}

	return cmds
}

// executePaletteAction maps a palette action to an existing method.
func (m Model) executePaletteAction(action components.PaletteAction) (tea.Model, tea.Cmd) {
	switch action {
	case components.ActionSetInProgress:
		return m.quickAction(data.StatusInProgress, "in_progress")
	case components.ActionSetOpen:
		return m.quickAction(data.StatusOpen, "open")
	case components.ActionCloseIssue:
		return m.closeSelectedIssue()
	case components.ActionSetPriorityHigh:
		return m.setPriority(data.PriorityHigh)
	case components.ActionSetPriorityMedium:
		return m.setPriority(data.PriorityMedium)
	case components.ActionSetPriorityLow:
		return m.setPriority(data.PriorityLow)
	case components.ActionSetPriorityBacklog:
		return m.setPriority(data.PriorityBacklog)
	case components.ActionCopyBranch:
		return m.copyBranchName()
	case components.ActionCreateBranch:
		return m.createAndSwitchBranch()
	case components.ActionNewIssue:
		m.creating = true
		m.createForm = components.NewCreateForm(m.width, m.height)
		return m, m.createForm.Init()
	case components.ActionToggleFocus:
		m.focusMode = !m.focusMode
		m.rebuildParade()
		label := "Focus mode ON"
		if !m.focusMode {
			label = "Focus mode OFF"
		}
		toast, cmd := components.ShowToast(label, components.ToastInfo, toastDuration)
		m.toast = toast
		return m, cmd
	case components.ActionToggleClosed:
		m.parade.ToggleClosed()
		m.syncSelection()
		return m, nil
	case components.ActionFilter:
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink
	case components.ActionLaunchAgent:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	case components.ActionKillAgent:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	case components.ActionSlingFormula:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	case components.ActionNudgeAgent:
		return m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	case components.ActionToggleGasTown:
		if !m.gtEnv.Available {
			return m, nil
		}
		m.showGasTown = !m.showGasTown
		if m.showGasTown {
			m.gasTown.SetStatus(m.townStatus, m.gtEnv)
		}
		return m, nil
	case components.ActionHelp:
		m.showHelp = true
		return m, nil
	case components.ActionQuit:
		return m, tea.Quit
	}
	return m, nil
}

// handleGasTownAction processes actions emitted by the Gas Town panel.
func (m Model) handleGasTownAction(msg views.GasTownActionMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "nudge":
		// Reuse the existing nudge input flow
		m.nudging = true
		target := msg.Agent.Address
		if target == "" {
			target = msg.Agent.Name
		}
		m.nudgeTarget = target
		m.nudgeInput = textinput.New()
		m.nudgeInput.Prompt = ui.InputPrompt.Render("nudge> ")
		m.nudgeInput.Placeholder = "Message for " + msg.Agent.Name + "..."
		m.nudgeInput.TextStyle = ui.InputText
		m.nudgeInput.Cursor.Style = ui.InputCursor
		m.nudgeInput.Width = 50
		m.nudgeInput.Focus()
		return m, textinput.Blink

	case "handoff":
		if !m.inTmux {
			toast, cmd := components.ShowToast(
				"Handoff requires tmux",
				components.ToastError, toastDuration,
			)
			m.toast = toast
			return m, cmd
		}
		target := msg.Agent.Address
		if target == "" {
			target = msg.Agent.Name
		}
		agentName := msg.Agent.Name
		projDir := m.projectDir
		return m, func() tea.Msg {
			_, err := gastown.HandoffInTmux(target, projDir)
			return handoffResultMsg{target: agentName, err: err}
		}

	case "decommission":
		address := msg.Agent.Address
		if address == "" {
			address = msg.Agent.Name
		}
		return m, func() tea.Msg {
			err := gastown.Decommission(address)
			return decommissionResultMsg{address: msg.Agent.Name, err: err}
		}
	}
	return m, nil
}

// diffIssues compares new issues against the previous snapshot and returns the count of changes.
func (m *Model) diffIssues(newIssues []data.Issue) int {
	if len(m.prevIssueMap) == 0 {
		return 0
	}

	changed := 0
	newMap := make(map[string]data.Status, len(newIssues))
	for _, iss := range newIssues {
		newMap[iss.ID] = iss.Status
	}

	// Check for status changes or new issues
	for id, newStatus := range newMap {
		oldStatus, existed := m.prevIssueMap[id]
		if !existed || oldStatus != newStatus {
			m.changedIDs[id] = true
			changed++
		}
	}

	// Check for removed issues
	for id := range m.prevIssueMap {
		if _, exists := newMap[id]; !exists {
			changed++
		}
	}

	return changed
}

// syncSelection updates the detail panel with the currently selected issue.
func (m *Model) syncSelection() {
	if m.parade.SelectedIssue != nil {
		m.detail.SetIssue(m.parade.SelectedIssue)
	}
}

// layout recalculates dimensions for all sub-components.
func (m *Model) layout() {
	headerH := 2
	footerH := 2
	bodyH := m.height - headerH - footerH
	if bodyH < 1 {
		bodyH = 1
	}

	paradeW := m.width * 2 / 5
	if paradeW < 30 {
		paradeW = 30
	}
	detailW := m.width - paradeW

	m.header = components.Header{
		Width:            m.width,
		Groups:           m.groups,
		AgentCount:       len(m.activeAgents),
		TownStatus:       m.townStatus,
		GasTownAvailable: m.gtEnv.Available,
	}

	m.parade.SetSize(paradeW, bodyH)
	m.detail.SetSize(detailW, bodyH)
	m.gasTown.SetSize(detailW, bodyH)
	m.detail.AllIssues = m.issues
	m.detail.IssueMap = data.BuildIssueMap(m.issues)
	m.detail.BlockingTypes = m.blockingTypes

	if len(m.parade.Items) == 0 {
		m.parade = views.NewParade(m.issues, paradeW, bodyH, m.blockingTypes)
		m.syncSelection()
	}

	m.detail.Viewport = viewport.New(detailW-2, bodyH)
	m.propagateAgentState()
	if m.parade.SelectedIssue != nil {
		m.detail.SetIssue(m.parade.SelectedIssue)
	}
}

// rebuildParade reconstructs the parade from current issues, preserving selection if possible.
func (m *Model) rebuildParade() {
	oldSelectedID := ""
	if m.parade.SelectedIssue != nil {
		oldSelectedID = m.parade.SelectedIssue.ID
	}
	oldShowClosed := m.parade.ShowClosed

	paradeW := m.parade.Width
	bodyH := m.parade.Height
	if paradeW == 0 {
		paradeW = m.width * 2 / 5
	}
	if bodyH == 0 {
		bodyH = m.height - 4
	}

	filteredIssues := data.FilterIssues(m.issues, m.filterInput.Value())
	if m.focusMode {
		filteredIssues = data.FocusFilter(filteredIssues, m.blockingTypes)
	}
	groups := m.groups
	if m.filterInput.Value() != "" || m.focusMode {
		groups = data.GroupByParade(filteredIssues, m.blockingTypes)
	}

	m.header = components.Header{
		Width:            m.width,
		Groups:           groups,
		AgentCount:       len(m.activeAgents),
		TownStatus:       m.townStatus,
		GasTownAvailable: m.gtEnv.Available,
	}

	m.parade = views.NewParade(filteredIssues, paradeW, bodyH, m.blockingTypes)
	if oldShowClosed {
		m.parade.ToggleClosed()
	}
	m.restoreParadeSelection(oldSelectedID)

	// Propagate change indicators to parade
	m.parade.ChangedIDs = m.changedIDs

	m.detail.AllIssues = m.issues
	m.detail.IssueMap = data.BuildIssueMap(m.issues)
	m.detail.BlockingTypes = m.blockingTypes
	m.propagateAgentState()
	m.syncSelection()
}

// restoreParadeSelection restores selection by issue ID when possible.
func (m *Model) restoreParadeSelection(issueID string) {
	if issueID == "" {
		return
	}
	for i, item := range m.parade.Items {
		if item.IsHeader || item.Issue == nil || item.Issue.ID != issueID {
			continue
		}
		m.parade.Cursor = i
		m.parade.SelectedIssue = item.Issue

		if m.parade.Cursor < m.parade.ScrollOffset {
			m.parade.ScrollOffset = m.parade.Cursor
		}
		if m.parade.Cursor >= m.parade.ScrollOffset+m.parade.Height {
			m.parade.ScrollOffset = m.parade.Cursor - m.parade.Height + 1
		}

		maxOffset := len(m.parade.Items) - m.parade.Height
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.parade.ScrollOffset > maxOffset {
			m.parade.ScrollOffset = maxOffset
		}
		if m.parade.ScrollOffset < 0 {
			m.parade.ScrollOffset = 0
		}
		return
	}
}

// propagateAgentState pushes active agent info to all sub-views.
func (m *Model) propagateAgentState() {
	m.parade.ActiveAgents = m.activeAgents
	m.parade.TownStatus = m.townStatus
	m.detail.ActiveAgents = m.activeAgents
	m.detail.TownStatus = m.townStatus
	m.header.AgentCount = len(m.activeAgents)
	m.header.TownStatus = m.townStatus
	m.header.GasTownAvailable = m.gtEnv.Available
	if m.detail.Issue != nil {
		m.detail.SetIssue(m.detail.Issue)
	}
}

// pollAgentState returns a Cmd that queries either Gas Town or raw tmux for agent state.
func pollAgentState(gtEnv gastown.Env, inTmux bool) tea.Cmd {
	if gtEnv.Available {
		return func() tea.Msg {
			status, err := gastown.FetchStatus()
			return townStatusMsg{status: status, err: err}
		}
	}
	if !inTmux {
		return nil
	}
	return func() tea.Msg {
		agents, err := agent.ListAgentWindows()
		if err != nil {
			return agentStatusMsg{activeAgents: make(map[string]string)}
		}
		return agentStatusMsg{activeAgents: agents}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	header := m.header.View()

	rightPanel := m.detail.View()
	if m.showGasTown && m.gtEnv.Available {
		rightPanel = m.gasTown.View()
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.parade.View(),
		rightPanel,
	)

	var bottomBar string
	if m.toast.Active() {
		bottomBar = m.toast.View(m.width)
	} else if m.parade.SelectionCount() > 0 {
		// Bulk action bar when items are multi-selected
		bottomBar = components.BulkFooter(m.width, m.parade.SelectionCount(), m.gtEnv.Available)
	} else if m.nudging {
		bottomBar = lipgloss.NewStyle().
			Padding(0, 1).
			Width(m.width).
			Render(m.nudgeInput.View())
	} else if m.filtering || m.filterInput.Value() != "" {
		bottomBar = lipgloss.NewStyle().
			Padding(0, 1).
			Width(m.width).
			Render(m.filterInput.View())
	} else {
		footer := components.NewFooter(m.width, m.activPane == PaneDetail, m.gtEnv.Available)
		footer.SourcePath = m.watchPath
		footer.LastRefresh = m.lastFileMod
		footer.PathExplicit = m.pathExplicit
		bottomBar = footer.View()
	}

	divider := components.Divider(m.width)

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		divider,
		bottomBar,
	)

	// Confetti overlay
	if m.confetti.Active() {
		overlay := m.confetti.View()
		if overlay != "" {
			screen = overlayStrings(screen, overlay)
		}
	}

	if m.showPalette {
		return m.palette.View()
	}

	if m.showHelp {
		helpModal := components.NewHelp(m.width, m.height).View()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpModal)
	}

	if m.creating {
		formTitle := ui.HelpTitle.Render("[ NEW ISSUE ]")
		formBody := m.createForm.View()
		formHint := ui.HelpHint.Render("esc to cancel")
		formContent := lipgloss.JoinVertical(lipgloss.Left, formTitle, "", formBody, "", formHint)
		formBox := ui.HelpOverlayBg.Width(m.width - 8).Render(formContent)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, formBox)
	}

	return screen
}

// overlayStrings composites non-space characters from overlay onto base.
func overlayStrings(base, overlay string) string {
	baseLines := splitLines(base)
	overlayLines := splitLines(overlay)

	for y := 0; y < len(overlayLines) && y < len(baseLines); y++ {
		baseRunes := []rune(baseLines[y])
		overlayRunes := []rune(overlayLines[y])
		for x := 0; x < len(overlayRunes) && x < len(baseRunes); x++ {
			if overlayRunes[x] != ' ' {
				baseRunes[x] = overlayRunes[x]
			}
		}
		baseLines[y] = string(baseRunes)
	}

	return joinLines(baseLines)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
