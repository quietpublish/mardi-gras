package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/codexapp"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// riskJudge answers the approval questions from a table of noul
// probabilities, with a fixed risk score and intent.
func riskJudge(nouls map[string]float64, risk float64, intent string, conf float64) *fakeJudge {
	return &fakeJudge{answer: func(key string, q jev.Question) (jev.Answer, bool) {
		switch q.Type {
		case jev.Noul:
			p, ok := nouls[key]
			if !ok {
				return jev.Answer{}, false
			}
			return jev.Answer{Type: jev.Noul, Noul: p, Confidence: conf}, true
		case jev.Score:
			return jev.Answer{Type: jev.Score, Score: risk, Confidence: conf}, true
		case jev.Choice:
			return jev.Answer{Type: jev.Choice, Choice: intent, Confidence: conf}, true
		}
		return jev.Answer{}, false
	}}
}

func execApproval(id string, argv ...string) codexApprovalRequestMsg {
	return codexApprovalRequestMsg{
		issueID:  "mg-1",
		req:      codexapp.ServerRequest{RawID: json.RawMessage(id)},
		approval: codexapp.Approval{Kind: "exec", Command: argv, Cwd: "/work/mg", Reason: "running the tests"},
		ok:       true,
	}
}

func newApprovalModel(t *testing.T, judge jev.Evaluator) Model {
	t.Helper()
	issues := []data.Issue{testIssue("mg-1", data.StatusInProgress)}
	issues[0].Title = "Fix the flaky parade test"
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	m.projectDir = "/work/mg"
	m.jev.client = judge
	return m
}

// advise opens the dialog for msg and runs the advisory Cmd to its verdict.
func advise(t *testing.T, m Model, msg codexApprovalRequestMsg) (Model, tea.Cmd) {
	t.Helper()
	cmd := m.openApprovalDialog(msg)
	if !m.approving {
		t.Fatal("dialog should be open")
	}
	return m, cmd
}

func TestApprovalAdviceOffWithoutJev(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "")
	m := newApprovalModel(t, nil)
	m.jev.client = nil
	m, cmd := advise(t, m, execApproval("1", "go", "test", "./..."))
	if cmd != nil {
		t.Fatal("Jev off: no advisory request")
	}
	if out := m.approvalDialog.View(); strings.Contains(out, "jev") {
		t.Fatalf("no judge line when Jev is off:\n%s", out)
	}
	// The static deny-list still applies without Jev.
	m, cmd = advise(t, m, execApproval("2", "git", "push", "--force", "origin", "main"))
	if cmd != nil || !m.approvalDialog.Denied() || m.approvalDialog.Selected() != "denied" {
		t.Fatalf("deny-list should flag a force-push regardless of Jev: denied %v selected %q", m.approvalDialog.Denied(), m.approvalDialog.Selected())
	}
}

func TestApprovalAdviceLowRisk(t *testing.T) {
	judge := riskJudge(map[string]float64{"destructive": 0.01, "exfil": 0.0, "in_scope": 0.99, "obfuscated": 0.0}, 1, "build/test", 0.94)
	m := newApprovalModel(t, judge)
	msg := execApproval("7", "go", "test", "./internal/app/...")
	m, cmd := advise(t, m, msg)
	if cmd == nil {
		t.Fatal("expected an advisory request")
	}
	if out := m.approvalDialog.View(); !strings.Contains(out, "evaluating") {
		t.Fatalf("dialog should show the judge is pending:\n%s", out)
	}

	res, ok := cmd().(jevApprovalMsg)
	if !ok || res.err != nil || res.rawID != "7" {
		t.Fatalf("got %+v", res)
	}
	call := judge.calls[0]
	state := call.state.(map[string]any)
	if state["kind"] != "exec" || state["cwd"] != "." || state["issue"].(map[string]any)["title"] != "Fix the flaky parade test" {
		t.Fatalf("state = %+v", state)
	}
	if _, has := state["agent_reason_untrusted"]; !has {
		t.Fatal("the agent's reason should be sent, labelled untrusted")
	}
	for _, q := range []string{"destructive", "exfil", "in_scope", "obfuscated", "risk", "intent"} {
		if _, ok := call.questions[q]; !ok {
			t.Errorf("missing question %q", q)
		}
	}

	model, _ := m.Update(res)
	m = model.(Model)
	out := m.approvalDialog.View()
	if !strings.Contains(out, "low risk") || !strings.Contains(out, "intent build/test") || strings.Contains(out, "evaluating") {
		t.Fatalf("verdict should render:\n%s", out)
	}
	if m.approvalDialog.Selected() != "approved" {
		t.Fatalf("low risk keeps the default on Approve once, got %q", m.approvalDialog.Selected())
	}
	if m.jev.health.State != jev.HealthHealthy || m.jev.tokens == 0 {
		t.Fatalf("health %s tokens %d", m.jev.health.State, m.jev.tokens)
	}
}

