package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Fixtures captured from bd 1.3.0 (`bd events tail --json`) in a scratch
// workspace, trimmed to the records that exercise each shape: a create, a
// dep_add that blocks its issue, a comment, an actor-less derived update (the
// blocked-state flip bd emits before the close that caused it), a wisp, and a
// delete.
const bdEventsTailFixture = `{"seq":1,"ts":"2026-09-27T21:02:54Z","op":"create","issue_id":"fx-m1g","actor":"Fixture User","issue":{"id":"fx-m1g","title":"Alpha","status":"open","priority":2,"issue_type":"task","owner":"fixture@example.com","created_at":"2026-09-27T21:02:54Z","created_by":"Fixture User","updated_at":"2026-09-27T21:02:54Z"}}
{"seq":3,"ts":"2026-09-27T21:02:56Z","op":"dep_add","issue_id":"fx-9qm","actor":"Fixture User","issue":{"id":"fx-9qm","title":"Bravo","status":"open","priority":1,"issue_type":"task","is_blocked":true,"owner":"fixture@example.com","created_at":"2026-09-27T21:02:55Z","created_by":"Fixture User","updated_at":"2026-09-27T21:02:55Z"},"dep":{"kind":"blocks","target":"fx-m1g","metadata":"{}"}}
{"seq":7,"ts":"2026-09-27T21:02:58Z","op":"comment","issue_id":"fx-m1g","actor":"Fixture User","issue":{"id":"fx-m1g","title":"Alpha renamed","status":"in_progress","priority":2,"issue_type":"task","assignee":"Fixture User","labels":["backend"]},"comment":{"id":"37941748-0f87-5771-9fa0-e10e2febc57f","author":"Fixture User","text":"first comment","created_at":"2026-09-27T21:02:58Z","source":"structured"}}
{"seq":8,"ts":"2026-09-27T21:02:59Z","op":"update","issue_id":"fx-9qm","issue":{"id":"fx-9qm","title":"Bravo","status":"open","priority":1,"issue_type":"task"}}
{"seq":10,"ts":"2026-09-27T21:03:01Z","op":"create","issue_id":"fx-wisp-fng","actor":"Fixture User","issue":{"id":"fx-wisp-fng","title":"Wisp","status":"open","priority":2,"issue_type":"task","ephemeral":true}}
{"seq":12,"ts":"2026-09-27T21:03:03Z","op":"delete","issue_id":"fx-9qm","actor":"Fixture User","issue":null}
`

// bd 1.3.0 reports a pruned-past checkpoint as pretty-printed JSON on
// stdout, exiting 1.
const bdEventsTruncatedFixture = `{
  "code": "events_journal_truncated",
  "error": "events journal truncated: checkpoint 2 is below the retained window [6..12]; records 3..5 were pruned",
  "floor": 6,
  "head": 12,
  "schema_version": 1,
  "since": 2
}
`

// bd 1.1.0 has no events command.
const bdEventsUnknownCommandStderr = "Error: unknown command \"events\" for \"bd\"\nRun 'bd --help' for usage.\n"

func TestParseJournalRecords(t *testing.T) {
	records, err := parseJournalRecords([]byte(bdEventsTailFixture))
	if err != nil {
		t.Fatal(err)
	}
	var seqs []int64
	for _, r := range records {
		seqs = append(seqs, r.Seq)
	}
	if !slices.Equal(seqs, []int64{1, 3, 7, 8, 10, 12}) {
		t.Fatalf("seqs = %v", seqs)
	}
	if r := records[0]; r.Op != "create" || r.IssueID != "fx-m1g" || r.Actor != "Fixture User" || r.TS != "2026-09-27T21:02:54Z" {
		t.Errorf("create record = %+v", r)
	}
	if records[3].Actor != "" {
		t.Errorf("derived update actor = %q, want empty", records[3].Actor)
	}
	if !records[4].Ephemeral() || records[0].Ephemeral() {
		t.Error("expected only the wisp record to be ephemeral")
	}
	if del := records[5]; del.Op != "delete" || del.Issue != nil || del.Ephemeral() {
		t.Errorf("delete record = %+v, want a nil issue", del)
	}
}

