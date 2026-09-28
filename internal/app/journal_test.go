package app

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

// newCLIModel builds a bd-backed model with no orchestrator, whatever is
// installed on the machine running the tests. Its commands are never run.
func newCLIModel(t *testing.T, issues []data.Issue) Model {
	t.Helper()
	t.Setenv("MG_EVENTS", "")
	m := New(issues, data.Source{Mode: data.SourceCLI, ProjectDir: t.TempDir()}, data.DefaultBlockingTypes)
	m.gtEnv = gastown.Env{}
	m.driver = gastown.NewGTDriver()
	// New may have kept the journal off for an orchestrator it found here.
	m.journal = newJournalLoop(data.SourceCLI, false)
	return m
}

// liveModel walks a CLI model through detection and head search to live,
// with the baseline reload landed.
func liveModel(t *testing.T, issues []data.Issue) Model {
	t.Helper()
	m := newCLIModel(t, issues)
	model, _ := m.Update(journalEnabledMsg{on: true})
	ref := data.JournalRecord{Seq: 10, TS: "t", Op: "update", IssueID: "x"}
	model, _ = model.(Model).Update(journalHeadMsg{anchor: data.JournalAnchor{Seq: 10, Ref: &ref}})
	got := model.(Model)
	if !got.journal.follower.Live() {
		t.Fatalf("setup: phase %v, want following", got.journal.follower.Phase)
	}
	model, _ = got.Update(refreshResultMsg{gen: got.refresh.fetchGen, msg: data.FileChangedMsg{Issues: issues}})
	return model.(Model)
}

func probeRecord(seq int64, id string) data.JournalRecord {
	return data.JournalRecord{Seq: seq, TS: "t", Op: "update", IssueID: id}
}

func TestNewJournalLoop(t *testing.T) {
	cli := newJournalLoop(data.SourceCLI, false)
	if cli.follower.Phase != data.JournalDetecting || !cli.busy {
		t.Errorf("CLI: phase %v busy %v, want detecting with the check in flight", cli.follower.Phase, cli.busy)
	}
	jsonl := newJournalLoop(data.SourceJSONL, false)
	if jsonl.follower.Phase != data.JournalOff || jsonl.busy {
		t.Errorf("JSONL: phase %v busy %v, want off", jsonl.follower.Phase, jsonl.busy)
	}
	opted := newJournalLoop(data.SourceCLI, true)
	if opted.follower.Phase != data.JournalOff || opted.busy {
		t.Errorf("MG_EVENTS=off: phase %v busy %v, want off", opted.follower.Phase, opted.busy)
	}
}

