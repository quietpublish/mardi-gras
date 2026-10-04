package app

import (
	"context"
	"os"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// Jev, when the operator enables it (MG_JEV_API_KEY), is a fast judge mg
// can ask typed questions about issues: a calibrated yes/no, a pick, a
// score. This file is the plumbing: one loop that asks about the issues a
// reload changed, caches the verdicts by what was sent, and keeps the
// circuit breaker and the footer chip honest. Nothing here decides anything
// in the UI; a feature registers a question set and reads the cached
// answers, and every such feature must render identically when the cache is
// empty, because that is what Jev being off, slow or broken looks like.
//
// The loop never bypasses the cache: a full backlog costs a fraction of a
// cent once, but re-asking on every 5s poll would cost dollars a day.

const (
	// jevChunkSize is how many issues share one request. Per-issue requests
	// would be one round trip each; one giant request would fail as a unit.
	jevChunkSize = 25
	// jevParallel bounds the chunk requests in flight for one sweep.
	jevParallel = 4
)

// jevQuestionSet builds the questions to ask about one issue, keyed by a
// short name. The loop scopes each question to its issue; the names come
// back as the keys of the cached answers. Nil means nothing is asked.
type jevQuestionSet func(snap data.IssueSnapshot) map[string]jev.Question

// jevLoop owns mg's side of the conversation with Jev.
type jevLoop struct {
	client    jev.Evaluator // nil when Jev is off
	scope     data.SnapshotScope
	questions jevQuestionSet      // nil until a feature registers one
	cache     map[string]jevEntry // issue ID -> the verdicts and what they answer
	inFlight  bool                // single-flight: one sweep at a time
	dirty     bool                // a reload landed mid-sweep: sweep again when it lands
	gen       uint64              // identifies the sweep in flight; stale results are dropped
	health    jev.Health
	tokens    int              // input tokens this session, for the meter
	now       func() time.Time // for tests
}

// jevEntry is one issue's cached verdicts, with the hash of the snapshot
// they answer. A matching hash means the issue would be sent identically.
type jevEntry struct {
	hash    string
	answers map[string]jev.Answer // question name -> answer
}

// jevVerdictsMsg is the result of one sweep. results holds every issue that
// got an answer; err is the first chunk failure, if any chunk failed.
type jevVerdictsMsg struct {
	gen     uint64
	results map[string]jevEntry
	tokens  int
	err     error
}

// jevProbeMsg is the result of the startup probe that checks the key and
// the endpoint before anything else is sent.
type jevProbeMsg struct {
	err    error
	tokens int
}

// newJevLoop reads the environment. A misconfiguration (bad URL) leaves Jev
// off; main has already warned about it.
func newJevLoop() jevLoop {
	j := jevLoop{cache: make(map[string]jevEntry), now: time.Now}
	client, err := jev.FromEnv()
	if err != nil || client == nil {
		return j
	}
	j.client = client
	j.scope, _ = data.ParseSnapshotScope(os.Getenv(jev.EnvScope))
	return j
}

func (j jevLoop) enabled() bool { return j.client != nil }

// jevFooter is the footer chip, or nil when Jev is off.
func (m Model) jevFooter() *components.JevStatus {
	if !m.jev.enabled() {
		return nil
	}
	return &components.JevStatus{Label: m.jev.health.Label(), Level: m.jev.health.Level()}
}

// jevAnswers returns the cached verdicts for an issue, or nil. Features
// read their answers here and must treat nil as "no opinion".
func (m Model) jevAnswers(issueID string) map[string]jev.Answer {
	if e, ok := m.jev.cache[issueID]; ok {
		return e.answers
	}
	return nil
}

// jevProbe asks one trivial question at startup so a rejected key or a
// wrong URL is reported at once, not when the first feature needs it.
func (m Model) jevProbe() tea.Cmd {
	client := m.jev.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*jev.Timeout())
		defer cancel()
		res, err := client.Evaluate(ctx,
			map[string]any{"probe": "mg startup"},
			map[string]jev.Question{"ok": jev.NewNoul("Is this a connectivity probe?")})
		msg := jevProbeMsg{err: err}
		if res != nil {
			msg.tokens = res.Usage.InputTokens
		}
		return msg
	}
}