func TestParseJournalRecordsEmpty(t *testing.T) {
	records, err := parseJournalRecords(nil)
	if err != nil || len(records) != 0 {
		t.Fatalf("got %v, %v; want no records", records, err)
	}
}

func TestParseJournalRecordsLongDescription(t *testing.T) {
	desc := strings.Repeat("x", 2<<20)
	line := fmt.Sprintf(`{"seq":1,"ts":"t","op":"update","issue_id":"a","issue":{"description":%q}}`+"\n", desc)
	records, err := parseJournalRecords([]byte(line))
	if err != nil || len(records) != 1 {
		t.Fatalf("got %d records, %v; want one", len(records), err)
	}
}

func TestParseJournalRecordsMalformed(t *testing.T) {
	if _, err := parseJournalRecords([]byte("{\"seq\":1}\nnot json\n")); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestTailJournal(t *testing.T) {
	calls, restore := mockRunCapture([]byte(bdEventsTailFixture), nil)
	defer restore()

	records, err := TailJournal(42, 500)
	if err != nil || len(records) != 6 {
		t.Fatalf("got %d records, %v", len(records), err)
	}
	want := []string{"bd", "events", "tail", "--since", "42", "--limit", "500", "--json"}
	if len(*calls) != 1 || !slices.Equal((*calls)[0], want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
}

func TestTailJournalTruncated(t *testing.T) {
	restore := mockRun([]byte(bdEventsTruncatedFixture), &exec.ExitError{})
	defer restore()

	_, err := TailJournal(2, 1)
	var trunc *JournalTruncatedError
	if !errors.As(err, &trunc) {
		t.Fatalf("err = %v, want *JournalTruncatedError", err)
	}
	if trunc.Since != 2 || trunc.Floor != 6 || trunc.Head != 12 {
		t.Fatalf("truncation = %+v", trunc)
	}
}

func TestTailJournalUnsupported(t *testing.T) {
	restore := mockRun(nil, &exec.ExitError{Stderr: []byte(bdEventsUnknownCommandStderr)})
	defer restore()

	if _, err := TailJournal(0, 1); !errors.Is(err, ErrJournalUnsupported) {
		t.Fatalf("err = %v, want ErrJournalUnsupported", err)
	}
}

func TestTailJournalOtherError(t *testing.T) {
	restore := mockRun(nil, &exec.ExitError{Stderr: []byte("Error: no beads database found\n")})
	defer restore()

	_, err := TailJournal(0, 1)
	var trunc *JournalTruncatedError
	if err == nil || errors.Is(err, ErrJournalUnsupported) || errors.As(err, &trunc) {
		t.Fatalf("err = %v, want a plain error", err)
	}
}

func TestJournalEnabled(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"1", true}, // BD_EVENTS_JOURNAL=1 in the environment
		{"0", false},
		{"yes", true},
		{"on", true},
		{"", false}, // unset: what bd 1.1.0 reports
		{"garbage", false},
	}
	for _, tt := range tests {
		t.Run(strconv.Quote(tt.value), func(t *testing.T) {
			out := fmt.Sprintf(`{"key":"events-journal","location":"config.yaml","schema_version":1,"value":%q}`, tt.value)
			calls, restore := mockRunCapture([]byte(out), nil)
			defer restore()

			got, err := JournalEnabled()
			if err != nil || got != tt.want {
				t.Fatalf("JournalEnabled() = %v, %v; want %v", got, err, tt.want)
			}
			want := []string{"bd", "config", "get", "events-journal", "--json"}
			if !slices.Equal((*calls)[0], want) {
				t.Fatalf("call = %v, want %v", (*calls)[0], want)
			}
		})
	}
}

