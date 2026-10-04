package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// dupJudge answers every "dup/<id>" question from a fixed table and records
// the state it was shown.
type dupJudge struct {
	fakeJudge
	probs map[string]float64 // issue ID -> p(duplicate)
	conf  float64
}

func (d *dupJudge) Evaluate(ctx context.Context, state any, qs map[string]jev.Question) (*jev.Result, error) {
	res, err := d.fakeJudge.Evaluate(ctx, state, qs)
	if err != nil {
		return nil, err
	}
	for k := range qs {
		id := strings.TrimPrefix(k, "dup/")
		res.Answers[k] = jev.Answer{Type: jev.Noul, Noul: d.probs[id], Confidence: d.conf}
	}
	return res, nil
}

func dupIssues() []data.Issue {
	now := time.Now()
	a := testIssue("mg-2", data.StatusInProgress)
	a.Title = "Fix CI pipeline timeout"
	b := testIssue("mg-17", data.StatusOpen)
	b.Title = "Flaky CI pipeline integration stage"
	c := testIssue("mg-1", data.StatusOpen)
	c.Title = "Login loop on Safari"
	for _, iss := range []*data.Issue{&a, &b, &c} {
		iss.UpdatedAt, iss.CreatedAt = now, now
	}
	return []data.Issue{a, b, c}
}

// stubCreate replaces the bd seams and records what they were asked to do.
func stubCreate(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	origCreate, origDep := createIssue, addDependencyTyped
	createIssue = func(title string, _ data.IssueType, _ data.Priority) (string, error) {
		calls = append(calls, "create "+title)
		return "mg-99", nil
	}
	addDependencyTyped = func(id, target, depType string) error {
		calls = append(calls, "dep "+id+" "+target+" "+depType)
		return nil
	}
	t.Cleanup(func() { createIssue, addDependencyTyped = origCreate, origDep })
	return &calls
}

func newDupModel(t *testing.T, judge jev.Evaluator) Model {
	t.Helper()
	m := New(dupIssues(), data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)
	m.jev.client = judge
	m.jev.now = time.Now
	return m
}

var newCI = components.CreateFormResult{Title: "CI pipeline times out", Type: "bug", Priority: "1"}

// submit runs the create form result and returns the model plus the message
// its command produced, if any.
func submit(t *testing.T, m Model, result components.CreateFormResult) (got Model, produced tea.Msg) {
	t.Helper()
	model, cmd := m.Update(result)
	got = model.(Model)
	if cmd == nil {
		return got, nil
	}
	return got, runBatch(cmd)
}

// runBatch runs a command (or batch) and returns the first message that is
// not a toast tick. Batch members run concurrently, so a toast's 4s timer
// does not hold the test; its goroutine is simply left to expire.
func runBatch(cmd tea.Cmd) tea.Msg {
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		if _, toast := msg.(components.ToastDismissMsg); toast {
			return nil
		}
		return msg
	}
	results := make(chan tea.Msg, len(batch))
	live := 0
	for _, c := range batch {
		if c == nil {
			continue
		}
		live++
		go func(c tea.Cmd) { results <- runBatch(c) }(c)
	}
	deadline := time.After(2 * time.Second)
	for ; live > 0; live-- {
		select {
		case m := <-results:
			if m != nil {
				return m
			}
		case <-deadline:
			return nil
		}
	}
	return nil
}

func TestSubmitCreateWithoutJevCreatesDirectly(t *testing.T) {
	calls := stubCreate(t)
	t.Setenv(jev.EnvAPIKey, "")
	m := New(dupIssues(), data.Source{}, data.DefaultBlockingTypes)
	_, msg := submit(t, m, newCI)
	res, ok := msg.(mutateResultMsg)
	if !ok || res.action != "created" || res.err != nil {
		t.Fatalf("got %+v, want a plain create", msg)
	}
	if len(*calls) != 1 || (*calls)[0] != "create CI pipeline times out" {
		t.Fatalf("bd calls = %v", *calls)
	}
}

func TestSubmitCreateEdgeCaseNoCandidatesSkipsJev(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{}, conf: 0.9}
	m := newDupModel(t, judge)
	_, msg := submit(t, m, components.CreateFormResult{Title: "Rotate the logo colours", Type: "task", Priority: "2"})
	if _, ok := msg.(mutateResultMsg); !ok {
		t.Fatalf("got %T, want a direct create", msg)
	}
	if judge.count() != 0 {
		t.Fatal("no overlapping titles means nothing to ask")
	}
}

