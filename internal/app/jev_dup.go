package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// Duplicate check on create: the first Jev feature. When the create form
// submits, mg picks the few existing issues whose titles overlap the new
// one, asks Jev whether each is the same piece of work, and only then runs
// bd create. A strong match opens a dialog (jump to it, create anyway, or
// create and mark the duplicate); a weak one adds a hint to the create
// toast; anything else, including Jev being off or slow, creates exactly as
// before. The user already pressed enter, so the check has a short budget
// and never blocks the create for long.

const (
	// dupCheckTimeout bounds the whole check; past it the issue is created.
	dupCheckTimeout = 1500 * time.Millisecond
	// dupMaxCandidates caps the issues sent with one check.
	dupMaxCandidates = 8
	// dupDialogThreshold opens the dialog; dupHintThreshold adds a toast hint.
	dupDialogThreshold = 0.8
	dupHintThreshold   = 0.5
	// dupMinConfidence is the least confidence a verdict needs to act on.
	dupMinConfidence = 0.6
	// dupDepType is the bd dependency type the dialog's link action adds.
	dupDepType = "duplicates"
)

// Seams for tests, which cannot run bd.
var (
	createIssue        = data.CreateIssue
	addDependencyTyped = data.AddDependencyTyped
)

// dupVerdict is Jev's answer about one candidate.
type dupVerdict struct {
	issue      data.Issue
	prob       float64
	confidence float64
}

// dupCheckMsg carries the check's outcome back with the create it was for.
type dupCheckMsg struct {
	create   components.CreateFormResult
	verdicts []dupVerdict // best first
	tokens   int
	err      error
}

// dupCheckable reports whether a create should be checked: Jev is on, the
// circuit is not open or disabled, and a question would make sense.
func (m Model) dupCheckable() bool {
	return m.jev.enabled() && m.jev.health.Level() < 2
}

// submitCreate is the create form's result: check for duplicates when that
// is possible, otherwise create as before.
func (m Model) submitCreate(result components.CreateFormResult) (tea.Model, tea.Cmd) {
	if !m.dupCheckable() {
		return m, m.createCmd(result, "created")
	}
	cands := data.DuplicateCandidates(m.issues, result.Title, m.jev.now(), dupMaxCandidates)
	if len(cands) == 0 {
		return m, m.createCmd(result, "created")
	}
	toast, toastCmd := components.ShowToast("Checking for duplicates…", components.ToastInfo, toastDuration)
	m.toast = toast
	return m, tea.Batch(toastCmd, dupCheckCmd(m.jev.client, m.jev.scope, result, cands, m.issues, m.blockingTypes, m.jev.now()))
}

// createCmd runs the create the form asked for. action is the toast's verb.
func (m Model) createCmd(result components.CreateFormResult, action string) tea.Cmd {
	title := result.Title
	if result.CrewMember != "" {
		crew := result.CrewMember
		driver := m.driver
		return func() tea.Msg {
			// Assign returns gt's output, not an issue ID, so this path
			// names the issue by its title.
			_, err := driver.Assign(context.Background(), crew, title, result.Type, result.Priority, "", true)
			return mutateResultMsg{issueID: title, action: fmt.Sprintf("assigned to %s", crew), err: err}
		}
	}
	issueType := data.IssueType(result.Type)
	priority := components.ParsePriority(result.Priority)
	return func() tea.Msg {
		id, err := createIssue(title, issueType, priority)
		return createdResult(id, title, action, err)
	}
}

// createdResult reports a create by the new issue's ID, so the toast names
// it and the parade selects it once the reload lands (mg-xow). The title
// stands in when bd did not return an ID.
func createdResult(id, title, action string, err error) mutateResultMsg {
	if err != nil || id == "" {
		return mutateResultMsg{issueID: title, action: action, err: err}
	}
	return mutateResultMsg{issueID: id, action: action, createdID: id}
}

// createLinkedCmd creates the issue and marks it a duplicate of target.
func (m Model) createLinkedCmd(result components.CreateFormResult, target string) tea.Cmd {
	title := result.Title
	issueType := data.IssueType(result.Type)
	priority := components.ParsePriority(result.Priority)
	return func() tea.Msg {
		id, err := createIssue(title, issueType, priority)
		if err != nil {
			return mutateResultMsg{issueID: title, action: "created", err: err}
		}
		if err := addDependencyTyped(id, target, dupDepType); err != nil {
			return mutateResultMsg{issueID: id, action: "mark duplicate of " + target, err: err}
		}
		return mutateResultMsg{issueID: id, action: "created as duplicate of " + target, createdID: id}
	}
}