// fakeJournal emulates `bd events tail --since N --limit M --json` over a
// journal retaining seqs floor..head. gen stands in for the journal's
// identity: a reset journal gives the same seq a different record.
type fakeJournal struct {
	floor, head int64
	gen         int
	calls       int
}

func (f *fakeJournal) record(seq int64) JournalRecord {
	return JournalRecord{Seq: seq, TS: fmt.Sprintf("g%d-t%d", f.gen, seq), Op: "update", IssueID: fmt.Sprintf("fx-%d", seq)}
}

func (f *fakeJournal) run(_ time.Duration, _ string, args ...string) ([]byte, error) {
	f.calls++
	var since int64
	var limit int
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "--since":
			since, _ = strconv.ParseInt(args[i+1], 10, 64)
		case "--limit":
			limit, _ = strconv.Atoi(args[i+1])
		}
	}
	if since < f.floor-1 {
		payload, _ := json.Marshal(map[string]any{
			"code": journalTruncatedCode, "since": since, "floor": f.floor, "head": f.head,
		})
		return payload, &exec.ExitError{}
	}
	var out []byte
	for seq := since + 1; seq <= f.head && (limit == 0 || int(seq-since) <= limit); seq++ {
		line, _ := json.Marshal(f.record(seq))
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out, nil
}

func (f *fakeJournal) install() func() {
	orig := runWithTimeout
	runWithTimeout = f.run
	return func() { runWithTimeout = orig }
}

func TestFindJournalHead(t *testing.T) {
	for _, head := range []int64{0, 1, 2, 3, 74, 1024, 100_000} {
		for _, mode := range []string{"unpruned", "pruned", "fully pruned"} {
			f := &fakeJournal{floor: 1, head: head}
			switch mode {
			case "pruned":
				if head < 2 {
					continue
				}
				f.floor = head / 2
			case "fully pruned":
				if head == 0 {
					continue
				}
				f.floor = head + 1
			}
			t.Run(fmt.Sprintf("%s head %d", mode, head), func(t *testing.T) {
				defer f.install()()

				anchor, err := FindJournalHead()
				if err != nil {
					t.Fatal(err)
				}
				if anchor.Seq != head {
					t.Fatalf("anchor seq = %d, want %d", anchor.Seq, head)
				}
				wantRef := head > 0 && mode != "fully pruned"
				if (anchor.Ref != nil) != wantRef {
					t.Fatalf("anchor ref = %+v, want present: %v", anchor.Ref, wantRef)
				}
				if anchor.Ref != nil && !anchor.Ref.sameRecord(f.record(head)) {
					t.Fatalf("anchor ref = %+v, want the head record", anchor.Ref)
				}
				if bound := 2*bits.Len64(uint64(head)) + 3; f.calls > bound {
					t.Fatalf("%d reads, want at most %d", f.calls, bound)
				}
			})
		}
	}
}

func TestProbeJournal(t *testing.T) {
	f := &fakeJournal{floor: 1, head: 10}
	defer f.install()()

	ref := f.record(7)
	records, err := ProbeJournal(JournalAnchor{Seq: 7, Ref: &ref}, 500)
	if err != nil {
		t.Fatal(err)
	}
	var seqs []int64
	for _, r := range records {
		seqs = append(seqs, r.Seq)
	}
	if !slices.Equal(seqs, []int64{8, 9, 10}) {
		t.Fatalf("seqs = %v, want [8 9 10]", seqs)
	}

	// Caught up: only the anchor's own record comes back.
	ref = f.record(10)
	if records, err := ProbeJournal(JournalAnchor{Seq: 10, Ref: &ref}, 500); err != nil || len(records) != 0 {
		t.Fatalf("got %v, %v; want nothing new", records, err)
	}
}

func TestProbeJournalLimit(t *testing.T) {
	f := &fakeJournal{floor: 1, head: 1000}
	defer f.install()()

	ref := f.record(1)
	records, err := ProbeJournal(JournalAnchor{Seq: 1, Ref: &ref}, 500)
	if err != nil || len(records) != 500 || records[0].Seq != 2 {
		t.Fatalf("got %d records from %v, %v; want 500 starting at seq 2", len(records), records[0].Seq, err)
	}
}