func TestSubmitCreateEdgeCaseCircuitOpenSkipsJev(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95}, conf: 0.9}
	m := newDupModel(t, judge)
	m.jev.health = jev.Health{State: jev.HealthOpen}
	_, msg := submit(t, m, newCI)
	if _, ok := msg.(mutateResultMsg); !ok || judge.count() != 0 {
		t.Fatalf("an open circuit must create directly; got %T after %d calls", msg, judge.count())
	}
}

func TestSubmitCreateAsksAboutCandidatesOnly(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95, "mg-17": 0.3}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)

	check, ok := msg.(dupCheckMsg)
	if !ok {
		t.Fatalf("got %T, want the check result", msg)
	}
	if judge.count() != 1 {
		t.Fatalf("calls = %d", judge.count())
	}
	call := judge.calls[0]
	state := call.state.(map[string]any)
	newIssue := state["new"].(map[string]any)
	if newIssue["title"] != "CI pipeline times out" || newIssue["type"] != "bug" {
		t.Fatalf("new = %+v", newIssue)
	}
	existing := state["existing"].([]data.IssueSnapshot)
	ids := make(map[string]bool, len(existing))
	for _, s := range existing {
		ids[s.ID] = true
	}
	if !ids["mg-2"] || !ids["mg-17"] || ids["mg-1"] || len(ids) != 2 {
		t.Fatalf("candidates = %v; the Safari issue shares no words", ids)
	}
	if _, ok := call.questions["dup/mg-2"]; !ok || len(call.questions) != len(existing) {
		t.Fatalf("questions = %v", call.questions)
	}
	if len(check.verdicts) != 2 || check.verdicts[0].issue.ID != "mg-2" {
		t.Fatalf("verdicts should be best first: %+v", check.verdicts)
	}
	if !strings.Contains(m.toast.Message, "Checking") {
		t.Fatalf("toast = %q", m.toast.Message)
	}
}

func TestHandleDupCheckStrongMatchOpensDialog(t *testing.T) {
	calls := stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95, "mg-17": 0.55}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	model, cmd := m.Update(msg)
	m = model.(Model)

	if !m.dupDialogOpen || m.pendingCreate == nil || m.pendingCreate.Title != newCI.Title {
		t.Fatalf("dialog open %v pending %+v", m.dupDialogOpen, m.pendingCreate)
	}
	if cmd != nil && runBatch(cmd) != nil {
		t.Fatal("nothing should be created while the dialog is open")
	}
	if len(*calls) != 0 {
		t.Fatalf("bd was called: %v", *calls)
	}
	if m.toast.Active() {
		t.Fatal("the checking toast should be cleared when the dialog opens")
	}
	if sel := m.dupDialog.Selected(); sel.ID != "mg-2" || sel.Prob != 0.95 {
		t.Fatalf("selected = %+v", sel)
	}
	if !strings.Contains(m.View().Content, "POSSIBLE DUPLICATE") {
		t.Fatal("the dialog should render")
	}
}

func TestHandleDupCheckEdgeCaseLowConfidenceCreates(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95}, conf: 0.3}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	model, cmd := m.Update(msg)
	m = model.(Model)
	if m.dupDialogOpen {
		t.Fatal("a verdict below the confidence floor must not interrupt")
	}
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || !strings.Contains(res.action, "similar to mg-2 (95%)") {
		t.Fatalf("got %+v, want a create with the hint (p is still above the hint threshold)", res)
	}
}

func TestHandleDupCheckWeakMatchHints(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.64, "mg-17": 0.2}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	model, cmd := m.Update(msg)
	m = model.(Model)
	res, ok := runBatch(cmd).(mutateResultMsg)
	if m.dupDialogOpen || !ok || res.action != "created · similar to mg-2 (64%)" {
		t.Fatalf("dialog %v, result %+v", m.dupDialogOpen, res)
	}
}

func TestHandleDupCheckEdgeCaseNoMatchCreates(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.1, "mg-17": 0.05}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	_, cmd := m.Update(msg)
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || res.action != "created" {
		t.Fatalf("got %+v", res)
	}
}

func TestHandleDupCheckEdgeCaseJevErrorCreates(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95}, conf: 0.9}
	judge.err = errors.New("jev: dial tcp: connection refused")
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	model, cmd := m.Update(msg)
	m = model.(Model)
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || res.action != "created" {
		t.Fatalf("a failed check must still create: %+v", res)
	}
	if m.jev.health.ConsecFailures != 1 {
		t.Fatal("the failure should feed the circuit")
	}
}

