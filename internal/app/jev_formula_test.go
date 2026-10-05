package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// formulaDriver is a mock driver whose Formulas() returns a fixed list.
type formulaDriver struct {
	gastown.Driver
	formulas []string
	err      error
	calls    int
}

func (d *formulaDriver) Formulas(context.Context) ([]string, error) {
	d.calls++
	return d.formulas, d.err
}

// Backend pins the tests to a Gas Town machine. The embedded driver is
// whatever SelectDriver picked on the host, and on a machine with `gc`
// installed that is Gas City, which orchestratorAvailable() treats as an
// orchestrator regardless of gtEnv.Available.
func (*formulaDriver) Backend() string { return gastown.BackendGasTown }

var installed = []string{"mol-polecat-work", "shiny", "security-audit"}

// choiceJudge answers the formula question from a fixed distribution.
func choiceJudge(probs map[string]float64) *fakeJudge {
	best, bestP := "", -1.0
	for k, p := range probs {
		if p > bestP {
			best, bestP = k, p
		}
	}
	return &fakeJudge{answer: func(key string, q jev.Question) (jev.Answer, bool) {
		if key != "formula" || q.Type != jev.Choice {
			return jev.Answer{}, false
		}
		return jev.Answer{Type: jev.Choice, Choice: best, Probabilities: probs, Confidence: 0.85}, true
	}}
}

// newFormulaModel is a model with Jev on and a Gas Town-shaped orchestrator
// whose formula list is fixed.
func newFormulaModel(t *testing.T, judge jev.Evaluator, drv *formulaDriver) Model {
	t.Helper()
	issues := []data.Issue{testIssue("mg-1", data.StatusOpen), testIssue("mg-2", data.StatusOpen), testIssue("mg-3", data.StatusClosed)}
	issues[0].Title = "Add authentication middleware"
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	// The first resize starts a formula-list fetch when the host has an
	// orchestrator; its Cmd is dropped here, so forget it.
	m.jevFormula = jevFormula{}
	m.jev.client = judge
	m.jev.now = time.Now
	m.gtEnv.Available = true
	m.gtPollInFlight = true
	m.patrolScanInFlight = true
	drv.Driver = m.driver // everything but Formulas() behaves as the real driver
	m.driver = drv
	return m
}

