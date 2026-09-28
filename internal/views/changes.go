package views

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Changes renders the Recent changes overlay in place of the detail pane: the
// newest records from the bd events journal, newest first.
type Changes struct {
	width   int
	height  int
	records []data.JournalRecord // oldest first, as the app keeps them
	issues  map[string]*data.Issue
	note    string // why there is nothing to show; "" when the journal is live
	offset  int    // rows scrolled down from the newest
	now     func() time.Time
}

// NewChanges creates a Changes panel.
func NewChanges(width, height int) Changes {
	return Changes{width: width, height: height, now: time.Now}
}

// SetSize updates dimensions.
func (c *Changes) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetRecords replaces what the overlay shows. issues supplies titles for
// records that carry none (a delete). A non-empty note explains why there is
// nothing to show, such as the journal being off.
func (c *Changes) SetRecords(records []data.JournalRecord, issues map[string]*data.Issue, note string) {
	c.records = records
	c.issues = issues
	c.note = note
	c.offset = min(c.offset, max(len(records)-1, 0))
}

// Update scrolls the list.
func (c Changes) Update(msg tea.Msg) (Changes, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || len(c.records) == 0 {
		return c, nil
	}
	last := len(c.records) - 1
	switch keyMsg.String() {
	case "j", "down":
		c.offset = min(c.offset+1, last)
	case "k", "up":
		c.offset = max(c.offset-1, 0)
	case "g":
		c.offset = 0
	case "G":
		c.offset = last
	}
	return c, nil
}

// View renders the panel.
func (c Changes) View() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)
	dim := lipgloss.NewStyle().Foreground(ui.Dim)
	lines := []string{
		headerStyle.Render("RECENT CHANGES") + dim.Render("  from the bd events journal"),
		"",
	}

	inner := max(c.width-4, 20)
	switch {
	case c.note != "":
		for _, l := range wrapWords(c.note, inner-2) {
			lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ui.Muted).Render(l))
		}
	case len(c.records) == 0:
		lines = append(lines, dim.Render("  No changes yet. They appear here as bd records them."))
	default:
		rows := max(c.height-5, 1) // header, blank, hint, border
		now := time.Now()
		if c.now != nil {
			now = c.now()
		}
		for i := len(c.records) - 1 - c.offset; i >= 0 && rows > 0; i-- {
			lines = append(lines, c.renderRow(c.records[i], now, inner))
			rows--
		}
	}

	lines = append(lines, "", dim.Render("  E close  j/k scroll"))
	return ui.DetailBorder.Width(c.width).Height(c.height).Render(strings.Join(lines, "\n"))
}

// renderRow lays out one record: age, issue, title, what happened, and who.
func (c Changes) renderRow(r data.JournalRecord, now time.Time, width int) string {
	age := "        "
	if t := r.Time(); !t.IsZero() {
		age = fmt.Sprintf("%8s", data.RelativeAge(now.Sub(t)))
	}
	what := DescribeChange(r)
	who := ""
	if r.Actor != "" {
		who = " · " + r.Actor
	}
	title := ""
	if r.Issue != nil {
		title = r.Issue.Title
	}
	if title == "" && c.issues != nil {
		if iss := c.issues[r.IssueID]; iss != nil {
			title = iss.Title
		}
	}

	fixed := len(age) + 2 + len(r.IssueID) + 2 + 2 + len([]rune(what)) + len([]rune(who)) + 2
	title = truncate(title, max(width-fixed, 8))

	return fmt.Sprintf("  %s  %s  %s  %s%s",
		lipgloss.NewStyle().Foreground(ui.Muted).Render(age),
		lipgloss.NewStyle().Foreground(ui.BrightGold).Render(r.IssueID),
		lipgloss.NewStyle().Foreground(ui.Light).Render(title),
		lipgloss.NewStyle().Foreground(changeColor(r)).Render(what),
		lipgloss.NewStyle().Foreground(ui.Dim).Render(who))
}

// DescribeChange says what a journal record did, in a few words.
func DescribeChange(r data.JournalRecord) string {
	switch r.Op {
	case "create":
		return "created"
	case "close":
		return "closed"
	case "delete":
		return "deleted"
	case "comment":
		if r.Comment != nil {
			text := strings.Join(strings.Fields(r.Comment.Text), " ")
			return "commented: " + truncate(text, 48)
		}
		return "commented"
	case "dep_add", "dep_remove":
		return describeDep(r)
	case "update":
		if r.Actor == "" && r.Issue != nil {
			// bd's derived record for a dependent whose blocked state flipped.
			if r.Issue.IsBlocked {
				return "became blocked"
			}
			return "unblocked"
		}
		if r.Issue != nil && r.Issue.Status != "" {
			return "updated (" + string(r.Issue.Status) + ")"
		}
		return "updated"
	}
	return r.Op
}

func describeDep(r data.JournalRecord) string {
	if r.Dep == nil {
		return strings.ReplaceAll(r.Op, "_", " ")
	}
	added := r.Op == "dep_add"
	switch r.Dep.Kind {
	case "blocks":
		if added {
			return "now waits on " + r.Dep.Target
		}
		return "no longer waits on " + r.Dep.Target
	case "parent-child":
		if added {
			return "moved under " + r.Dep.Target
		}
		return "moved out of " + r.Dep.Target
	}
	if added {
		return fmt.Sprintf("linked to %s (%s)", r.Dep.Target, r.Dep.Kind)
	}
	return fmt.Sprintf("unlinked from %s (%s)", r.Dep.Target, r.Dep.Kind)
}

func changeColor(r data.JournalRecord) color.Color {
	switch r.Op {
	case "close":
		return ui.StatusPassed
	case "create":
		return ui.StatusRolling
	case "delete":
		return ui.StatusStalled
	}
	if r.Op == "update" && r.Actor == "" && r.Issue != nil && r.Issue.IsBlocked {
		return ui.StatusStalled
	}
	return ui.Light
}

// wrapWords breaks text into lines of at most width runes.
func wrapWords(text string, width int) []string {
	var lines []string
	var line string
	for _, w := range strings.Fields(text) {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) > width:
			lines = append(lines, line)
			line = w
		default:
			line += " " + w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
