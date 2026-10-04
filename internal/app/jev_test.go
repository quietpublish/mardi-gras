package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// fakeJudge records every Evaluate call and answers each question with a
// fixed verdict, or fails every call with err.
type fakeJudge struct {
	mu    sync.Mutex
	calls []fakeCall
	err   error
}

type fakeCall struct {
	state     any
	questions map[string]jev.Question
}

func (f *fakeJudge) Evaluate(_ context.Context, state any, qs map[string]jev.Question) (*jev.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{state: state, questions: qs})
	if f.err != nil {
		return nil, f.err
	}
	res := &jev.Result{Model: "fake", Answers: make(map[string]jev.Answer, len(qs))}
	for k, q := range qs {
		res.Answers[k] = jev.Answer{Type: q.Type, Noul: 0.75, Confidence: 0.9}
	}
	res.Usage.InputTokens = 10 * len(qs)
	return res, nil
}

func (f *fakeJudge) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// issuesIn returns the snapshots a call carried as its state.
func (c fakeCall) issuesIn(t *testing.T) []data.IssueSnapshot {
	t.Helper()
	state, ok := c.state.(map[string]any)
	if !ok {
		t.Fatalf("state is %T, want map", c.state)
	}
	snaps, ok := state["issues"].([]data.IssueSnapshot)
	if !ok {
		t.Fatalf("state.issues is %T", state["issues"])
	}
	return snaps
}

var jevTestNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

// newJevModel is a model with Jev on, a registered one-question set and a
// fixed clock, whatever the environment of the test run says.
func newJevModel(fake *fakeJudge, issues []data.Issue) Model {
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.jev.client = fake
	m.jev.now = func() time.Time { return jevTestNow }
	m.jev.questions = func(data.IssueSnapshot) map[string]jev.Question {
		return map[string]jev.Question{"urgent": jev.NewNoul("Is this urgent?")}
	}
	return m
}

// sweep runs the sweep scheduleJev returned and delivers its result.
func sweep(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a sweep to be scheduled")
	}
	msg, ok := cmd().(jevVerdictsMsg)
	if !ok {
		t.Fatalf("sweep returned %T", msg)
	}
	model, _ := m.Update(msg)
	return model.(Model)
}

func TestJevDisabledByDefault(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "")
	m := New([]data.Issue{testIssue("a", data.StatusOpen)}, data.Source{}, data.DefaultBlockingTypes)
	if m.jev.enabled() {
		t.Fatal("Jev must be off without MG_JEV_API_KEY")
	}
	if m.jevFooter() != nil {
		t.Fatal("no footer chip when Jev is off")
	}
	m.jev.questions = func(data.IssueSnapshot) map[string]jev.Question {
		return map[string]jev.Question{"q": jev.NewNoul("?")}
	}
	if m.scheduleJev() != nil {
		t.Fatal("a registered question set must not start a sweep while Jev is off")
	}
}

func TestJevEnabledFromEnv(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "k")
	t.Setenv(jev.EnvToggle, "")
	t.Setenv(jev.EnvScope, "minimal")
	m := New(nil, data.Source{}, data.DefaultBlockingTypes)
	if !m.jev.enabled() || m.jev.scope != data.SnapshotMinimal {
		t.Fatalf("enabled %v scope %v", m.jev.enabled(), m.jev.scope)
	}
	if chip := m.jevFooter(); chip == nil || chip.Label != "" || chip.Level != 0 {
		t.Fatalf("healthy chip = %+v", chip)
	}

	t.Setenv(jev.EnvToggle, "off")
	if New(nil, data.Source{}, data.DefaultBlockingTypes).jev.enabled() {
		t.Fatal("MG_JEV=off must win over the key")
	}
}

func TestJevInitProbesOnlyWhenEnabled(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "")
	off := New(nil, data.Source{}, data.DefaultBlockingTypes)
	on := off
	on.jev.client = &fakeJudge{}
	count := func(m Model) int {
		batch, ok := m.Init()().(tea.BatchMsg)
		if !ok {
			t.Fatal("Init should return a batch")
		}
		return len(batch)
	}
	if count(on) != count(off)+1 {
		t.Fatalf("Init with Jev on should add exactly the probe: %d vs %d", count(on), count(off))
	}
}

