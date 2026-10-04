package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// Formula choice over the live list: the second Jev feature. The FORMULA
// section of the detail panel and the `s` picker both suggest a workflow
// formula for the selected issue. The heuristic (gastown.RecommendFormulas)
// matches words in the title against a fixed vocabulary, so it can suggest a
// formula that is not installed while the picker lists the ones that are.
// With Jev on, mg fetches the installed formulas once, asks Jev to pick one
// for the selected issue, shows the ranking in the detail panel and puts it
// first in the picker. Without a verdict both read exactly as before.

const (
	// formulaDebounce lets j/k scrolling settle before an issue is asked about.
	formulaDebounce = 250 * time.Millisecond
	// formulaListTTL is how long the installed-formula list is trusted.
	formulaListTTL = 5 * time.Minute
	// formulaMinProb is the least probability on the best formula for the
	// ranking to replace the heuristic.
	formulaMinProb = 0.4
	// formulaAskTimeout bounds one request.
	formulaAskTimeout = 3 * time.Second
)

// jevFormula holds the installed formulas and the per-issue rankings.
type jevFormula struct {
	formulas  []string
	fetchedAt time.Time
	fetching  bool
	cache     map[string]formulaEntry // issue ID -> ranking and what it answers
	gen       uint64                  // debounce generation: only the latest tick asks
	asking    map[string]bool         // issue IDs with a request out
}

type formulaEntry struct {
	hash string // snapshot hash + formula list hash
	recs []gastown.FormulaRecommendation
}

// formulaCacheMsg carries a fresh installed-formula list for the cache.
type formulaCacheMsg struct {
	formulas []string
	err      error
}

// formulaAskMsg fires after the debounce for the issue selected at the time.
type formulaAskMsg struct {
	issueID string
	gen     uint64
}

// formulaVerdictMsg is Jev's ranking for one issue.
type formulaVerdictMsg struct {
	issueID string
	hash    string
	recs    []gastown.FormulaRecommendation
	tokens  int
	err     error
}

// formulaSuggestable reports whether the selected issue can be asked about:
// Jev on and not paused, an orchestrator to run formulas, an open issue.
func (m Model) formulaSuggestable() bool {
	return m.jev.enabled() && m.jev.health.Level() < 2 && m.orchestratorAvailable()
}

// scheduleFormulaSuggest runs on selection change. It shows a cached ranking
// at once, fetches the formula list when it is missing or stale, and
// otherwise starts the debounce that leads to a request.
func (m *Model) scheduleFormulaSuggest() tea.Cmd {
	sel := m.parade.SelectedIssue
	if sel == nil || !m.formulaSuggestable() || sel.Status == data.StatusClosed {
		return nil
	}
	f := &m.jevFormula
	if f.cache == nil {
		f.cache = make(map[string]formulaEntry)
		f.asking = make(map[string]bool)
	}
	if e, ok := f.cache[sel.ID]; ok && e.hash == m.formulaHash(*sel) {
		m.detail.SetFormulaRecs(sel.ID, e.recs)
		return nil
	}
	if len(f.formulas) == 0 || m.jev.now().Sub(f.fetchedAt) > formulaListTTL {
		if f.fetching {
			return nil
		}
		f.fetching = true
		driver := m.driver
		return func() tea.Msg {
			formulas, err := driver.Formulas(context.Background())
			return formulaCacheMsg{formulas: formulas, err: err}
		}
	}
	if f.asking[sel.ID] {
		return nil
	}
	f.gen++
	gen, id := f.gen, sel.ID
	return tea.Tick(formulaDebounce, func(time.Time) tea.Msg { return formulaAskMsg{issueID: id, gen: gen} })
}

// formulaHash keys a ranking by what it was computed from: the issue as
// sent, and the list it chose from.
func (m Model) formulaHash(iss data.Issue) string {
	snap := data.SnapshotForJudge(iss, iss.EvaluateDependencies(data.BuildIssueMap(m.issues), m.blockingTypes), m.jev.scope, m.jev.now())
	sum := sha256.Sum256([]byte(snap.Hash() + "|" + strings.Join(m.jevFormula.formulas, ",")))
	return hex.EncodeToString(sum[:8])
}

// handleFormulaCache stores the installed formulas and asks about the
// selected issue. A failed fetch is retried on the next selection change.
func (m Model) handleFormulaCache(msg formulaCacheMsg) (tea.Model, tea.Cmd) {
	m.jevFormula.fetching = false
	if msg.err != nil || len(msg.formulas) == 0 {
		return m, nil
	}
	m.jevFormula.formulas = msg.formulas
	m.jevFormula.fetchedAt = m.jev.now()
	cmd := m.scheduleFormulaSuggest() // before the return copies m
	return m, cmd
}

