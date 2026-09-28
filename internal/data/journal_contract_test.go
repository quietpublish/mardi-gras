//go:build bdcontract

package data

// Contract test against a real bd binary: `make contract-bd BD=/path/to/bd`.
//
// The unit tests decode fixtures captured from bd; this replays the journal
// scenarios mg depends on against the binary itself, so a bd upgrade that
// changes the contract fails here instead of in someone's terminal. It runs
// only in a throwaway workspace under t.TempDir() with its own HOME and
// TMPDIR, because bd migrates a workspace's schema on first use and that
// cannot be undone.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// contractWorkspace initializes a scratch bd workspace and points this
// process (cwd, PATH, HOME) at it, so mg's own client code runs the real
// binary there.
func contractWorkspace(t *testing.T) (bd func(args ...string) string) {
	t.Helper()
	bin := os.Getenv("MG_BD_CONTRACT_BIN")
	if bin == "" {
		t.Skip("set MG_BD_CONTRACT_BIN to a bd binary (make contract-bd BD=...)")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	for _, dir := range []string{ws, filepath.Join(root, "home"), filepath.Join(root, "tmp"), filepath.Join(root, "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for dir := filepath.Dir(ws); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".beads")); err == nil {
			t.Fatalf("refusing: %s has a .beads; bd could reach a real workspace from here", dir)
		}
	}
	// PATH holds only this bd (as "bd") and the system tools bd needs.
	if err := os.Symlink(bin, filepath.Join(root, "bin", "bd")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(root, "bin")+":/usr/bin:/bin")
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("BD_DISABLE_METRICS", "1")
	t.Setenv("BD_NON_INTERACTIVE", "1")
	t.Setenv("BD_EVENTS_JOURNAL", "")
	os.Unsetenv("BD_EVENTS_JOURNAL")
	t.Chdir(ws)

	bd = func(args ...string) string {
		t.Helper()
		out, err := exec.Command("bd", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("bd %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Contract Test"},
		{"config", "user.email", "contract@example.com"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	bd("init", "--non-interactive", "-p", "ct")
	return bd
}

// createIssue creates an issue and returns its ID.
func createIssue(t *testing.T, bd func(...string) string, args ...string) string {
	t.Helper()
	out := bd(append([]string{"create"}, append(args, "--silent")...)...)
	id := strings.TrimSpace(out)
	if id == "" || strings.ContainsAny(id, " \n") {
		t.Fatalf("bd create %v printed %q, want just an ID", args, out)
	}
	return id
}

func TestBdContractJournal(t *testing.T) {
	bd := contractWorkspace(t)
	t.Logf("bd under test: %s", strings.TrimSpace(bd("version")))

	// Off by default, and a tail of a disabled journal is an empty success.
	if on, err := JournalEnabled(); err != nil || on {
		t.Fatalf("JournalEnabled() = %v, %v; want off by default", on, err)
	}
	if _, err := TailJournal(0, 1); errors.Is(err, ErrJournalUnsupported) {
		t.Skipf("this bd has no events journal; client reports %v as expected", err)
	} else if err != nil {
		t.Fatalf("TailJournal on a disabled journal: %v", err)
	}

	bd("config", "set", "events-journal", "true")
	if on, err := JournalEnabled(); err != nil || !on {
		t.Fatalf("JournalEnabled() = %v, %v; want on", on, err)
	}
	t.Run("env override", func(t *testing.T) {
		t.Setenv("BD_EVENTS_JOURNAL", "0")
		if on, err := JournalEnabled(); err != nil || on {
			t.Fatalf("JournalEnabled() with BD_EVENTS_JOURNAL=0 = %v, %v; want off", on, err)
		}
	})

	a := createIssue(t, bd, "Alpha", "-t", "task")
	b := createIssue(t, bd, "Bravo", "-t", "task")

	head, err := FindJournalHead()
	if err != nil || head.Seq != 2 || head.Ref == nil || head.Ref.IssueID != b {
		t.Fatalf("FindJournalHead() = %+v, %v; want seq 2 on %s", head, err, b)
	}
	if records, err := ProbeJournal(head, JournalProbeLimit); err != nil || len(records) != 0 {
		t.Fatalf("probe at head = %v, %v; want caught up", records, err)
	}

	// Each op mg keys on, with the shape it decodes.
	bd("update", a, "--title", "Alpha renamed")
	bd("dep", "add", b, a)
	bd("close", a)
	wisp := createIssue(t, bd, "Wisp", "-t", "task", "--ephemeral")
	bd("delete", b, "--force")

	records, err := ProbeJournal(head, JournalProbeLimit)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	var ops []string
	for i, r := range records {
		if r.Seq != head.Seq+int64(i)+1 || r.TS == "" {
			t.Errorf("record %d = %+v; want contiguous seqs and a timestamp", i, r)
		}
		ops = append(ops, r.Op+":"+r.IssueID)
	}
	has := func(op, id string) bool {
		for _, o := range ops {
			if o == op+":"+id {
				return true
			}
		}
		return false
	}
	for _, want := range [][2]string{{"update", a}, {"dep_add", b}, {"close", a}, {"create", wisp}, {"delete", b}} {
		if !has(want[0], want[1]) {
			t.Errorf("no %s record for %s in %v", want[0], want[1], ops)
		}
	}
	for _, r := range records {
		switch {
		case r.IssueID == wisp && r.Op == "create" && !r.Ephemeral():
			t.Errorf("wisp create not marked ephemeral: %+v", r)
		case r.Op == "delete" && r.Issue != nil:
			t.Errorf("delete record carries an issue: %+v", r)
		case r.Op != "delete" && r.Issue == nil:
			t.Errorf("%s record has no issue: %+v", r.Op, r)
		}
	}

	// bd list must still parse, and wisps stay out of it.
	issues, err := FetchIssuesCLI(".")
	if err != nil {
		t.Fatalf("FetchIssuesCLI: %v", err)
	}
	for _, iss := range issues {
		if iss.ID == wisp {
			t.Errorf("bd list returned the wisp %s; mg relies on it not doing so", wisp)
		}
	}

	// Prune everything, anchor included: truncation is typed, the head search
	// anchors without a fingerprint, and a probe from there reads as caught up.
	last := records[len(records)-1]
	bd("config", "set", "events-journal-retain-days", "0")
	bd("config", "set", "events-journal-retain-rows", "0")
	bd("events", "prune", "--before", strconv.FormatInt(last.Seq+1, 10))

	_, err = TailJournal(0, 1)
	var trunc *JournalTruncatedError
	if !errors.As(err, &trunc) || trunc.Head != last.Seq || trunc.Floor != last.Seq+1 {
		t.Fatalf("TailJournal after a full prune = %v; want truncation with head %d, floor %d", err, last.Seq, last.Seq+1)
	}
	anchor := JournalAnchor{Seq: last.Seq, Ref: &last}
	if _, err := ProbeJournal(anchor, JournalProbeLimit); !errors.As(err, &trunc) {
		t.Fatalf("probe from a pruned anchor = %v; want truncation", err)
	}
	pruned, err := FindJournalHead()
	if err != nil || pruned.Seq != last.Seq || pruned.Ref != nil {
		t.Fatalf("FindJournalHead after a full prune = %+v, %v; want seq %d, no fingerprint", pruned, err, last.Seq)
	}
	if records, err := ProbeJournal(pruned, JournalProbeLimit); err != nil || len(records) != 0 {
		t.Fatalf("probe of a still-pruned journal = %v, %v; want caught up", records, err)
	}

	// A write made with the journal switched off leaves no record and no gap.
	t.Run("unjournaled write", func(t *testing.T) {
		t.Setenv("BD_EVENTS_JOURNAL", "0")
		bd("update", a, "--title", "Alpha, quietly")
	})
	if records, err := ProbeJournal(pruned, JournalProbeLimit); err != nil || len(records) != 0 {
		t.Fatalf("after an unjournaled write: %v, %v; want no record", records, err)
	}
	bd("update", a, "--priority", "1")
	records, err = ProbeJournal(pruned, JournalProbeLimit)
	if err != nil || len(records) != 1 || records[0].Seq != last.Seq+1 {
		t.Fatalf("after a journaled write: %+v, %v; want one record at seq %d", records, err, last.Seq+1)
	}
}
