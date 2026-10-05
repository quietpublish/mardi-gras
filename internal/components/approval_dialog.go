package components

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// ApprovalDialogResult is sent when the approval dialog completes. Decision is a
// codex ReviewDecision value ("approved", "approved_for_session", "denied",
// "abort"). Cancelled is true when the user dismissed the dialog (esc/q); the app
// treats that as a denial.
type ApprovalDialogResult struct {
	Decision  string
	Cancelled bool
}

// approvalDecision is one selectable choice in the dialog.
type approvalDecision struct {
	Label string
	Value string
}

// approvalDecisions is the lean-minimum decision set offered for every exec/patch
// approval. Amendment variants (execpolicy/network) are intentionally omitted —
// their response shapes are unverified upstream.
var approvalDecisions = []approvalDecision{
	{"Approve once", "approved"},
	{"Approve for this session", "approved_for_session"},
	{"Deny", "denied"},
	{"Abort turn", "abort"},
}

// ApprovalVerdict is a judge's advisory reading of the request, rendered
// under the command. Level is 0 (looks routine), 1 (worth a look) or 2
// (high risk); Summary and Detail are display text. The app builds it from
// the jev package's answers so this package never imports jev.
type ApprovalVerdict struct {
	Summary string // "low risk", "review", "high risk", "unavailable"
	Detail  string // e.g. "destructive 1% · exfil 0% · in scope 99% · conf 94%"
	Intent  string // e.g. "build/test"
	Level   int
}

// ApprovalDialog prompts the user to approve or deny a codex action (a shell
// command or a patch). It mirrors RecoveryDialog's Update/View shape and is
// decoupled from codexmcp — the app passes plain fields.
type ApprovalDialog struct {
	kind    string // "exec" | "patch"
	message string
	command []string
	cwd     string
	reason  string
	files   []string
	selIdx  int
	width   int
	height  int

	// Advisory annotations. touched records that the user moved the cursor,
	// after which a verdict may inform but never move it.
	pending  bool // a judge's verdict is on its way
	verdict  *ApprovalVerdict
	denyRule string // a static deny-list rule the request tripped
	denyWhat string
	touched  bool
}

// SetPending shows that a judge is evaluating the request.
func (ad *ApprovalDialog) SetPending(on bool) { ad.pending = on }

// SetVerdict shows the judge's reading. If the user has not moved the
// cursor, a high-risk verdict moves it to Deny and a routine one leaves it
// on Approve once; the human still confirms.
func (ad *ApprovalDialog) SetVerdict(v *ApprovalVerdict) {
	ad.pending = false
	ad.verdict = v
	if v == nil || ad.touched || ad.denyRule != "" {
		return
	}
	if v.Level >= 2 {
		ad.selIdx = ad.indexOf("denied")
	}
}

// SetDenyHit flags a request the static deny-list caught: a banner, the
// cursor on Deny, and "approve for this session" withheld, since that
// would blanket-approve whatever comes next.
func (ad *ApprovalDialog) SetDenyHit(rule, what string) {
	ad.denyRule, ad.denyWhat = rule, what
	if rule != "" && !ad.touched {
		ad.selIdx = ad.indexOf("denied")
	}
}

// Denied reports whether the static deny-list flagged the request.
func (ad ApprovalDialog) Denied() bool { return ad.denyRule != "" }

// Selected returns the decision under the cursor.
func (ad ApprovalDialog) Selected() string { return ad.decisions()[ad.selIdx].Value }

// decisions is the choice list, without session-wide approval for a
// deny-listed request.
func (ad ApprovalDialog) decisions() []approvalDecision {
	if ad.denyRule == "" {
		return approvalDecisions
	}
	out := make([]approvalDecision, 0, len(approvalDecisions)-1)
	for _, d := range approvalDecisions {
		if d.Value != "approved_for_session" {
			out = append(out, d)
		}
	}
	return out
}

func (ad ApprovalDialog) indexOf(value string) int {
	for i, d := range ad.decisions() {
		if d.Value == value {
			return i
		}
	}
	return 0
}

// NewApprovalDialog builds an approval dialog. For exec approvals pass command +
// cwd; for patch approvals pass files. reason is optional for both.
func NewApprovalDialog(kind, message string, command []string, cwd, reason string, files []string, width, height int) ApprovalDialog {
	return ApprovalDialog{
		kind:    kind,
		message: message,
		command: command,
		cwd:     cwd,
		reason:  reason,
		files:   files,
		width:   width,
		height:  height,
	}
}