func TestJournalOptedOut(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"off", true},
		{"OFF", true},
		{"  off\n", true},
		{"auto", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Setenv("MG_EVENTS", tt.value)
		if got := journalOptedOut(); got != tt.want {
			t.Errorf("MG_EVENTS=%q: got %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestJournalGoesLive(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := newCLIModel(t, issues)
	if got := m.refreshInterval(); got != data.CLIPollInterval {
		t.Fatalf("before live: interval %v, want %v", got, data.CLIPollInterval)
	}

	model, cmd := m.Update(journalEnabledMsg{on: true})
	got := model.(Model)
	if got.journal.follower.Phase != data.JournalBaselining || !got.journal.busy || cmd == nil {
		t.Fatalf("enabled: phase %v busy %v, want a head search in flight", got.journal.follower.Phase, got.journal.busy)
	}

	model, _ = got.Update(journalHeadMsg{anchor: data.JournalAnchor{Seq: 3}})
	got = model.(Model)
	if !got.journal.follower.Live() {
		t.Fatalf("head found: phase %v, want following", got.journal.follower.Phase)
	}
	if !got.refresh.inFlight {
		t.Fatal("expected going live to reload, so everything up to the anchor is on screen")
	}
	if got.refreshInterval() != journalBackstopInterval {
		t.Fatalf("live: interval %v, want the %v backstop", got.refreshInterval(), journalBackstopInterval)
	}
}

func TestJournalDisabledStaysPolling(t *testing.T) {
	m := newCLIModel(t, nil)
	model, cmd := m.Update(journalEnabledMsg{on: false})
	got := model.(Model)
	if got.journal.follower.Phase != data.JournalOff || got.journal.busy {
		t.Fatalf("phase %v busy %v, want off and idle", got.journal.follower.Phase, got.journal.busy)
	}
	if cmd == nil {
		t.Fatal("expected a timer to re-check the journal later")
	}
	if got.refreshInterval() != data.CLIPollInterval {
		t.Fatal("expected the legacy poll while the journal is off")
	}
}

func TestNewJournalOffUnderOrchestrator(t *testing.T) {
	t.Setenv("MG_EVENTS", "")
	fakeGT, err := filepath.Abs("../../testdata") // testdata/gt: a fake Gas Town
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeGT)
	m := New(nil, data.Source{Mode: data.SourceCLI, ProjectDir: t.TempDir()}, data.DefaultBlockingTypes)
	if m.journal.follower.Phase != data.JournalOff || m.journal.busy {
		t.Fatalf("phase %v busy %v: orchestrator writes can bypass the journal, so it stays off", m.journal.follower.Phase, m.journal.busy)
	}
}

func TestRefreshIntervalWhileFailing(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.sourceHealth = m.sourceHealth.RecordFailure(errors.New("dolt: connection refused"))
	if got := m.refreshInterval(); got != data.CLIPollInterval {
		t.Fatalf("interval %v while bd list fails, want retries at %v", got, data.CLIPollInterval)
	}
}

func TestJournalProbeTriggersReload(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := liveModel(t, issues)
	m.refresh.landedAt = time.Now().Add(-time.Minute) // the last reload was long ago

	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got := model.(Model)
	if got.journal.follower.Anchor.Seq != 11 {
		t.Fatalf("anchor %d, want 11", got.journal.follower.Anchor.Seq)
	}
	if !got.refresh.inFlight {
		t.Fatal("expected a record for an issue on screen to reload now")
	}
	if _, ok := got.journal.pending["a"]; !ok {
		t.Fatal("expected the record's issue to be pending until a reload shows it")
	}
}

func TestJournalProbeSpacesReloads(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.refresh.landedAt = time.Now() // a reload just landed

	model, cmd := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got := model.(Model)
	if got.refresh.inFlight {
		t.Fatal("expected a busy journal not to reload more often than the old poll")
	}
	if cmd == nil || got.refresh.dueAt.After(time.Now().Add(data.CLIPollInterval)) {
		t.Fatalf("expected the reload timer pulled in to within %v, due %v", data.CLIPollInterval, got.refresh.dueAt)
	}
}

func TestJournalProbeIgnoresWisps(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.refresh.landedAt = time.Now().Add(-time.Minute)
	var wisp data.JournalRecord
	line := `{"seq":11,"ts":"t","op":"create","issue_id":"fx-wisp-1","issue":{"id":"fx-wisp-1","ephemeral":true}}`
	if err := json.Unmarshal([]byte(line), &wisp); err != nil {
		t.Fatal(err)
	}

	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{wisp}})
	got := model.(Model)
	if got.refresh.inFlight {
		t.Fatal("a wisp never reaches bd list, so it must not reload")
	}
	if got.journal.follower.Anchor.Seq != 11 {
		t.Fatal("expected the anchor to move past the wisp")
	}
}

func TestJournalLearnsHiddenIssues(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := liveModel(t, issues)
	m.refresh.landedAt = time.Now().Add(-time.Minute)

	// A record for an issue bd list never returns (a template, say).
	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "tmpl-1")}})
	got := model.(Model)
	if !got.refresh.inFlight {
		t.Fatal("expected an unknown issue to reload once")
	}
	model, _ = got.Update(refreshResultMsg{gen: got.refresh.fetchGen, msg: data.FileChangedMsg{Issues: issues}})
	got = model.(Model)
	if !got.journal.ignore["tmpl-1"] {
		t.Fatal("expected an issue the reload did not return to be ignored from now on")
	}

	got.refresh.landedAt = time.Now().Add(-time.Minute)
	model, _ = got.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(12, "tmpl-1")}})
	if model.(Model).refresh.inFlight {
		t.Fatal("expected an ignored issue's records not to reload")
	}
}