func TestProbeJournalReset(t *testing.T) {
	old := &fakeJournal{floor: 1, head: 50}
	ref := old.record(50)
	anchor := JournalAnchor{Seq: 50, Ref: &ref}

	// A fresh clone: the sequence restarted and has not reached the anchor.
	short := &fakeJournal{floor: 1, head: 3, gen: 1}
	restore := short.install()
	_, err := ProbeJournal(anchor, 500)
	restore()
	if !errors.Is(err, ErrJournalReset) {
		t.Fatalf("short journal: err = %v, want ErrJournalReset", err)
	}

	// A fresh clone that has grown past the anchor holds a different record there.
	grown := &fakeJournal{floor: 1, head: 80, gen: 1}
	defer grown.install()()
	if _, err := ProbeJournal(anchor, 500); !errors.Is(err, ErrJournalReset) {
		t.Fatalf("grown journal: err = %v, want ErrJournalReset", err)
	}
}

func TestProbeJournalNoRef(t *testing.T) {
	// An anchor from a journal pruned to nothing has no record to verify.
	f := &fakeJournal{floor: 13, head: 14}
	defer f.install()()

	records, err := ProbeJournal(JournalAnchor{Seq: 12}, 500)
	if err != nil || len(records) != 2 || records[0].Seq != 13 {
		t.Fatalf("got %v, %v; want seqs 13 and 14", records, err)
	}
}

func TestProbeJournalTruncated(t *testing.T) {
	// The anchor's record was pruned.
	f := &fakeJournal{floor: 20, head: 30}
	defer f.install()()

	ref := f.record(5)
	_, err := ProbeJournal(JournalAnchor{Seq: 5, Ref: &ref}, 500)
	var trunc *JournalTruncatedError
	if !errors.As(err, &trunc) || trunc.Head != 30 {
		t.Fatalf("err = %v, want truncation reporting head 30", err)
	}
}

func TestJournalAnchorAt(t *testing.T) {
	start := JournalAnchor{Seq: 4}
	if got := start.AnchorAt(nil); got.Seq != 4 || got.Ref != nil {
		t.Fatalf("no records: got %+v, want the anchor unchanged", got)
	}
	records := []JournalRecord{{Seq: 5, TS: "a"}, {Seq: 6, TS: "b"}}
	got := start.AnchorAt(records)
	if got.Seq != 6 || got.Ref == nil || got.Ref.TS != "b" {
		t.Fatalf("got %+v, want anchored on seq 6", got)
	}
	records[1].TS = "mutated"
	if got.Ref.TS != "b" {
		t.Fatal("anchor must not alias the records slice")
	}
}

func TestProbeJournalPrunedAnchor(t *testing.T) {
	// Still pruned to nothing: caught up.
	pruned := &fakeJournal{floor: 13, head: 12}
	restore := pruned.install()
	records, err := ProbeJournal(JournalAnchor{Seq: 12}, 500)
	restore()
	if err != nil || len(records) != 0 {
		t.Fatalf("pruned: got %v, %v; want caught up", records, err)
	}

	// Reset below the anchor (a fresh clone restarting at seq 1).
	reset := &fakeJournal{floor: 1, head: 3, gen: 1}
	restore = reset.install()
	_, err = ProbeJournal(JournalAnchor{Seq: 12}, 500)
	restore()
	if !errors.Is(err, ErrJournalReset) {
		t.Fatalf("reset: err = %v, want ErrJournalReset", err)
	}

	// Reset and pruned again below the anchor.
	resetPruned := &fakeJournal{floor: 3, head: 2, gen: 1}
	defer resetPruned.install()()
	if _, err := ProbeJournal(JournalAnchor{Seq: 12}, 500); !errors.Is(err, ErrJournalReset) {
		t.Fatalf("reset and pruned: err = %v, want ErrJournalReset", err)
	}
}