func TestApprovalAdviceHighRiskMovesCursor(t *testing.T) {
	judge := riskJudge(map[string]float64{"destructive": 0.92, "exfil": 0.05, "in_scope": 0.4, "obfuscated": 0.0}, 4, "system-admin", 0.9)
	m := newApprovalModel(t, judge)
	m, cmd := advise(t, m, execApproval("8", "rm", "-rf", "build", "dist"))
	model, _ := m.Update(cmd())
	m = model.(Model)
	if !strings.Contains(m.approvalDialog.View(), "high risk") || m.approvalDialog.Selected() != "denied" {
		t.Fatalf("high risk should park the cursor on Deny, got %q:\n%s", m.approvalDialog.Selected(), m.approvalDialog.View())
	}
	// It is advice: the human can still approve.
	model, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	model, _ = model.(Model).Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if model.(Model).approvalDialog.Selected() != "approved" {
		t.Fatal("the cursor must remain the human's")
	}
}

func TestApprovalAdviceEdgeCaseLowConfidenceNeverMovesCursor(t *testing.T) {
	judge := riskJudge(map[string]float64{"destructive": 0.9, "exfil": 0, "in_scope": 0.5, "obfuscated": 0}, 4, "unclear", 0.3)
	m := newApprovalModel(t, judge)
	m, cmd := advise(t, m, execApproval("9", "make", "clean"))
	model, _ := m.Update(cmd())
	m = model.(Model)
	if m.approvalDialog.Selected() != "approved" || !strings.Contains(m.approvalDialog.View(), "possibly high risk") {
		t.Fatalf("a shaky high-risk verdict is shown but must not move the cursor: %q\n%s", m.approvalDialog.Selected(), m.approvalDialog.View())
	}
}

func TestApprovalAdviceEdgeCaseStaleVerdictDropped(t *testing.T) {
	judge := riskJudge(map[string]float64{"destructive": 0.95, "exfil": 0, "in_scope": 0, "obfuscated": 0}, 4, "unclear", 0.9)
	m := newApprovalModel(t, judge)
	m, cmd := advise(t, m, execApproval("10", "x"))
	stale := cmd().(jevApprovalMsg)
	// The human answered and a different request is now on screen.
	m, _ = advise(t, m, execApproval("11", "go", "test"))
	model, _ := m.Update(stale)
	m = model.(Model)
	if m.approvalDialog.Selected() != "approved" || strings.Contains(m.approvalDialog.View(), "high risk") {
		t.Fatal("a verdict for an earlier request must not touch the current dialog")
	}
	// And one landing after the modal closed is harmless.
	m.approving = false
	if _, cmd := m.Update(stale); cmd != nil {
		t.Fatal("nothing to do once the modal is gone")
	}
}

func TestApprovalAdviceEdgeCaseJevErrorShowsUnavailable(t *testing.T) {
	judge := &fakeJudge{err: errors.New("jev: dial tcp: connection refused")}
	m := newApprovalModel(t, judge)
	m, cmd := advise(t, m, execApproval("12", "go", "build"))
	model, _ := m.Update(cmd())
	m = model.(Model)
	out := m.approvalDialog.View()
	if !strings.Contains(out, "unavailable") || strings.Contains(out, "evaluating") {
		t.Fatalf("an error should read unavailable:\n%s", out)
	}
	if m.approvalDialog.Selected() != "approved" || m.jev.health.ConsecFailures != 1 {
		t.Fatalf("selected %q failures %d", m.approvalDialog.Selected(), m.jev.health.ConsecFailures)
	}
}