func TestJevScheduleAsksOpenIssuesOnce(t *testing.T) {
	fake := &fakeJudge{}
	issues := []data.Issue{
		testIssue("a", data.StatusOpen),
		testIssue("b", data.StatusInProgress),
		testIssue("c", data.StatusClosed),
	}
	m := newJevModel(fake, issues)

	m = sweep(t, m, m.scheduleJev())

	if fake.count() != 1 {
		t.Fatalf("calls = %d, want one chunk", fake.count())
	}
	call := fake.calls[0]
	snaps := call.issuesIn(t)
	if len(snaps) != 2 || snaps[0].ID != "a" || snaps[1].ID != "b" {
		t.Fatalf("state carried %+v; closed issues must be skipped", snaps)
	}
	q, ok := call.questions["a/urgent"]
	if !ok || !strings.HasPrefix(q.Instructions, "About the issue with id a: ") {
		t.Fatalf("questions = %+v", call.questions)
	}
	if len(call.questions) != 2 {
		t.Fatalf("questions = %d, want one per open issue", len(call.questions))
	}

	if m.jev.inFlight {
		t.Fatal("gate still closed after the verdict landed")
	}
	if a := m.jevAnswers("a"); a == nil || a["urgent"].Noul != 0.75 {
		t.Fatalf("cached answers for a = %+v", a)
	}
	if m.jevAnswers("c") != nil {
		t.Fatal("closed issue should have no verdict")
	}
	if m.jev.tokens != 20 {
		t.Fatalf("tokens = %d", m.jev.tokens)
	}
	if m.jev.health.State != jev.HealthHealthy {
		t.Fatalf("health = %s", m.jev.health.State)
	}

	// The same issues again: nothing to ask, no request.
	if cmd := m.scheduleJev(); cmd != nil {
		t.Fatal("an unchanged backlog must not be re-asked")
	}
	model, _ := m.Update(data.FileChangedMsg{Issues: issues})
	m = model.(Model)
	if fake.count() != 1 {
		t.Fatalf("calls = %d after an unchanged reload", fake.count())
	}
}

func TestJevScheduleEdgeCaseReasksOnlyChangedIssues(t *testing.T) {
	fake := &fakeJudge{}
	issues := []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)}
	m := newJevModel(fake, issues)
	m = sweep(t, m, m.scheduleJev())

	// A label is something the snapshot carries; an assignee is not.
	changed := []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)}
	changed[0].Labels = []string{"security"}
	changed[1].Assignee = "someone"
	m.issues = changed
	m = sweep(t, m, m.scheduleJev())

	if fake.count() != 2 {
		t.Fatalf("calls = %d", fake.count())
	}
	snaps := fake.calls[1].issuesIn(t)
	if len(snaps) != 1 || snaps[0].ID != "a" {
		t.Fatalf("second sweep asked about %+v, want only a", snaps)
	}
}

func TestJevScheduleEdgeCaseForgetsRemovedIssues(t *testing.T) {
	fake := &fakeJudge{}
	m := newJevModel(fake, []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)})
	m = sweep(t, m, m.scheduleJev())
	m.issues = []data.Issue{testIssue("a", data.StatusOpen)}
	if m.scheduleJev() != nil {
		t.Fatal("removing an issue is not a reason to ask again")
	}
	if m.jevAnswers("b") != nil {
		t.Fatal("verdicts for a removed issue should be evicted")
	}
}

func TestJevScheduleEdgeCaseChunks(t *testing.T) {
	fake := &fakeJudge{}
	var issues []data.Issue
	for i := range 2*jevChunkSize + 1 {
		issues = append(issues, testIssue(string(rune('a'+i%26))+string(rune('a'+i/26)), data.StatusOpen))
	}
	m := newJevModel(fake, issues)
	m = sweep(t, m, m.scheduleJev())
	if fake.count() != 3 {
		t.Fatalf("calls = %d, want 3 chunks for %d issues", fake.count(), len(issues))
	}
	for _, c := range fake.calls {
		if n := len(c.issuesIn(t)); n > jevChunkSize {
			t.Fatalf("a chunk carried %d issues", n)
		}
	}
	if len(m.jev.cache) != len(issues) {
		t.Fatalf("cached %d of %d", len(m.jev.cache), len(issues))
	}
}

func TestJevScheduleEdgeCaseDirtyWhileInFlight(t *testing.T) {
	fake := &fakeJudge{}
	m := newJevModel(fake, []data.Issue{testIssue("a", data.StatusOpen)})
	cmd := m.scheduleJev()
	if !m.jev.inFlight {
		t.Fatal("expected the gate to close")
	}

	// A reload lands mid-sweep with a change: remembered, not started.
	m.issues[0].Labels = []string{"ui"}
	if m.scheduleJev() != nil || !m.jev.dirty {
		t.Fatal("a second sweep must wait for the first")
	}

	// When the first lands, the dirty reload starts the second.
	msg := cmd().(jevVerdictsMsg)
	model, next := m.Update(msg)
	m = model.(Model)
	if !m.jev.inFlight || m.jev.dirty || next == nil {
		t.Fatalf("inFlight %v dirty %v cmd %v after a dirty verdict", m.jev.inFlight, m.jev.dirty, next != nil)
	}
}

