package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

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

func TestJournalBackstopWithOrchestrator(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.gtEnv = gastown.Env{Available: true}
	if got := m.refreshInterval(); got != data.CLIPollInterval {
		t.Fatalf("interval %v, want the legacy %v under an orchestrator", got, data.CLIPollInterval)
	}
}

func TestJournalProbeTriggersReload(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := liveModel(t, issues)
	m.refresh.startedAt = time.Now().Add(-time.Minute) // the last reload was long ago

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
	m.refresh.startedAt = time.Now() // a reload just started and landed

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
	m.refresh.startedAt = time.Now().Add(-time.Minute)
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
	m.refresh.startedAt = time.Now().Add(-time.Minute)

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

	got.refresh.startedAt = time.Now().Add(-time.Minute)
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

func TestRefreshAfterMutationProbesFirst(t *testing.T) {
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)})

	model, _ := m.Update(mutateResultMsg{issueID: "a", action: "claimed"})
	got := model.(Model)
	if !got.journal.busy || got.refresh.inFlight {
		t.Fatalf("busy %v inFlight %v, want a probe before the reload", got.journal.busy, got.refresh.inFlight)
	}

	// The probe returns mg's own record: exactly one reload, right away.
	model, _ = got.Update(journalProbeMsg{records: []data.JournalRecord{probeRecord(11, "a")}, afterMutation: true})
	got = model.(Model)
	if !got.refresh.inFlight || got.refresh.dirty {
		t.Fatalf("inFlight %v dirty %v, want exactly one reload", got.refresh.inFlight, got.refresh.dirty)
	}
	if got.journal.follower.Anchor.Seq != 11 {
		t.Fatal("expected the anchor past mg's own record")
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
