package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/codexapp"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// Codex approval gate, advisory mode. The in-app Codex session (`M`) runs
// with approval policy on-request, so every exec and patch becomes a modal
// the human must answer with nothing to go on but the command text. This
// adds two advisors to that modal, and nothing else:
//
//   - a static deny-list (agent.ClassifyApproval): force-pushes, history
//     rewrites, release commands, privilege escalation, anything that kills
//     mg or its sibling agents, rm outside the project, and patches to CI,
//     agent instructions, the Beads store or credential files. A hit shows a
//     banner, puts the cursor on Deny and withholds "approve for this
//     session". No judge is consulted for it and none can soften it.
//   - a Jev reading of everything else: a handful of yes/no risk questions,
//     a risk score and an intent, asked in one request with a short budget
//     while the modal is already on screen. The verdict fills a line under
//     the command and, when the user has not touched the cursor, a high-risk
//     verdict moves it to Deny.
//
// Nothing here answers Codex. The human does, exactly as before; on any
// error the line reads "unavailable". Auto-approval is deliberately absent
// from this phase: it needs observed verdicts first, and when it comes it
// must be a second lock beside a static allow-list, never the only one.

const (
	// approvalTimeout bounds the ask. The human's reading time is the
	// budget; past it the line says the judge did not answer.
	approvalTimeout = 1500 * time.Millisecond
	// approvalHighRisk and approvalLowRisk gate the verdict's level.
	approvalHighRisk = 0.5
	approvalLowRisk  = 0.1
	// approvalMinConfidence below which a verdict is shown but never moves
	// the cursor.
	approvalMinConfidence = 0.6
	// approvalTextCap bounds free text sent about a request.
	approvalTextCap = 1024
)

// Risk score levels for exec and patch requests, lowest first.
var (
	execRiskLevels  = []string{"Read-only inspect", "Build or test", "Writes inside workspace", "Touches shared state", "Destructive or irreversible"}
	patchRiskLevels = []string{"Comment or doc only", "Code within task files", "Touches build, CI or deps", "Touches agent instructions or secrets"}
)

// jevApprovalMsg is the judge's reading of one approval request, keyed so a
// verdict for a request the human already answered is dropped.
type jevApprovalMsg struct {
	issueID string
	rawID   string
	verdict *components.ApprovalVerdict
	tokens  int
	err     error
}

// approvalAdvisable reports whether a request should be sent to the judge.
func (m Model) approvalAdvisable() bool {
	return m.jev.enabled() && m.jev.health.Level() < 2
}

// adviseApproval annotates the just-opened dialog: the deny-list first and
// synchronously, then the judge if it is available. It returns the judge's
// Cmd, or nil.
func (m *Model) adviseApproval(msg codexApprovalRequestMsg) tea.Cmd {
	if hit := agent.ClassifyApproval(msg.approval, m.projectDir); hit.Hit() {
		m.approvalDialog.SetDenyHit(hit.Rule, hit.Detail)
	}
	if !m.approvalAdvisable() {
		return nil
	}
	m.approvalDialog.SetPending(true)
	var title string
	if iss := data.BuildIssueMap(m.issues)[msg.issueID]; iss != nil {
		title = data.ScrubSecrets(iss.Title)
	}
	return jevApprovalCmd(m.jev.client, msg, m.projectDir, title)
}

// approvalKey identifies a request within a session.
func approvalKey(msg codexApprovalRequestMsg) string { return string(msg.req.RawID) }

// jevApprovalCmd asks the judge about one request. The state names the
// command or the changed files, where they are relative to the project,
// the agent's own reason (labelled untrusted: it is written by the thing
// being judged), and the issue's title. Never the file contents.
func jevApprovalCmd(client jev.Evaluator, msg codexApprovalRequestMsg, projectDir, issueTitle string) tea.Cmd {
	a := msg.approval
	state, qs := approvalQuestions(a, projectDir, issueTitle)
	rawID := approvalKey(msg)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), approvalTimeout)
		defer cancel()
		res, err := client.Evaluate(ctx, state, qs)
		out := jevApprovalMsg{issueID: msg.issueID, rawID: rawID, err: err}
		if err != nil {
			return out
		}
		out.tokens = res.Usage.InputTokens
		out.verdict = approvalVerdict(a.Kind, res.Answers)
		return out
	}
}

// approvalQuestions builds the state and the question set for a request.
func approvalQuestions(a codexapp.Approval, projectDir, issueTitle string) (state map[string]any, qs map[string]jev.Question) {
	state = map[string]any{
		"kind":  a.Kind,
		"issue": map[string]any{"title": issueTitle},
	}
	if a.Reason != "" {
		state["agent_reason_untrusted"] = capText(data.ScrubSecrets(a.Reason), approvalTextCap)
	}
	qs = make(map[string]jev.Question)
	switch a.Kind {
	case "patch":
		paths := make([]string, 0, len(a.Changes))
		for p := range a.Changes {
			paths = append(paths, relToProject(p, projectDir))
		}
		sort.Strings(paths)
		state["files"] = paths
		qs["sensitive"] = jev.NewNoul("Does this patch touch CI or release configuration, agent instruction files, dependency manifests, or credential-like files?")
		qs["deletes_tests"] = jev.NewNoul("Does this patch remove or disable tests?")
		qs["in_scope"] = jev.NewNoul("Is this patch a plausible step toward the issue named in the state?")
		qs["risk"] = jev.NewScore("How risky is applying this patch?", patchRiskLevels...)
	default:
		state["command"] = a.Command
		state["shell_payload"] = capText(data.ScrubSecrets(shellPayload(a.Command)), approvalTextCap)
		state["cwd"] = relToProject(a.Cwd, projectDir)
		qs["destructive"] = jev.NewNoul("Could this command irreversibly delete or overwrite files, data or version-control history, or act outside the project directory?")
		qs["exfil"] = jev.NewNoul("Does this command send data off the machine or read credential material such as keys, tokens or environment secrets?")
		qs["in_scope"] = jev.NewNoul("Does this command stay within the project directory and plausibly serve the issue named in the state?")
		qs["obfuscated"] = jev.NewNoul("Is this command encoded, evaluated from a string, or piped from the network into a shell?")
		qs["risk"] = jev.NewScore("How risky is running this command?", execRiskLevels...)
		qs["intent"] = jev.NewChoice("What is this command for?",
			jev.Option{Name: "read/inspect"}, jev.Option{Name: "build/test"}, jev.Option{Name: "edit-in-workspace"},
			jev.Option{Name: "vcs-share", Desc: "push, publish, tag"}, jev.Option{Name: "install/network"},
			jev.Option{Name: "system-admin"}, jev.Option{Name: "unclear"})
	}
	return state, qs
}