func TestJournalPendingWaitsForLaterReload(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := liveModel(t, issues)

	// A reload is in flight when a create is probed: that reload may predate
	// the write, so it must not conclude bd list hides the new issue.
	m.requestRefresh()
	m.journal.pending = map[string]time.Time{"new-1": time.Now().Add(time.Second)}
	model, _ := m.Update(refreshResultMsg{gen: m.refresh.fetchGen, msg: data.FileChangedMsg{Issues: issues}})
	got := model.(Model)
	if got.journal.ignore["new-1"] {
		t.Fatal("an issue created after the reload started must not be ignored")
	}
	if _, ok := got.journal.pending["new-1"]; !ok {
		t.Fatal("expected the issue to stay pending for the next reload")
	}
}

func TestRefreshAfterMutation(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)}
	m := liveModel(t, issues)
	m.refresh.landedAt = time.Now().Add(-time.Minute)

	model, _ := m.Update(mutateResultMsg{issueID: "a", action: "claimed"})
	got := model.(Model)
	if !got.refresh.inFlight {
		t.Fatal("expected mg's own write to reload right away, not wait on a probe")
	}
	if !got.journal.busy {
		t.Fatal("expected a probe alongside, to move the anchor past mg's own record")
	}

	// The probe returns mg's own record: the reload under way covers it.
	model, _ = got.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got = model.(Model)
	if got.refresh.dirty || got.refresh.spaced {
		t.Fatal("expected mg's own record not to ask for a second reload")
	}
	if got.journal.follower.Anchor.Seq != 11 {
		t.Fatal("expected the anchor past mg's own record")
	}

	// Another writer's record still counts.
	model, _ = got.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(12, "b")}})
	if got = model.(Model); !got.refresh.spaced {
		t.Fatal("expected another writer's record to ask for a reload")
	}
}

func TestRefreshAfterMutationOwnRecordConsumed(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.journal.own = map[string]time.Time{"a": time.Now()}
	m.refresh.landedAt = time.Now().Add(-time.Minute)

	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got := model.(Model)
	if got.refresh.inFlight {
		t.Fatal("expected mg's own record to be covered")
	}
	// A later write to the same issue, by someone else, reloads.
	model, _ = got.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(12, "a")}})
	if !model.(Model).refresh.inFlight {
		t.Fatal("expected only the one record to be covered")
	}
}

func TestJournalProbeSpacedWhileInFlight(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.requestRefresh() // a reload is in flight

	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got := model.(Model)
	if got.refresh.dirty || !got.refresh.spaced {
		t.Fatalf("dirty %v spaced %v: want the request spaced, not an immediate follow-up", got.refresh.dirty, got.refresh.spaced)
	}

	// The reload lands: the next one waits CLIPollInterval rather than
	// starting back to back.
	model, _ = got.Update(refreshResultMsg{gen: got.refresh.fetchGen, msg: data.FileChangedMsg{Issues: got.issues}})
	got = model.(Model)
	if got.refresh.inFlight {
		t.Fatal("expected no back-to-back reload")
	}
	if due := time.Until(got.refresh.dueAt); due > data.CLIPollInterval || due <= 0 {
		t.Fatalf("next reload due in %v, want within %v", due, data.CLIPollInterval)
	}
}

func TestJournalTransitionKeepsPulledInTimer(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.refresh.landedAt = time.Now().Add(-2 * time.Second)

	// A record pulls the next reload in to ~3s.
	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}})
	got := model.(Model)
	pulledIn := got.refresh.dueAt

	// Then partial decays off, changing the backstop interval: the pulled
	// in reload must not be pushed out to 30s.
	got.journal.follower.Partial = true
	before := got.journal.follower
	got.journal.follower.Partial = false
	got.journalTransition(before)
	if got.refresh.dueAt.After(pulledIn) {
		t.Fatalf("reload pushed from %v to %v", pulledIn, got.refresh.dueAt)
	}
}