// dupCheckCmd asks Jev, for each candidate, whether the new issue is the
// same work. The state carries the new issue's title, type and priority and
// the candidates' redacted snapshots; nothing else.
func dupCheckCmd(client jev.Evaluator, scope data.SnapshotScope, result components.CreateFormResult, cands, all []data.Issue, blockingTypes map[string]bool, now time.Time) tea.Cmd {
	issueMap := data.BuildIssueMap(all)
	snaps := make([]data.IssueSnapshot, len(cands))
	qs := make(map[string]jev.Question, len(cands))
	for i, c := range cands {
		snaps[i] = data.SnapshotForJudge(c, c.EvaluateDependencies(issueMap, blockingTypes), scope, now)
		qs["dup/"+c.ID] = jev.NewNoul(fmt.Sprintf(
			"Is NEW the same piece of work as the existing issue with id %s, such that creating NEW would be a duplicate? "+
				"The same symptom or the same deliverable counts even when worded differently. "+
				"A sub-task of the existing issue, or a follow-up to it, is not a duplicate.", c.ID))
	}
	state := map[string]any{
		"new": map[string]any{
			"title":    data.ScrubSecrets(result.Title),
			"type":     result.Type,
			"priority": result.Priority,
		},
		"existing": snaps,
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dupCheckTimeout)
		defer cancel()
		res, err := client.Evaluate(ctx, state, qs)
		msg := dupCheckMsg{create: result, err: err}
		if err != nil {
			return msg
		}
		msg.tokens = res.Usage.InputTokens
		for _, c := range cands {
			if a, ok := res.Answers["dup/"+c.ID]; ok {
				msg.verdicts = append(msg.verdicts, dupVerdict{issue: c, prob: a.Noul, confidence: a.Confidence})
			}
		}
		sort.SliceStable(msg.verdicts, func(i, j int) bool { return msg.verdicts[i].prob > msg.verdicts[j].prob })
		return msg
	}
}

// handleDupCheck decides what the verdicts mean for the pending create.
func (m Model) handleDupCheck(msg dupCheckMsg) (tea.Model, tea.Cmd) {
	m.jev.tokens += msg.tokens
	var cmds []tea.Cmd
	if cmd := m.jevRecord(msg.err); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if msg.err != nil || len(msg.verdicts) == 0 {
		return m, tea.Batch(append(cmds, m.createCmd(msg.create, "created"))...)
	}
	top := msg.verdicts[0]
	switch {
	case top.prob >= dupDialogThreshold && top.confidence >= dupMinConfidence:
		var cands []components.DuplicateCandidate
		for _, v := range msg.verdicts {
			if v.prob < dupHintThreshold && len(cands) > 0 {
				break
			}
			cands = append(cands, components.DuplicateCandidate{ID: v.issue.ID, Title: v.issue.Title, Status: string(v.issue.Status), Prob: v.prob})
		}
		m.pendingCreate = &msg.create
		m.dupDialog = components.NewDuplicateDialog(msg.create.Title, cands, min(m.width-8, 72)-4)
		m.dupDialogOpen = true
		m.toast = components.Toast{} // the "checking" toast has done its job
		return m, tea.Batch(cmds...)
	case top.prob >= dupHintThreshold:
		action := fmt.Sprintf("created · similar to %s (%.0f%%)", top.issue.ID, top.prob*100)
		return m, tea.Batch(append(cmds, m.createCmd(msg.create, action))...)
	default:
		return m, tea.Batch(append(cmds, m.createCmd(msg.create, "created"))...)
	}
}

// handleDupDialogResult acts on the user's choice and closes the dialog.
func (m Model) handleDupDialogResult(msg components.DuplicateDialogResult) (tea.Model, tea.Cmd) {
	m.dupDialogOpen = false
	pending := m.pendingCreate
	m.pendingCreate = nil
	if pending == nil {
		return m, nil
	}
	switch msg.Action {
	case components.DuplicateJump:
		if m.restoreParadeSelection(msg.TargetID) {
			m.syncSelection()
			return m, nil
		}
		toast, cmd := components.ShowToast(fmt.Sprintf("%s is not in the parade right now", msg.TargetID), components.ToastInfo, toastDuration)
		m.toast = toast
		return m, cmd
	case components.DuplicateCreate:
		return m, m.createCmd(*pending, "created")
	case components.DuplicateLink:
		if pending.CrewMember != "" || msg.TargetID == "" {
			return m, m.createCmd(*pending, "created")
		}
		return m, m.createLinkedCmd(*pending, msg.TargetID)
	default:
		return m, nil
	}
}