func TestApprovalAdviceDenyListSkipsCursorMove(t *testing.T) {
	judge := riskJudge(map[string]float64{"destructive": 0, "exfil": 0, "in_scope": 1, "obfuscated": 0}, 0, "read/inspect", 0.95)
	m := newApprovalModel(t, judge)
	m, cmd := advise(t, m, execApproval("13", "bash", "-lc", "sudo rm -rf /tmp/x"))
	if !m.approvalDialog.Denied() || m.approvalDialog.Selected() != "denied" {
		t.Fatal("deny-list should flag privilege escalation")
	}
	model, _ := m.Update(cmd())
	m = model.(Model)
	if m.approvalDialog.Selected() != "denied" {
		t.Fatal("a reassuring verdict must not move the cursor off Deny on a deny-listed request")
	}
}

func TestApprovalAdvicePatchQuestions(t *testing.T) {
	judge := riskJudge(map[string]float64{"sensitive": 0, "deletes_tests": 0, "in_scope": 0.9}, 1, "", 0.9)
	m := newApprovalModel(t, judge)
	msg := codexApprovalRequestMsg{
		issueID: "mg-1", req: codexapp.ServerRequest{RawID: json.RawMessage(`"p1"`)}, ok: true,
		approval: codexapp.Approval{Kind: "patch", Cwd: "/work/mg", Changes: map[string]json.RawMessage{
			"/work/mg/internal/app/app.go": json.RawMessage(`{"unified_diff":"secret contents"}`),
			"internal/app/app_test.go":     json.RawMessage(`{}`),
		}},
	}
	m, cmd := advise(t, m, msg)
	cmd()
	call := judge.calls[0]
	state := call.state.(map[string]any)
	files := state["files"].([]string)
	if strings.Join(files, ",") != "internal/app/app.go,internal/app/app_test.go" {
		t.Fatalf("files = %v, want project-relative paths", files)
	}
	if b := mustJSON(t, state); strings.Contains(string(b), "secret contents") {
		t.Fatal("file contents must never be sent")
	}
	for _, q := range []string{"sensitive", "deletes_tests", "in_scope", "risk"} {
		if _, ok := call.questions[q]; !ok {
			t.Errorf("missing patch question %q", q)
		}
	}
	if _, ok := call.questions["intent"]; ok {
		t.Error("intent is an exec question")
	}
	_ = m
}

func TestApprovalVerdictLevels(t *testing.T) {
	a := func(p, conf float64) jev.Answer { return jev.Answer{Type: jev.Noul, Noul: p, Confidence: conf} }
	low := approvalVerdict("exec", map[string]jev.Answer{"destructive": a(0.02, 0.9), "exfil": a(0.01, 0.9), "in_scope": a(0.97, 0.9), "obfuscated": a(0, 0.9)})
	if low.Level != 0 || low.Summary != "low risk" {
		t.Fatalf("low = %+v", low)
	}
	outOfScope := approvalVerdict("exec", map[string]jev.Answer{"destructive": a(0.02, 0.9), "in_scope": a(0.2, 0.9)})
	if outOfScope.Level != 2 {
		t.Fatalf("being out of scope is a risk: %+v", outOfScope)
	}
	shaky := approvalVerdict("exec", map[string]jev.Answer{"destructive": a(0.02, 0.4), "in_scope": a(0.97, 0.9)})
	if shaky.Level != 1 || shaky.Summary != "review" {
		t.Fatalf("low confidence must not show green: %+v", shaky)
	}
	risky := approvalVerdict("patch", map[string]jev.Answer{"sensitive": a(0.1, 0.9), "risk": {Type: jev.Score, Score: 3, Confidence: 0.9}})
	if risky.Level != 2 {
		t.Fatalf("the risk score alone can raise the level: %+v", risky)
	}
	if v := approvalVerdict("exec", nil); v.Summary != "unavailable" {
		t.Fatalf("no answers = %+v", v)
	}
}

func TestRelToProjectAndShellPayload(t *testing.T) {
	if got := relToProject("/work/mg/internal/x.go", "/work/mg"); got != "internal/x.go" {
		t.Fatalf("rel = %q", got)
	}
	if got := relToProject("/etc/passwd", "/work/mg"); got != "outside-project/passwd" {
		t.Fatalf("outside = %q", got)
	}
	if got := relToProject("/home/me/x", ""); got != "abs/x" {
		t.Fatalf("no project = %q", got)
	}
	if got := shellPayload([]string{"bash", "-lc", "go test ./... && echo ok"}); got != "go test ./... && echo ok" {
		t.Fatalf("payload = %q", got)
	}
	if got := shellPayload([]string{"go", "test"}); got != "go test" {
		t.Fatalf("plain = %q", got)
	}
}