// handleFormulaAsk sends the request if the issue is still selected and
// this is the latest debounce tick.
func (m Model) handleFormulaAsk(msg formulaAskMsg) (tea.Model, tea.Cmd) {
	sel := m.parade.SelectedIssue
	if msg.gen != m.jevFormula.gen || sel == nil || sel.ID != msg.issueID || !m.formulaSuggestable() {
		return m, nil
	}
	if e, ok := m.jevFormula.cache[sel.ID]; ok && e.hash == m.formulaHash(*sel) {
		return m, nil
	}
	m.jevFormula.asking[sel.ID] = true
	return m, formulaAskCmd(m.jev.client, m.jev.scope, *sel, m.issues, m.blockingTypes, m.jevFormula.formulas, m.formulaHash(*sel), m.jev.now())
}

// formulaAskCmd asks Jev to pick one of the installed formulas for iss.
func formulaAskCmd(client jev.Evaluator, scope data.SnapshotScope, iss data.Issue, all []data.Issue, blockingTypes map[string]bool, formulas []string, hash string, now time.Time) tea.Cmd {
	snap := data.SnapshotForJudge(iss, iss.EvaluateDependencies(data.BuildIssueMap(all), blockingTypes), scope, now)
	opts := make([]jev.Option, len(formulas))
	for i, f := range formulas {
		opts[i] = jev.Option{Name: f, Desc: gastown.DescribeFormula(f)}
	}
	q := jev.NewChoice("Which workflow formula best fits this issue? Match the scope and the risk: "+
		"security or auth work wants an audit-style formula; a large feature wants the full lifecycle; "+
		"a small fix or chore wants the minimal lifecycle; high priority wants parallel review.", opts...)
	state := map[string]any{"issue": snap}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), formulaAskTimeout)
		defer cancel()
		res, err := client.Evaluate(ctx, state, map[string]jev.Question{"formula": q})
		msg := formulaVerdictMsg{issueID: iss.ID, hash: hash, err: err}
		if err != nil {
			return msg
		}
		msg.tokens = res.Usage.InputTokens
		a, ok := res.Answers["formula"]
		if !ok {
			return msg
		}
		probs := a.Probabilities
		if len(probs) == 0 && a.Choice != "" {
			probs = map[string]float64{a.Choice: a.Confidence}
		}
		msg.recs = gastown.RankFormulas(formulas, probs, formulaMinProb)
		return msg
	}
}

// handleFormulaVerdict caches the ranking and shows it if the issue is
// still selected. An empty ranking is cached too, so a weak verdict is not
// re-asked until the issue changes.
func (m Model) handleFormulaVerdict(msg formulaVerdictMsg) (tea.Model, tea.Cmd) {
	delete(m.jevFormula.asking, msg.issueID)
	m.jev.tokens += msg.tokens
	cmd := m.jevRecord(msg.err)
	if msg.err != nil {
		return m, cmd
	}
	m.jevFormula.cache[msg.issueID] = formulaEntry{hash: msg.hash, recs: msg.recs}
	if sel := m.parade.SelectedIssue; sel != nil && sel.ID == msg.issueID {
		m.detail.SetFormulaRecs(msg.issueID, msg.recs)
	}
	return m, cmd
}

// formulaPickerCommands builds the `s` picker's entries from the installed
// formulas, best first when Jev has ranked them for issueID.
func (m Model) formulaPickerCommands(formulas []string, issueID string) []components.PaletteCommand {
	byName := make(map[string]gastown.FormulaRecommendation)
	if e, ok := m.jevFormula.cache[issueID]; ok && issueID != "" {
		for _, r := range e.recs {
			byName[r.Formula] = r
		}
	}
	var ranked, rest []components.PaletteCommand
	for _, f := range formulas {
		cmd := components.PaletteCommand{Name: f, Desc: "Formula", Action: components.ActionFormulaSelect}
		if r, ok := byName[f]; ok {
			cmd.Desc = strings.TrimSpace(r.Reason + " · jev " + percent(r.P))
			ranked = append(ranked, cmd)
			continue
		}
		rest = append(rest, cmd)
	}
	// ranked keeps the cache's best-first order because the cache's recs
	// are sorted; re-sort by P to be safe.
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && byName[ranked[j].Name].P > byName[ranked[j-1].Name].P; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}
	return append(ranked, rest...)
}

func percent(p float64) string { return fmt.Sprintf("%.0f%%", p*100) }