func TestJournalProbeIgnoresWispDelete(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.refresh.landedAt = time.Now().Add(-time.Minute)
	var create, del data.JournalRecord
	for line, rec := range map[string]*data.JournalRecord{
		`{"seq":11,"ts":"t","op":"create","issue_id":"w-1","issue":{"id":"w-1","ephemeral":true}}`: &create,
		`{"seq":12,"ts":"t","op":"delete","issue_id":"w-1","issue":null}`:                          &del,
	} {
		if err := json.Unmarshal([]byte(line), rec); err != nil {
			t.Fatal(err)
		}
	}
	model, _ := m.Update(journalProbeMsg{records: []data.JournalRecord{create}})
	model, _ = model.(Model).Update(journalProbeMsg{records: []data.JournalRecord{del}})
	if model.(Model).refresh.inFlight {
		t.Fatal("a wisp's delete record (issue null) must not reload")
	}
}

func TestJournalReloadedSkipsFallback(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.journal.pending = map[string]time.Time{"x": time.Now().Add(-time.Minute)}
	m.sourceHealth.State = data.HealthFallback // reloading from issues.jsonl
	m.journalReloaded(nil, []string{"a"})
	if m.journal.ignore["x"] || len(m.journal.pending) != 1 {
		t.Fatal("a JSONL fallback reload must not feed the journal's bookkeeping")
	}
}

func TestRefreshAfterMutationWhenNotLive(t *testing.T) {
	m := newCLIModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.journal = journalLoop{} // off
	model, _ := m.Update(mutateResultMsg{issueID: "a", action: "claimed"})
	if !model.(Model).refresh.inFlight {
		t.Fatal("expected a plain reload when the journal is not live")
	}
}

func TestJournalBackoffToast(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	boom := errors.New("timeout")
	var model any = m
	for range 3 {
		got := model.(Model)
		got.journal.busy = true
		model, _ = got.Update(journalProbeMsg{err: boom})
	}
	got := model.(Model)
	if got.journal.follower.Phase != data.JournalBackoff {
		t.Fatalf("phase %v, want backoff", got.journal.follower.Phase)
	}
	if !got.toast.Active() {
		t.Fatal("expected a toast when live updates stop")
	}
	if got.refreshInterval() != data.CLIPollInterval {
		t.Fatal("expected the legacy poll while backing off")
	}
}

func TestJournalTickSkipsWhileBusy(t *testing.T) {
	m := liveModel(t, nil)
	m.journal.busy = true
	if _, cmd := m.Update(journalTickMsg{gen: m.journal.gen}); cmd != nil {
		t.Fatal("expected no second journal call while one is in flight")
	}
	m.journal.busy = false
	if _, cmd := m.Update(journalTickMsg{gen: m.journal.gen - 1}); cmd != nil {
		t.Fatal("expected a stale journal tick to be dropped")
	}
	model, cmd := m.Update(journalTickMsg{gen: m.journal.gen})
	if cmd == nil || !model.(Model).journal.busy {
		t.Fatal("expected the live tick to probe")
	}
}

func TestDataFreshAt(t *testing.T) {
	m := liveModel(t, nil)
	m.lastFileMod = time.Now().Add(-25 * time.Second)
	m.journal.freshAt = time.Now()
	if !m.dataFreshAt().Equal(m.journal.freshAt) {
		t.Fatal("expected a clean probe to count as fresh data while live")
	}
	m.journal.follower.Partial = true
	if !m.dataFreshAt().Equal(m.lastFileMod) {
		t.Fatal("expected only bd list to count while the journal is partial")
	}
}

func wispRecord(seq int64, id string) data.JournalRecord {
	return data.JournalRecord{Seq: seq, TS: "t", Op: "create", IssueID: id, Issue: &data.JournalIssue{Ephemeral: true}}
}

