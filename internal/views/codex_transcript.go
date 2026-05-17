package views

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// CodexTranscriptEntry is one rendered row of the live codex MCP transcript.
// Entries are appended as codex/event notifications arrive and rendered in
// arrival order.
type CodexTranscriptEntry struct {
	At    time.Time
	Kind  string // "agent", "user", "exec", "tool", "search", "patch", "task", "error", "info"
	Title string
	Body  string // optional multi-line body (wrapped on render)
	Error bool
}

// CodexTranscriptState is the rendered state for one issue's session.
type CodexTranscriptState struct {
	IssueID  string
	ThreadID string
	Model    string
	Cwd      string
	Status   string // "running", "done", "errored", "canceled"
	Entries  []CodexTranscriptEntry
	StartAt  time.Time
	EndAt    time.Time
}

// maxTranscriptEntries caps Entries to prevent unbounded growth over long
// codex sessions. The view only renders a screen-height tail, so older
// entries are not visible — we drop them rather than retain forever.
const maxTranscriptEntries = 500

// AppendEntry appends e and trims Entries to the most recent
// maxTranscriptEntries by dropping the oldest in batches. Exported because
// codexDoneMsg handling in internal/app appends synthesized "final" /
// "error" entries that also need bounded growth.
func (s *CodexTranscriptState) AppendEntry(e CodexTranscriptEntry) {
	s.Entries = append(s.Entries, e)
	if overflow := len(s.Entries) - maxTranscriptEntries; overflow > 0 {
		s.Entries = append(s.Entries[:0], s.Entries[overflow:]...)
	}
}

// CodexTranscript renders the right-pane overlay showing live agent state.
type CodexTranscript struct {
	width  int
	height int
	state  *CodexTranscriptState
}

// NewCodexTranscript constructs a transcript view with the given dimensions.
func NewCodexTranscript(w, h int) CodexTranscript {
	return CodexTranscript{width: w, height: h}
}

// SetSize updates dimensions.
func (c *CodexTranscript) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// SetState swaps the underlying transcript state pointer. Passing nil renders
// an empty "no active session" placeholder.
func (c *CodexTranscript) SetState(s *CodexTranscriptState) { c.state = s }

// Update is a no-op for now; the transcript view is read-only. Future work
// will add scrolling, copy, and a kill keybind.
func (c CodexTranscript) Update(_ tea.Msg) (CodexTranscript, tea.Cmd) { return c, nil }

// View renders the transcript inside ui.DetailBorder.
func (c CodexTranscript) View() string {
	body := c.body()
	return ui.DetailBorder.Width(c.width).Height(c.height).Render(body)
}

func (c CodexTranscript) body() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold).Render("CODEX (MCP)")

	if c.state == nil {
		hint := lipgloss.NewStyle().Foreground(ui.Dim).Render(
			"No active codex MCP session for the selected issue.\nPress M to launch one.",
		)
		return header + "\n\n" + hint
	}

	meta := c.metaLine()
	statusLine := c.statusLine()

	out := []string{header, meta, statusLine, ""}

	if len(c.state.Entries) == 0 {
		waiting := lipgloss.NewStyle().Foreground(ui.Dim).Render("waiting for first event...")
		out = append(out, waiting)
	} else {
		// Reserve some space for the header/meta lines. Show the last N
		// entries that fit; transcripts can grow long.
		budget := max(c.height-6, 5)
		rendered := c.renderEntries(budget)
		out = append(out, rendered...)
	}

	hint := lipgloss.NewStyle().Foreground(ui.Dim).Render("  M close  K kill session  esc back")
	out = append(out, "", hint)

	return strings.Join(out, "\n")
}

func (c CodexTranscript) metaLine() string {
	st := c.state
	parts := []string{}
	if st.IssueID != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.Light).Render(st.IssueID))
	}
	if st.Model != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.Muted).Render(st.Model))
	}
	if st.ThreadID != "" {
		short := st.ThreadID
		if len(short) > 8 {
			short = short[:8]
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.Dim).Render("thread "+short))
	}
	return strings.Join(parts, "  ")
}

func (c CodexTranscript) statusLine() string {
	st := c.state
	var sym, label string
	var style lipgloss.Style
	switch st.Status {
	case "done":
		sym = ui.SymWorking
		label = "complete"
		style = lipgloss.NewStyle().Foreground(ui.BrightGreen)
	case "errored":
		sym = ui.SymStalled
		label = "errored"
		style = lipgloss.NewStyle().Foreground(ui.StatusStalled)
	case "canceled":
		sym = ui.SymStalled
		label = "canceled"
		style = lipgloss.NewStyle().Foreground(ui.Muted)
	default:
		sym = ui.SymWorking
		label = "running"
		style = lipgloss.NewStyle().Foreground(ui.BrightGold)
	}
	elapsed := ""
	if !st.StartAt.IsZero() {
		end := st.EndAt
		if end.IsZero() {
			end = time.Now()
		}
		elapsed = fmt.Sprintf("  (%s)", end.Sub(st.StartAt).Truncate(time.Second))
	}
	return style.Render(sym+" "+label) + lipgloss.NewStyle().Foreground(ui.Dim).Render(elapsed)
}