func TestJevVerdictsEdgeCaseStaleGenerationDropped(t *testing.T) {
	m := newJevModel(&fakeJudge{}, []data.Issue{testIssue("a", data.StatusOpen)})
	_ = m.scheduleJev()
	model, _ := m.Update(jevVerdictsMsg{gen: m.jev.gen - 1, results: map[string]jevEntry{"a": {hash: "x"}}})
	m = model.(Model)
	if !m.jev.inFlight || len(m.jev.cache) != 0 {
		t.Fatal("a result from a superseded sweep must be ignored")
	}
}

func TestJevVerdictsWhileModalOpen(t *testing.T) {
	m := newJevModel(&fakeJudge{}, []data.Issue{testIssue("a", data.StatusOpen)})
	cmd := m.scheduleJev()
	m.showPalette = true
	model, _ := m.Update(cmd())
	if model.(Model).jev.inFlight {
		t.Fatal("a verdict swallowed by the palette would stop every later sweep")
	}
}

func TestJevFailureOpensCircuit(t *testing.T) {
	fake := &fakeJudge{err: errors.New("jev: dial tcp: connection refused")}
	m := newJevModel(fake, []data.Issue{testIssue("a", data.StatusOpen)})

	m = sweep(t, m, m.scheduleJev())
	if m.jev.health.ConsecFailures != 1 || !m.toast.Active() {
		t.Fatalf("first failure: health %+v, toast %v", m.jev.health, m.toast.Active())
	}
	if m.jevAnswers("a") != nil {
		t.Fatal("a failed sweep must not cache anything")
	}
	for range 4 {
		m = sweep(t, m, m.scheduleJev())
	}
	if m.jev.health.State != jev.HealthOpen {
		t.Fatalf("health = %s after 5 failures", m.jev.health.State)
	}
	if chip := m.jevFooter(); chip.Label != "paused" || chip.Level != 2 {
		t.Fatalf("chip = %+v", chip)
	}
	if m.scheduleJev() != nil {
		t.Fatal("an open circuit must not schedule")
	}

	// After the backoff one probe chunk goes out, and a success closes it.
	fake.err = nil
	calls := fake.count()
	m.jev.now = func() time.Time { return m.jev.health.RetryAt }
	cmd := m.scheduleJev()
	if cmd == nil || m.jev.health.State != jev.HealthHalfOpen {
		t.Fatalf("at RetryAt: cmd %v, health %s", cmd != nil, m.jev.health.State)
	}
	m = sweep(t, m, cmd)
	if fake.count() != calls+1 || m.jev.health.State != jev.HealthHealthy || m.jevAnswers("a") == nil {
		t.Fatalf("after probe: calls %d, health %s", fake.count(), m.jev.health.State)
	}
}

func TestJevProbeEdgeCaseRejectedKeyDisables(t *testing.T) {
	m := newJevModel(&fakeJudge{}, []data.Issue{testIssue("a", data.StatusOpen)})
	model, _ := m.Update(jevProbeMsg{err: &jev.StatusError{Code: http.StatusUnauthorized}})
	m = model.(Model)
	if m.jev.health.State != jev.HealthDisabled {
		t.Fatalf("health = %s", m.jev.health.State)
	}
	if !m.toast.Active() || !strings.Contains(m.toast.Message, "401") {
		t.Fatalf("toast = %+v", m.toast)
	}
	if chip := m.jevFooter(); chip.Label != "off" {
		t.Fatalf("chip = %+v", chip)
	}
	if m.scheduleJev() != nil {
		t.Fatal("disabled Jev must never schedule")
	}
}

func TestJevProbeStartsFirstSweep(t *testing.T) {
	fake := &fakeJudge{}
	m := newJevModel(fake, []data.Issue{testIssue("a", data.StatusOpen)})
	model, cmd := m.Update(jevProbeMsg{tokens: 7})
	m = model.(Model)
	if cmd == nil || !m.jev.inFlight || m.jev.tokens != 7 {
		t.Fatalf("after a good probe: cmd %v inFlight %v tokens %d", cmd != nil, m.jev.inFlight, m.jev.tokens)
	}
}

func TestJevProbeRunsAgainstClient(t *testing.T) {
	fake := &fakeJudge{}
	m := newJevModel(fake, nil)
	msg, ok := m.jevProbe()().(jevProbeMsg)
	if !ok || msg.err != nil || fake.count() != 1 || msg.tokens != 10 {
		t.Fatalf("probe = %+v (%T), calls %d", msg, msg, fake.count())
	}
	if _, ok := fake.calls[0].questions["ok"]; !ok {
		t.Fatalf("probe questions = %+v", fake.calls[0].questions)
	}
}

func TestJevScheduleEdgeCaseNoQuestionSet(t *testing.T) {
	fake := &fakeJudge{}
	m := newJevModel(fake, []data.Issue{testIssue("a", data.StatusOpen)})
	m.jev.questions = nil
	if m.scheduleJev() != nil || fake.count() != 0 {
		t.Fatal("without a registered question set the loop sends nothing")
	}
}