func TestJournalAddRecent(t *testing.T) {
	var j journalLoop
	j.addRecent([]data.JournalRecord{probeRecord(1, "a"), wispRecord(2, "w"), probeRecord(3, "b")})
	j.addRecent([]data.JournalRecord{probeRecord(3, "b"), probeRecord(4, "a")}) // 3 again: a re-read anchor
	var seqs []int64
	for _, r := range j.recent {
		seqs = append(seqs, r.Seq)
	}
	if len(seqs) != 3 || seqs[0] != 1 || seqs[1] != 3 || seqs[2] != 4 {
		t.Fatalf("seqs = %v, want [1 3 4]: no wisps, no repeats", seqs)
	}
	if got := j.recentFor("a"); len(got) != 2 {
		t.Fatalf("recentFor(a) = %d records, want 2", len(got))
	}

	var many []data.JournalRecord
	for i := int64(10); i < 10+journalRecentCap+50; i++ {
		many = append(many, probeRecord(i, "x"))
	}
	j.addRecent(many)
	if len(j.recent) != journalRecentCap || j.recent[len(j.recent)-1].Seq != 10+journalRecentCap+49 {
		t.Fatalf("len %d, newest %d: want the newest %d kept", len(j.recent), j.recent[len(j.recent)-1].Seq, journalRecentCap)
	}
}

func TestJournalAddBackfill(t *testing.T) {
	var j journalLoop
	j.addRecent([]data.JournalRecord{probeRecord(50, "a")})
	j.addBackfill([]data.JournalRecord{probeRecord(48, "b"), wispRecord(49, "w"), probeRecord(50, "a")})
	if len(j.recent) != 2 || j.recent[0].Seq != 48 || j.recent[1].Seq != 50 {
		t.Fatalf("recent = %+v, want 48 then 50", j.recent)
	}
}

func TestJournalProbeFeedsRecentChanges(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)}
	m := liveModel(t, issues)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	sel := m.parade.SelectedIssue.ID

	model, _ = m.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, sel), probeRecord(12, "other")}})
	got := model.(Model)
	if len(got.journal.recent) != 2 {
		t.Fatalf("feed has %d records, want 2", len(got.journal.recent))
	}
	if len(got.detail.RecentChanges) != 1 || got.detail.RecentChanges[0].IssueID != sel {
		t.Fatalf("detail changes = %+v, want the selected issue's record", got.detail.RecentChanges)
	}
}

func TestJournalBackfillMsg(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	model, _ := m.Update(journalBackfillMsg{records: []data.JournalRecord{probeRecord(5, "a"), probeRecord(6, "a")}})
	if got := model.(Model); len(got.journal.recent) != 2 {
		t.Fatalf("feed has %d records after backfill, want 2", len(got.journal.recent))
	}
}

func TestKeyEOpensRecentChanges(t *testing.T) {
	m := newCLIModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.startedAt = time.Now().Add(-time.Second)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	m.showDoctor = true

	model, _ = m.Update(tea.KeyPressMsg{Code: 'E', Text: "E"})
	got := model.(Model)
	if !got.showChanges || got.showDoctor {
		t.Fatalf("showChanges %v showDoctor %v: want E to open it and close the doctor", got.showChanges, got.showDoctor)
	}
	model, _ = got.Update(tea.KeyPressMsg{Code: 'D', Text: "D"})
	if got = model.(Model); got.showChanges {
		t.Fatal("expected D to close Recent changes")
	}
}

func TestJournalNote(t *testing.T) {
	m := newCLIModel(t, nil)
	m.journal = journalLoop{} // off: the journal is disabled here
	if note := m.journalNote(); !strings.Contains(note, "bd config set events-journal true") {
		t.Errorf("off: note %q should say how to turn it on", note)
	}
	if live := liveModel(t, nil); live.journalNote() != "" {
		t.Errorf("live: note %q, want none", live.journalNote())
	}
	jsonl := New(nil, data.Source{Mode: data.SourceJSONL, Path: "x.jsonl"}, data.DefaultBlockingTypes)
	if note := jsonl.journalNote(); !strings.Contains(note, "issues.jsonl") {
		t.Errorf("JSONL: note %q should say mg isn't using bd", note)
	}
}