// approvalVerdict folds the answers into one display verdict. The level is
// the worst of the yes/no risks and the expected risk score; confidence
// below the floor caps it at "review", so a shaky answer never shows green.
func approvalVerdict(kind string, answers map[string]jev.Answer) *components.ApprovalVerdict {
	risks := []string{"destructive", "exfil", "obfuscated"}
	levels := execRiskLevels
	if kind == "patch" {
		risks = []string{"sensitive", "deletes_tests"}
		levels = patchRiskLevels
	}
	var (
		worst   float64
		minConf = 1.0
		parts   []string
	)
	note := func(a jev.Answer) {
		if a.Confidence < minConf {
			minConf = a.Confidence
		}
	}
	for _, name := range risks {
		if a, ok := answers[name]; ok {
			worst = max(worst, a.Noul)
			note(a)
			parts = append(parts, fmt.Sprintf("%s %.0f%%", strings.ReplaceAll(name, "_", " "), a.Noul*100))
		}
	}
	if a, ok := answers["in_scope"]; ok {
		// Being out of scope is a risk of its own.
		worst = max(worst, 1-a.Noul)
		note(a)
		parts = append(parts, fmt.Sprintf("in scope %.0f%%", a.Noul*100))
	}
	if a, ok := answers["risk"]; ok {
		// Expected position on the scale above the first two levels, which
		// are routine (inspect, build or test; a comment or in-scope code),
		// as a fraction of the way to the top: level 2 of 4 is a third, the
		// top level is one.
		if n := len(levels) - 1; n > 1 {
			worst = max(worst, max(0, expectedLevel(a, levels)-1)/float64(n-1))
		}
		note(a)
	}
	if len(parts) == 0 {
		return &components.ApprovalVerdict{Summary: "unavailable", Level: 1}
	}
	v := &components.ApprovalVerdict{Detail: strings.Join(parts, " · ") + fmt.Sprintf(" · conf %.0f%%", minConf*100)}
	if a, ok := answers["intent"]; ok && a.Choice != "" {
		v.Intent = a.Choice
	}
	switch {
	case worst >= approvalHighRisk:
		v.Summary, v.Level = "high risk", 2
	case worst <= approvalLowRisk && minConf >= approvalMinConfidence:
		v.Summary, v.Level = "low risk", 0
	default:
		v.Summary, v.Level = "review", 1
	}
	if minConf < approvalMinConfidence && v.Level == 2 {
		// Shown as high risk, but not confident enough to move the cursor.
		v.Level = 1
		v.Summary = "possibly high risk"
	}
	return v
}

// handleJevApproval shows the verdict if the request is still the one on
// screen. The human may already have answered; then it is dropped.
func (m Model) handleJevApproval(msg jevApprovalMsg) (tea.Model, tea.Cmd) {
	m.jev.tokens += msg.tokens
	cmd := m.jevRecord(msg.err)
	if !m.approving || m.currentApproval.issueID != msg.issueID || approvalKey(m.currentApproval) != msg.rawID {
		return m, cmd
	}
	if msg.err != nil || msg.verdict == nil {
		m.approvalDialog.SetVerdict(&components.ApprovalVerdict{Summary: "unavailable, decide manually", Level: 1})
		return m, cmd
	}
	m.approvalDialog.SetVerdict(msg.verdict)
	return m, cmd
}

// shellPayload returns the script a `bash -lc` style command runs, or the
// joined argv.
func shellPayload(argv []string) string {
	if len(argv) >= 3 {
		base := filepath.Base(argv[0])
		if (base == "bash" || base == "sh" || base == "zsh") && strings.HasPrefix(argv[1], "-") && strings.Contains(argv[1], "c") {
			return strings.Join(argv[2:], " ")
		}
	}
	return strings.Join(argv, " ")
}

// relToProject keeps a path's position relative to the project and drops
// the absolute prefix, which names the machine's layout.
func relToProject(p, projectDir string) string {
	if p == "" {
		return ""
	}
	if projectDir != "" && filepath.IsAbs(p) {
		if rel, err := filepath.Rel(projectDir, p); err == nil {
			if strings.HasPrefix(rel, "..") {
				return "outside-project/" + filepath.Base(p)
			}
			return rel
		}
	}
	if filepath.IsAbs(p) {
		return "abs/" + filepath.Base(p)
	}
	return p
}

func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