// handleJevProbe records the probe and, when it passed, starts the first
// sweep. A failed probe leaves that to the next reload.
func (m Model) handleJevProbe(msg jevProbeMsg) (tea.Model, tea.Cmd) {
	m.jev.tokens += msg.tokens
	var cmds []tea.Cmd
	if cmd := m.jevRecord(msg.err); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if msg.err == nil {
		if cmd := m.scheduleJev(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// jevRecord feeds a call's outcome to the circuit and toasts its notice,
// if it raised one. Successes are silent.
func (m *Model) jevRecord(err error) tea.Cmd {
	now := m.jev.now()
	if err != nil {
		m.jev.health = m.jev.health.RecordFailure(err, now)
	} else {
		m.jev.health = m.jev.health.RecordSuccess(now)
	}
	notice := m.jev.health.Notice
	if notice == "" {
		return nil
	}
	m.jev.health.Notice = ""
	toast, cmd := components.ShowToast(notice, components.ToastWarn, toastDuration)
	m.toast = toast
	return cmd
}

// scheduleJev starts a sweep over the issues whose snapshot has changed
// since they were last asked about. It returns nil when Jev is off, no
// feature has registered questions, nothing changed, a sweep is already
// out (the reload is remembered as dirty), or the circuit is open.
func (m *Model) scheduleJev() tea.Cmd {
	j := &m.jev
	if j.client == nil || j.questions == nil {
		return nil
	}
	if j.inFlight {
		j.dirty = true
		return nil
	}
	now := j.now()
	work := j.workSet(m.issues, m.blockingTypes, now)
	if len(work) == 0 {
		return nil
	}
	var ok bool
	if j.health, ok = j.health.Allow(now); !ok {
		return nil
	}
	if j.health.State == jev.HealthHalfOpen {
		// One chunk probes a paused circuit; the rest waits for its verdict.
		work = work[:min(len(work), jevChunkSize)]
	}
	j.inFlight, j.dirty = true, false
	j.gen++
	return jevSweepCmd(j.client, j.questions, work, j.gen)
}

// workSet snapshots every non-closed issue and keeps the ones the cache
// cannot answer. It also forgets issues that are gone.
func (j *jevLoop) workSet(issues []data.Issue, blockingTypes map[string]bool, now time.Time) []data.IssueSnapshot {
	issueMap := data.BuildIssueMap(issues)
	present := make(map[string]bool, len(issues))
	var work []data.IssueSnapshot
	for i := range issues {
		iss := &issues[i]
		present[iss.ID] = true
		if iss.Status == data.StatusClosed {
			continue
		}
		snap := data.SnapshotForJudge(*iss, iss.EvaluateDependencies(issueMap, blockingTypes), j.scope, now)
		if e, ok := j.cache[iss.ID]; ok && e.hash == snap.Hash() {
			continue
		}
		work = append(work, snap)
	}
	for id := range j.cache {
		if !present[id] {
			delete(j.cache, id)
		}
	}
	return work
}

// jevSweepCmd asks about work in chunks, a few at a time, and returns one
// message when every chunk has reported.
func jevSweepCmd(client jev.Evaluator, questions jevQuestionSet, work []data.IssueSnapshot, gen uint64) tea.Cmd {
	return func() tea.Msg {
		msg := jevVerdictsMsg{gen: gen, results: make(map[string]jevEntry, len(work))}
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, jevParallel)
		for start := 0; start < len(work); start += jevChunkSize {
			chunk := work[start:min(start+jevChunkSize, len(work))]
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				results, tokens, err := jevAskChunk(client, questions, chunk)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					if msg.err == nil {
						msg.err = err
					}
					return
				}
				msg.tokens += tokens
				for id, e := range results {
					msg.results[id] = e
				}
			}()
		}
		wg.Wait()
		return msg
	}
}

// jevAskChunk sends one request about a chunk of issues. The state is the
// list of snapshots; each question is scoped to its issue by name and by
// a prefix on its instructions, and keyed "<id>/<name>" on the wire.
func jevAskChunk(client jev.Evaluator, questions jevQuestionSet, chunk []data.IssueSnapshot) (results map[string]jevEntry, tokens int, err error) {
	type ref struct{ id, name string }
	qs := make(map[string]jev.Question)
	refs := make(map[string]ref)
	results = make(map[string]jevEntry, len(chunk))
	for _, snap := range chunk {
		results[snap.ID] = jevEntry{hash: snap.Hash(), answers: make(map[string]jev.Answer)}
		for name, q := range questions(snap) {
			key := snap.ID + "/" + name
			q.Instructions = "About the issue with id " + snap.ID + ": " + q.Instructions
			qs[key] = q
			refs[key] = ref{snap.ID, name}
		}
	}
	if len(qs) == 0 {
		// Nothing to ask about these; cache them so they are not re-walked.
		return results, 0, nil
	}
	res, err := client.Evaluate(context.Background(), map[string]any{"issues": chunk}, qs)
	if err != nil {
		return nil, 0, err
	}
	for key, a := range res.Answers {
		if r, ok := refs[key]; ok {
			results[r.id].answers[r.name] = a
		}
	}
	return results, res.Usage.InputTokens, nil
}

// handleJevVerdicts lands a sweep: closes the gate, merges the cache, feeds
// the circuit, and sweeps again if a reload arrived meanwhile. Features
// that render verdicts refresh their views from here.
func (m Model) handleJevVerdicts(msg jevVerdictsMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.jev.gen {
		return m, nil
	}
	m.jev.inFlight = false
	for id, e := range msg.results {
		m.jev.cache[id] = e
	}
	m.jev.tokens += msg.tokens
	var cmds []tea.Cmd
	if cmd := m.jevRecord(msg.err); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(msg.results) > 0 {
		m.applyFocusVerdicts()
	}
	if m.jev.dirty {
		if cmd := m.scheduleJev(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}
