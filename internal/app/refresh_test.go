package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// newJSONLRefreshModel builds a model watching a real JSONL file, so the
// refresh loop has a source to fetch from.
func newJSONLRefreshModel(t *testing.T, issues []data.Issue) Model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "issues.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, iss := range issues {
		if err := enc.Encode(iss); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return New(issues, data.Source{Mode: data.SourceJSONL, Path: path}, data.DefaultBlockingTypes)
}

func TestRequestRefresh(t *testing.T) {
	m := newJSONLRefreshModel(t, []data.Issue{testIssue("a", data.StatusOpen)})

	if cmd := m.requestRefresh(); cmd == nil {
		t.Fatal("expected a fetch Cmd when nothing is in flight")
	}
	if !m.refresh.inFlight {
		t.Fatal("expected a fetch in flight")
	}
	if m.refresh.tickGen != 1 {
		t.Fatalf("expected the armed timer to be retired (tickGen 1), got %d", m.refresh.tickGen)
	}
}

func TestRequestRefreshWhileInFlight(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := newJSONLRefreshModel(t, issues)
	m.requestRefresh()

	// A burst of requests during one fetch coalesces into a single follow-up.
	for i := 0; i < 100; i++ {
		if cmd := m.requestRefresh(); cmd != nil {
			t.Fatalf("request %d: expected no new fetch while one is in flight", i)
		}
	}
	if !m.refresh.dirty {
		t.Fatal("expected the loop to be marked dirty")
	}

	model, _ := m.Update(refreshResultMsg{gen: m.refresh.fetchGen, msg: data.FileChangedMsg{Issues: issues}})
	got := model.(Model)
	if !got.refresh.inFlight || got.refresh.dirty {
		t.Fatalf("expected exactly one follow-up fetch, got inFlight=%v dirty=%v", got.refresh.inFlight, got.refresh.dirty)
	}

	// The follow-up lands with nothing pending: back to the timer.
	model, _ = got.Update(refreshResultMsg{gen: got.refresh.fetchGen, msg: data.FileUnchangedMsg{}})
	got = model.(Model)
	if got.refresh.inFlight || got.refresh.dirty {
		t.Fatalf("expected the loop to be idle on its timer, got inFlight=%v dirty=%v", got.refresh.inFlight, got.refresh.dirty)
	}
}

// TestRefreshMutationsKeepOneLoop is the regression test for the poll-chain
// leak: every mutation used to start another self-re-arming poll.
func TestRefreshMutationsKeepOneLoop(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	m := newJSONLRefreshModel(t, issues)

	var model any = m
	for i := 0; i < 3; i++ {
		model, _ = model.(Model).Update(mutateResultMsg{issueID: "a", action: "claimed"})
	}
	got := model.(Model)
	if !got.refresh.inFlight {
		t.Fatal("expected the first mutation to start a fetch")
	}

	// First fetch lands, the coalesced follow-up runs, then it lands too.
	for i := 0; i < 2; i++ {
		model, _ = got.Update(refreshResultMsg{gen: got.refresh.fetchGen, msg: data.FileChangedMsg{Issues: issues}})
		got = model.(Model)
	}
	if got.refresh.inFlight {
		t.Fatal("expected the loop to be back on its timer")
	}

	// Only the newest timer may start a fetch; every older one is retired.
	live := got.refresh.tickGen
	for gen := uint64(0); gen < live; gen++ {
		model, cmd := got.Update(refreshTickMsg{gen: gen})
		if cmd != nil || model.(Model).refresh.inFlight {
			t.Fatalf("stale tick gen %d started a fetch (live gen %d)", gen, live)
		}
	}
	model, cmd := got.Update(refreshTickMsg{gen: live})
	if cmd == nil || !model.(Model).refresh.inFlight {
		t.Fatal("expected the live tick to start a fetch")
	}
}

func TestRefreshTickWhileInFlight(t *testing.T) {
	m := newJSONLRefreshModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.requestRefresh()

	model, cmd := m.Update(refreshTickMsg{gen: m.refresh.tickGen})
	if cmd != nil {
		t.Fatal("expected no second fetch while one is in flight")
	}
	if !model.(Model).refresh.inFlight {
		t.Fatal("expected the original fetch to stay in flight")
	}
}

func TestRequestRefreshNoWatchPath(t *testing.T) {
	m := New(nil, data.Source{Mode: data.SourceJSONL}, data.DefaultBlockingTypes)
	if cmd := m.requestRefresh(); cmd != nil {
		t.Fatal("expected no fetch without a file to watch")
	}
	if m.refresh.inFlight {
		t.Fatal("a fetch that never started must not leave the loop in flight")
	}
}

func TestFileWatchErrorMsgContinuesLoop(t *testing.T) {
	m := newJSONLRefreshModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	m.requestRefresh()

	model, _ := m.Update(refreshResultMsg{gen: m.refresh.fetchGen, msg: data.FileWatchErrorMsg{Err: errors.New("boom")}})
	got := model.(Model)
	if got.refresh.inFlight {
		t.Fatal("expected the failed fetch to end")
	}
	if got.refresh.tickGen <= m.refresh.tickGen {
		t.Fatal("expected a fresh timer after the failure")
	}
}

func TestCLIHealthCheckMsgRecovery(t *testing.T) {
	before := []data.Issue{testIssue("a", data.StatusOpen), testIssue("b", data.StatusOpen)}
	m := newJSONLRefreshModel(t, before)
	m.sourceHealth = data.SourceHealth{State: data.HealthRecovering, ConsecSuccesses: 1}
	m.healthChecking = true

	// A JSONL fetch is in flight when bd recovers.
	m.requestRefresh()
	staleGen := m.refresh.fetchGen

	after := []data.Issue{testIssue("a", data.StatusClosed), testIssue("b", data.StatusOpen)}
	model, _ := m.Update(data.CLIHealthCheckMsg{Issues: after})
	got := model.(Model)

	if got.sourceMode != data.SourceCLI {
		t.Fatal("expected recovery to switch back to CLI")
	}
	if got.refresh.inFlight {
		t.Fatal("expected the JSONL fetch to be abandoned")
	}
	if got.prevIssueMap["a"] != data.StatusClosed {
		t.Fatalf("expected recovery to refresh the diff snapshot, got %q", got.prevIssueMap["a"])
	}
	if !got.changedIDs["a"] {
		t.Fatal("expected recovery to mark the issue that changed while in fallback")
	}

	// The abandoned JSONL result lands late and must not overwrite CLI data.
	model, _ = got.Update(refreshResultMsg{gen: staleGen, msg: data.FileChangedMsg{Issues: before}})
	got = model.(Model)
	if got.prevIssueMap["a"] != data.StatusClosed {
		t.Fatal("stale JSONL result was applied over recovered CLI data")
	}
}