func (c CodexTranscript) renderEntries(budget int) []string {
	entries := c.state.Entries
	// Take last `budget` entries — naive line budget; entries may take more
	// than one line each but this is close enough until we add a viewport.
	start := 0
	if len(entries) > budget {
		start = len(entries) - budget
	}
	var lines []string
	for _, e := range entries[start:] {
		lines = append(lines, c.renderEntry(e)...)
	}
	return lines
}

func (c CodexTranscript) renderEntry(e CodexTranscriptEntry) []string {
	ts := lipgloss.NewStyle().Foreground(ui.Dim).Render(e.At.Format("15:04:05"))
	icon, fg := iconFor(e.Kind, e.Error)
	titleStyle := lipgloss.NewStyle().Foreground(fg)
	line := fmt.Sprintf("%s %s %s", ts, titleStyle.Render(icon), e.Title)
	out := []string{line}
	if e.Body != "" {
		bodyStyle := lipgloss.NewStyle().Foreground(ui.Light)
		body := e.Body
		// Truncate very long bodies — full body is in transcript state but
		// rendering 10k lines kills the TUI.
		if len(body) > 800 {
			body = body[:800] + "…"
		}
		for ln := range strings.SplitSeq(body, "\n") {
			out = append(out, "    "+bodyStyle.Render(ln))
		}
	}
	return out
}

func iconFor(kind string, isError bool) (string, color.Color) {
	if isError {
		return ui.SymStalled, ui.StatusStalled
	}
	switch kind {
	case "agent":
		return "▶", ui.BrightGold
	case "user":
		return "◀", ui.Muted
	case "exec":
		return "$", ui.Light
	case "tool":
		return "→", ui.BrightGold
	case "search":
		return "?", ui.Muted
	case "patch":
		return "±", ui.BrightGreen
	case "task":
		return "─", ui.Dim
	case "info":
		return "·", ui.Dim
	default:
		return "·", ui.Dim
	}
}

// AppendEvent converts a CodexEvent into a transcript entry, mutating state.
// Returns true if the event was actually appended (i.e. it was one of the
// display-relevant kinds). Unrecognized events are dropped — they remain
// available via the raw stream for future expansion.
func (s *CodexTranscriptState) AppendEvent(ev codexmcp.CodexEvent) bool {
	now := time.Now()
	switch ev.EventType() {
	case "session_configured":
		var sc codexmcp.SessionConfiguredEvent
		_ = json.Unmarshal(ev.Msg, &sc)
		if s.ThreadID == "" {
			s.ThreadID = sc.ThreadID
		}
		s.Model = sc.Model
		s.Cwd = sc.Cwd
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "info",
			Title: fmt.Sprintf("session ready  %s  %s", sc.Model, sc.ApprovalPolicy),
		})
		return true
	case "task_started":
		var ts codexmcp.TaskStartedEvent
		_ = json.Unmarshal(ev.Msg, &ts)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "task",
			Title: "turn " + ts.TurnID + " started",
		})
		return true
	case "task_complete":
		var tc codexmcp.TaskCompleteEvent
		_ = json.Unmarshal(ev.Msg, &tc)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "task",
			Title: "turn complete",
		})
		return true
	case "agent_message":
		var am codexmcp.AgentMessageEvent
		_ = json.Unmarshal(ev.Msg, &am)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "agent",
			Title: firstLine(am.Message),
			Body:  remainder(am.Message),
		})
		return true
	case "user_message":
		var um codexmcp.UserMessageEvent
		_ = json.Unmarshal(ev.Msg, &um)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "user",
			Title: firstLine(um.Message),
		})
		return true
	case "exec_command_begin":
		var ec codexmcp.ExecCommandBeginEvent
		_ = json.Unmarshal(ev.Msg, &ec)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "exec",
			Title: strings.Join(ec.Command, " "),
		})
		return true
	case "exec_command_end":
		var ec codexmcp.ExecCommandEndEvent
		_ = json.Unmarshal(ev.Msg, &ec)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "exec",
			Title: fmt.Sprintf("exit %d", ec.ExitCode),
			Error: ec.ExitCode != 0,
		})
		return true
	case "mcp_tool_call_begin":
		var mc codexmcp.MCPToolCallBeginEvent
		_ = json.Unmarshal(ev.Msg, &mc)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "tool",
			Title: mc.Invocation.Server + "/" + mc.Invocation.Tool,
		})
		return true
	case "mcp_tool_call_end":
		var mc codexmcp.MCPToolCallEndEvent
		_ = json.Unmarshal(ev.Msg, &mc)
		title := "tool done"
		if mc.IsError {
			title = "tool errored"
		}
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "tool",
			Title: title,
			Error: mc.IsError,
		})
		return true
	case "error":
		var er codexmcp.ErrorEvent
		_ = json.Unmarshal(ev.Msg, &er)
		s.AppendEntry(CodexTranscriptEntry{
			At:    now,
			Kind:  "agent",
			Title: er.Message,
			Error: true,
		})
		return true
	}
	return false
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for ln := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			return ln
		}
	}
	return ""
}

// remainder returns everything after the first line of s, or "" if there
// is only one line.
func remainder(s string) string {
	if _, rest, ok := strings.Cut(s, "\n"); ok {
		return rest
	}
	return ""
}