// suggest drives one selection-change cycle to completion: list fetch,
// debounce tick, request, verdict. It returns the model after the verdict.
func suggest(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.scheduleFormulaSuggest()
	for cmd != nil {
		msg := cmd()
		if _, ok := msg.(formulaVerdictMsg); ok {
			model, _ := m.Update(msg)
			return model.(Model)
		}
		model, next := m.Update(msg)
		m, cmd = model.(Model), next
	}
	return m
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFormulaSuggestOffWithoutJevOrOrchestrator(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "")
	drv := &formulaDriver{formulas: installed}
	m := New([]data.Issue{testIssue("mg-1", data.StatusOpen)}, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	drv.Driver = m.driver
	m.driver = drv
	m.gtEnv.Available = true
	if m.scheduleFormulaSuggest() != nil || drv.calls != 0 {
		t.Fatal("Jev off: no fetch, no suggestion")
	}

	m = newFormulaModel(t, &fakeJudge{}, drv)
	m.gtEnv.Available = false
	if m.scheduleFormulaSuggest() != nil || drv.calls != 0 {
		t.Fatal("no orchestrator: formulas cannot run, so nothing to suggest")
	}
}

func TestFormulaSuggestRanksInstalledFormulas(t *testing.T) {
	judge := choiceJudge(map[string]float64{"shiny": 0.7, "mol-polecat-work": 0.2, "security-audit": 0.1})
	drv := &formulaDriver{formulas: installed}
	m := newFormulaModel(t, judge, drv)

	m = suggest(t, m)

	if drv.calls != 1 || judge.count() != 1 {
		t.Fatalf("driver calls %d, judge calls %d", drv.calls, judge.count())
	}
	call := judge.calls[0]
	q := call.questions["formula"]
	if q.Type != jev.Choice {
		t.Fatalf("question = %+v", q)
	}
	state := call.state.(map[string]any)
	if snap, ok := state["issue"].(data.IssueSnapshot); !ok || snap.ID != "mg-1" {
		t.Fatalf("state = %+v", state)
	}
	if !strings.Contains(string(mustJSON(t, q.Criteria)), `"shiny":"`+gastown.FormulaDescriptions["shiny"]) {
		t.Fatalf("options should carry descriptions: %s", mustJSON(t, q.Criteria))
	}

	if m.detail.FormulaIssueID != "mg-1" || len(m.detail.FormulaRecs) != 3 || m.detail.FormulaRecs[0].Formula != "shiny" {
		t.Fatalf("detail recs = %+v", m.detail.FormulaRecs)
	}
	if m.detail.FormulaRecs[0].P != 0.7 {
		t.Fatalf("top probability = %v", m.detail.FormulaRecs[0].P)
	}

	// Same issue again: cached, no second request, ranking still shown.
	m.detail.SetFormulaRecs("", nil)
	if cmd := m.scheduleFormulaSuggest(); cmd != nil {
		t.Fatal("a cached ranking must not be re-asked")
	}
	if judge.count() != 1 || m.detail.FormulaRecs == nil {
		t.Fatalf("calls %d, recs %v", judge.count(), m.detail.FormulaRecs)
	}
}

func TestFormulaSuggestEdgeCaseWeakVerdictKeepsHeuristic(t *testing.T) {
	judge := choiceJudge(map[string]float64{"shiny": 0.35, "mol-polecat-work": 0.33, "security-audit": 0.32})
	m := newFormulaModel(t, judge, &formulaDriver{formulas: installed})
	m = suggest(t, m)
	if m.detail.FormulaRecs != nil {
		t.Fatalf("a best below %.0f%% should leave the heuristic, got %+v", formulaMinProb*100, m.detail.FormulaRecs)
	}
	if cmd := m.scheduleFormulaSuggest(); cmd != nil {
		t.Fatal("a weak verdict is cached too; no re-ask until the issue changes")
	}
}

func TestFormulaSuggestEdgeCaseJevErrorFeedsCircuit(t *testing.T) {
	judge := &fakeJudge{err: errors.New("jev: dial tcp: connection refused")}
	m := newFormulaModel(t, judge, &formulaDriver{formulas: installed})
	m = suggest(t, m)
	if m.detail.FormulaRecs != nil || m.jev.health.ConsecFailures != 1 {
		t.Fatalf("recs %v, failures %d", m.detail.FormulaRecs, m.jev.health.ConsecFailures)
	}
	if m.jevFormula.asking["mg-1"] {
		t.Fatal("a failed ask must release the in-flight mark")
	}
}

func TestFormulaSuggestEdgeCaseFormulaFetchFails(t *testing.T) {
	judge := choiceJudge(map[string]float64{"shiny": 0.9})
	drv := &formulaDriver{err: errors.New("gt formula list: exit 1")}
	m := newFormulaModel(t, judge, drv)
	m = suggest(t, m)
	if judge.count() != 0 || m.jevFormula.fetching {
		t.Fatalf("no list means no question; judge calls %d fetching %v", judge.count(), m.jevFormula.fetching)
	}
	// The next selection change retries the fetch.
	drv.err, drv.formulas = nil, installed
	if m.scheduleFormulaSuggest() == nil {
		t.Fatal("expected a retry of the formula fetch")
	}
}

func TestFormulaSuggestEdgeCaseDebounceDropsStaleTick(t *testing.T) {
	judge := choiceJudge(map[string]float64{"shiny": 0.9})
	m := newFormulaModel(t, judge, &formulaDriver{formulas: installed})
	m.jevFormula.formulas, m.jevFormula.fetchedAt = installed, time.Now()
	m.jevFormula.cache, m.jevFormula.asking = map[string]formulaEntry{}, map[string]bool{}

	_ = m.scheduleFormulaSuggest() // tick for mg-1, gen 1
	m.parade.MoveDown()
	m.syncSelection()
	second := m.scheduleFormulaSuggest() // tick for mg-2, gen 2

	model, cmd := m.Update(formulaAskMsg{issueID: "mg-1", gen: 1})
	if cmd != nil {
		t.Fatal("a superseded tick must not ask")
	}
	m = model.(Model)
	model, cmd = m.Update(second())
	if cmd == nil {
		t.Fatal("the latest tick should ask")
	}
	if !model.(Model).jevFormula.asking["mg-2"] {
		t.Fatal("expected mg-2 to be marked in flight")
	}
}

func TestFormulaPickerPutsRankedFirst(t *testing.T) {
	judge := choiceJudge(map[string]float64{"shiny": 0.7, "mol-polecat-work": 0.2, "security-audit": 0.1})
	m := newFormulaModel(t, judge, &formulaDriver{formulas: installed})
	m = suggest(t, m)

	m.formulaTarget = "mg-1"
	model, cmd := m.Update(formulaListMsg{formulas: []string{"mol-polecat-work", "security-audit", "shiny", "custom-flow"}})
	m = model.(Model)
	if !m.formulaPicking || !m.showPalette || cmd == nil {
		t.Fatal("expected the picker to open")
	}
	if m.palette.SelectedName() != "shiny" {
		t.Fatalf("picker should start on the ranked best, got %q", m.palette.SelectedName())
	}
	cmds := m.formulaPickerCommands([]string{"mol-polecat-work", "security-audit", "shiny", "custom-flow"}, "mg-1")
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	if strings.Join(names, ",") != "shiny,mol-polecat-work,security-audit,custom-flow" {
		t.Fatalf("order = %v", names)
	}
	if !strings.Contains(cmds[0].Desc, "jev 70%") || cmds[3].Desc != "Formula" {
		t.Fatalf("descs = %q / %q", cmds[0].Desc, cmds[3].Desc)
	}
	if m.jevFormula.formulas[3] != "custom-flow" {
		t.Fatal("the picker's list should refresh the cached installed list")
	}
}

func TestFormulaPickerEdgeCaseNoRankingKeepsOrder(t *testing.T) {
	m := newFormulaModel(t, &fakeJudge{}, &formulaDriver{formulas: installed})
	cmds := m.formulaPickerCommands(installed, "mg-1")
	for i, c := range cmds {
		if c.Name != installed[i] || c.Desc != "Formula" {
			t.Fatalf("without a ranking the picker must be unchanged: %+v", cmds)
		}
	}
	// Multi-select has no single target: list order too.
	cmds = m.formulaPickerCommands(installed, "")
	if cmds[0].Name != installed[0] {
		t.Fatalf("multi-select picker reordered: %+v", cmds)
	}
}

func TestFormulaVerdictWhileModalOpen(t *testing.T) {
	m := newFormulaModel(t, choiceJudge(map[string]float64{"shiny": 0.9}), &formulaDriver{formulas: installed})
	m.jevFormula.cache, m.jevFormula.asking = map[string]formulaEntry{}, map[string]bool{"mg-1": true}
	m.showPalette = true
	model, _ := m.Update(formulaVerdictMsg{issueID: "mg-1", hash: "h", recs: []gastown.FormulaRecommendation{{Formula: "shiny", P: 0.9}}})
	got := model.(Model)
	if got.jevFormula.asking["mg-1"] || len(got.jevFormula.cache) != 1 {
		t.Fatal("a verdict behind the palette must still land")
	}
}

func TestFormulaSuggestEdgeCaseClosedIssue(t *testing.T) {
	drv := &formulaDriver{formulas: installed}
	m := newFormulaModel(t, &fakeJudge{}, drv)
	m.parade.MoveToBottom()
	m.syncSelection()
	if sel := m.parade.SelectedIssue; sel == nil || sel.Status != data.StatusClosed {
		t.Skip("fixture: expected the closed issue at the bottom of the parade")
	}
	if m.scheduleFormulaSuggest() != nil || drv.calls != 0 {
		t.Fatal("closed issues get no formula")
	}
}

func TestJevProbeAsksAboutStartupSelection(t *testing.T) {
	// The issue selected at startup never changed selection, so it never got
	// a formula pick until you moved off it and back (mg-xge.5).
	drv := &formulaDriver{formulas: installed}
	m := newFormulaModel(t, choiceJudge(map[string]float64{"shiny": 0.9}), drv)
	_, cmd := m.Update(jevProbeMsg{})
	if cmd == nil {
		t.Fatal("a successful probe should start the formula pick")
	}
	runBatch(cmd)
	if drv.calls == 0 {
		t.Fatal("the probe's command should fetch the installed formulas")
	}
}