// Update handles key events for the approval dialog.
func (ad ApprovalDialog) Update(msg tea.Msg) (ApprovalDialog, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return ad, nil
	}

	switch km.String() {
	case "esc", "q":
		return ad, func() tea.Msg {
			return ApprovalDialogResult{Cancelled: true}
		}

	case "j", "down":
		ad.touched = true
		if ad.selIdx < len(ad.decisions())-1 {
			ad.selIdx++
		}

	case "k", "up":
		ad.touched = true
		if ad.selIdx > 0 {
			ad.selIdx--
		}

	case "enter":
		selected := ad.decisions()[ad.selIdx]
		return ad, func() tea.Msg {
			return ApprovalDialogResult{Decision: selected.Value}
		}
	}

	return ad, nil
}

// View renders the approval prompt.
func (ad ApprovalDialog) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	normalStyle := lipgloss.NewStyle().Foreground(ui.Light)
	selectedStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)

	var lines []string

	// Title
	heading := "CODEX WANTS TO RUN A COMMAND"
	if ad.kind == "patch" {
		heading = "CODEX WANTS TO APPLY A PATCH"
	}
	lines = append(lines, ad.indented(titleStyle.Render(ui.SymGate+" "+heading))...)
	lines = append(lines, "")

	// Body
	switch ad.kind {
	case "patch":
		lines = append(lines, normalStyle.Render(fmt.Sprintf("  %d file(s) changed:", len(ad.files))))
		for _, f := range ad.files {
			lines = append(lines, fmt.Sprintf("    %s", dimStyle.Render(f)))
		}
	default:
		lines = append(lines, ad.indented(normalStyle.Render(strings.Join(ad.command, " ")))...)
		if ad.cwd != "" {
			lines = append(lines, ad.indented(dimStyle.Render("cwd: "+ad.cwd))...)
		}
	}
	if ad.reason != "" {
		lines = append(lines, "")
		lines = append(lines, ad.indented(dimStyle.Render("reason: "+ad.reason))...)
	}

	// Advisory lines: the deny-list banner, then the judge's reading.
	if ad.denyRule != "" {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.StatusStalled).Render(
			fmt.Sprintf("  %s DENY-LIST: %s", ui.SymStalled, ad.denyRule)))
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  %s", truncate(ad.denyWhat, ad.width-2))))
	}
	if ad.pending || ad.verdict != nil {
		lines = append(lines, "")
		lines = append(lines, ad.indented(renderApprovalVerdict(ad.pending, ad.verdict))...)
	}
	lines = append(lines, "")

	// Decisions
	for i, d := range ad.decisions() {
		cursor := "    "
		labelStyle := normalStyle
		if i == ad.selIdx {
			cursor = selectedStyle.Render("  > ")
			labelStyle = selectedStyle
		}
		lines = append(lines, fmt.Sprintf("%s%s", cursor, labelStyle.Render(ansi.Truncate(d.Label, max(ad.width-4, 8), "…"))))
	}
	lines = append(lines, "")
	lines = append(lines, ad.indented(dimStyle.Render("↑/↓ select   enter confirm   esc deny"))...)

	return strings.Join(lines, "\n")
}

// indented word-wraps s to the dialog's content width under a two-space
// indent. Long reasons and readings used to overrun the box, which wrapped
// them flush left ("conf" / "63%").
func (ad ApprovalDialog) indented(s string) []string {
	// Wrap also hard-breaks a word longer than the line (a long path), which
	// Wordwrap left whole for the box to re-wrap.
	wrapped := ansi.Wrap(s, max(ad.width-2, 20), " /·")
	out := strings.Split(wrapped, "\n")
	for i, l := range out {
		out[i] = "  " + l
	}
	return out
}

// renderApprovalVerdict is the one-line judge reading: "jev ⟳ evaluating…",
// or "jev ✓ low risk  destructive 1% · … · intent build/test".
func renderApprovalVerdict(pending bool, v *ApprovalVerdict) string {
	tag := lipgloss.NewStyle().Foreground(ui.Muted).Render("jev")
	if pending || v == nil {
		return tag + lipgloss.NewStyle().Foreground(ui.Dim).Render(" ⟳ evaluating…")
	}
	var mark string
	var style lipgloss.Style
	switch v.Level {
	case 0:
		mark, style = "✓", lipgloss.NewStyle().Foreground(ui.BrightGreen)
	case 1:
		mark, style = "?", lipgloss.NewStyle().Foreground(ui.StateStuck)
	default:
		mark, style = "✗", lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
	}
	out := tag + " " + style.Render(mark+" "+v.Summary)
	if v.Detail != "" {
		out += lipgloss.NewStyle().Foreground(ui.Dim).Render("  " + v.Detail)
	}
	if v.Intent != "" {
		out += lipgloss.NewStyle().Foreground(ui.Muted).Render(" · intent " + v.Intent)
	}
	return out
}