func TestHandleDupCheckWhileModalOpen(t *testing.T) {
	stubCreate(t)
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	m.showPalette = true // the user opened something during the check
	model, _ := m.Update(msg)
	if !model.(Model).dupDialogOpen {
		t.Fatal("a check result swallowed by the palette would lose the create")
	}
}

func dialogModel(t *testing.T) Model {
	t.Helper()
	judge := &dupJudge{probs: map[string]float64{"mg-2": 0.95, "mg-17": 0.55}, conf: 0.9}
	m := newDupModel(t, judge)
	m, msg := submit(t, m, newCI)
	model, _ := m.Update(msg)
	m = model.(Model)
	if !m.dupDialogOpen {
		t.Fatal("setup: dialog should be open")
	}
	return m
}

func TestDupDialogCancel(t *testing.T) {
	calls := stubCreate(t)
	m := dialogModel(t)
	model, cmd := m.Update(components.DuplicateDialogResult{Action: components.DuplicateCancel, TargetID: "mg-2"})
	m = model.(Model)
	if m.dupDialogOpen || m.pendingCreate != nil || cmd != nil || len(*calls) != 0 {
		t.Fatalf("cancel should drop the create: open %v pending %v cmd %v calls %v", m.dupDialogOpen, m.pendingCreate, cmd != nil, *calls)
	}
}

func TestDupDialogJumpSelectsExisting(t *testing.T) {
	stubCreate(t)
	m := dialogModel(t)
	model, _ := m.Update(components.DuplicateDialogResult{Action: components.DuplicateJump, TargetID: "mg-17"})
	m = model.(Model)
	if m.dupDialogOpen || m.parade.SelectedIssue == nil || m.parade.SelectedIssue.ID != "mg-17" {
		t.Fatalf("selected = %+v", m.parade.SelectedIssue)
	}
}

func TestDupDialogJumpEdgeCaseHiddenTarget(t *testing.T) {
	stubCreate(t)
	m := dialogModel(t)
	model, cmd := m.Update(components.DuplicateDialogResult{Action: components.DuplicateJump, TargetID: "mg-404"})
	m = model.(Model)
	if cmd == nil || !strings.Contains(m.toast.Message, "mg-404") {
		t.Fatalf("a target not in the parade should say so: %q", m.toast.Message)
	}
}

func TestDupDialogCreateAnyway(t *testing.T) {
	calls := stubCreate(t)
	m := dialogModel(t)
	model, cmd := m.Update(components.DuplicateDialogResult{Action: components.DuplicateCreate, TargetID: "mg-2"})
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || res.action != "created" || model.(Model).pendingCreate != nil {
		t.Fatalf("got %+v", res)
	}
	if len(*calls) != 1 || (*calls)[0] != "create CI pipeline times out" {
		t.Fatalf("calls = %v", *calls)
	}
}

func TestDupDialogCreateAndLink(t *testing.T) {
	calls := stubCreate(t)
	m := dialogModel(t)
	_, cmd := m.Update(components.DuplicateDialogResult{Action: components.DuplicateLink, TargetID: "mg-2"})
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || res.err != nil || res.issueID != "mg-99" || res.action != "created as duplicate of mg-2" {
		t.Fatalf("got %+v", res)
	}
	if strings.Join(*calls, "; ") != "create CI pipeline times out; dep mg-99 mg-2 duplicates" {
		t.Fatalf("calls = %v", *calls)
	}
}

func TestDupDialogCreateAndLinkEdgeCaseLinkFails(t *testing.T) {
	stubCreate(t)
	addDependencyTyped = func(string, string, string) error { return errors.New("exit status 1") }
	m := dialogModel(t)
	_, cmd := m.Update(components.DuplicateDialogResult{Action: components.DuplicateLink, TargetID: "mg-2"})
	res, ok := runBatch(cmd).(mutateResultMsg)
	if !ok || res.err == nil || res.issueID != "mg-99" || !strings.Contains(res.action, "mark duplicate") {
		t.Fatalf("a failed link should report against the created issue: %+v", res)
	}
}

func TestDupDialogRoutesKeys(t *testing.T) {
	stubCreate(t)
	m := dialogModel(t)
	model, cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = model.(Model)
	if cmd != nil || m.dupDialog.Selected().ID != "mg-17" {
		t.Fatalf("keys should reach the dialog: selected %+v", m.dupDialog.Selected())
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if r, ok := cmd().(components.DuplicateDialogResult); !ok || r.Action != components.DuplicateCancel {
		t.Fatalf("esc = %v", cmd())
	}
}
