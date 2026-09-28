package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// The bd events journal (bd 1.2.1+, opt-in per workspace with
// `bd config set events-journal true`) is an ordered record of every mutation
// made through the bd CLI. mg reads it only as a change signal: a record means
// "reload", never "apply this". A record's issue snapshot carries labels but
// not dependencies, and mg derives blocked state from dependencies, so
// `bd list` stays the source of truth.

// JournalRecord is the part of an events journal record mg reads.
type JournalRecord struct {
	Seq     int64         `json:"seq"`
	TS      string        `json:"ts"`
	Op      string        `json:"op"`
	IssueID string        `json:"issue_id"`
	Actor   string        `json:"actor,omitempty"` // empty on derived records (e.g. a blocked-state flip)
	Issue   *journalIssue `json:"issue"`           // nil on delete
}

type journalIssue struct {
	Ephemeral bool `json:"ephemeral,omitempty"`
}

// Ephemeral reports whether the record is about a wisp. Wisps are journaled
// but `bd list --all` never returns them, so they cannot change the parade.
func (r JournalRecord) Ephemeral() bool {
	return r.Issue != nil && r.Issue.Ephemeral
}

// sameRecord reports whether two reads of a seq returned the same record.
// seq alone cannot tell: a fresh clone restarts the sequence at 1.
func (r JournalRecord) sameRecord(o JournalRecord) bool {
	return r.Seq == o.Seq && r.TS == o.TS && r.Op == o.Op && r.IssueID == o.IssueID
}

// JournalAnchor is the newest journal record mg has accounted for.
type JournalAnchor struct {
	Seq int64
	// Ref is the record at Seq. Probes re-read it to detect a journal that
	// was reset under mg. It is nil when there was nothing to fingerprint: an
	// empty journal, or one pruned to nothing.
	Ref *JournalRecord
}

// AnchorAt returns the anchor for the last of records, or a unchanged when
// records is empty.
func (a JournalAnchor) AnchorAt(records []JournalRecord) JournalAnchor {
	if len(records) == 0 {
		return a
	}
	last := records[len(records)-1]
	return JournalAnchor{Seq: last.Seq, Ref: &last}
}

// JournalTruncatedError reports a checkpoint that fell below the journal's
// retained window: the records after it were pruned.
type JournalTruncatedError struct {
	Since, Floor, Head int64
}

func (e *JournalTruncatedError) Error() string {
	return fmt.Sprintf("events journal truncated: checkpoint %d is below the retained window [%d..%d]", e.Since, e.Floor, e.Head)
}

var (
	// ErrJournalUnsupported means the bd on PATH predates the journal.
	ErrJournalUnsupported = errors.New("bd has no events journal (needs bd 1.2.1 or newer)")
	// ErrJournalReset means the record an anchor points at is gone or
	// different, so the sequence mg was following no longer exists.
	ErrJournalReset = errors.New("events journal was reset")
)

const journalTruncatedCode = "events_journal_truncated"

// JournalEnabled reports whether this workspace records the events journal.
// `bd config get` resolves the value the way bd itself does, including a
// BD_EVENTS_JOURNAL override in the environment.
func JournalEnabled() (bool, error) {
	out, err := runWithTimeout(timeoutShort, "bd", "config", "get", "events-journal", "--json")
	if err != nil {
		return false, wrapExitError("bd config get events-journal", err)
	}
	var resp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return false, fmt.Errorf("bd config get events-journal: %w", err)
	}
	return parseConfigBool(resp.Value), nil
}

// parseConfigBool reads a bd config flag; anything unrecognised, including
// an unset key, is off.
func parseConfigBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "on":
		return true
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	return err == nil && b
}

// TailJournal returns up to limit records with seq greater than since, in
// seq order. A limit of 0 means no limit.
func TailJournal(since int64, limit int) ([]JournalRecord, error) {
	out, err := runWithTimeout(timeoutShort, "bd", journalTailArgs(since, limit)...)
	if err != nil {
		return nil, journalError(out, err)
	}
	return parseJournalRecords(out)
}

func journalTailArgs(since int64, limit int) []string {
	return []string{"events", "tail", "--since", strconv.FormatInt(since, 10), "--limit", strconv.Itoa(limit), "--json"}
}

// journalError classifies a failed `bd events tail`. A truncated checkpoint
// is reported as JSON on stdout (exit 1); a bd without the journal fails with
// an unknown-command error on stderr.
func journalError(stdout []byte, err error) error {
	var resp struct {
		Code  string `json:"code"`
		Since int64  `json:"since"`
		Floor int64  `json:"floor"`
		Head  int64  `json:"head"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &resp) == nil && resp.Code == journalTruncatedCode {
		return &JournalTruncatedError{Since: resp.Since, Floor: resp.Floor, Head: resp.Head}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), `unknown command "events"`) {
		return ErrJournalUnsupported
	}
	return wrapExitError("bd events tail", err)
}

// parseJournalRecords decodes `bd events tail --json` output: one record per
// line. A decoder rather than a line scanner, so a record carrying a long
// description is never cut at a buffer limit.
func parseJournalRecords(out []byte) ([]JournalRecord, error) {
	var records []JournalRecord
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var r JournalRecord
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("bd events tail parse: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}

// ProbeJournal returns up to limit records after anchor. It re-reads the
// anchor's own record first and returns ErrJournalReset when that record is
// gone or different; a checkpoint above a reset journal's head would
// otherwise read as "caught up" forever. An anchor without a record gets the
// same guarantee from verifyPrunedAnchor, at the cost of a second read when
// nothing is new.
func ProbeJournal(anchor JournalAnchor, limit int) ([]JournalRecord, error) {
	if anchor.Ref == nil {
		records, err := TailJournal(anchor.Seq, limit)
		if err != nil || len(records) > 0 || anchor.Seq == 0 {
			return records, err
		}
		return nil, verifyPrunedAnchor(anchor)
	}
	records, err := TailJournal(anchor.Seq-1, limit+1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || !records[0].sameRecord(*anchor.Ref) {
		return nil, ErrJournalReset
	}
	return records[1:], nil
}

// verifyPrunedAnchor checks an anchor with no record to fingerprint, left
// by a journal pruned to nothing. While the journal stays that way, a read
// from 0 reports it truncated with the anchor as head. Any other answer
// means the journal was reset below the anchor, where every probe would
// read as caught up forever.
func verifyPrunedAnchor(anchor JournalAnchor) error {
	_, err := TailJournal(0, 1)
	var trunc *JournalTruncatedError
	switch {
	case errors.As(err, &trunc):
		if trunc.Head < anchor.Seq {
			return ErrJournalReset
		}
		return nil
	case err != nil:
		return err
	}
	return ErrJournalReset
}

// FindJournalHead locates the newest journal record, so mg can follow from
// there. bd reports the head only in a truncation error, so an unpruned
// journal is searched: gallop up in powers of two until a read comes back
// empty, then binary search. That is about 2*log2(head) one-record reads.
// If the journal grows during the search the result can come out low,
// which only means the records after it read as new.
func FindJournalHead() (JournalAnchor, error) {
	// newest returns the record at seq k+1, or nil when k is at or past head.
	newest := func(k int64) (*JournalRecord, error) {
		records, err := TailJournal(k, 1)
		if err != nil || len(records) == 0 {
			return nil, err
		}
		return &records[0], nil
	}

	first, err := newest(0)
	var trunc *JournalTruncatedError
	if errors.As(err, &trunc) {
		return anchorFromTruncation(trunc, newest)
	}
	if err != nil {
		return JournalAnchor{}, err
	}
	if first == nil {
		return JournalAnchor{}, nil // empty journal
	}

	// Invariant: best is the record at seq lo+1, and hi is at or past head.
	lo, best := int64(0), first
	hi := int64(1)
	for {
		r, err := newest(hi)
		if errors.As(err, &trunc) {
			return anchorFromTruncation(trunc, newest)
		}
		if err != nil {
			return JournalAnchor{}, err
		}
		if r == nil {
			break
		}
		lo, best = hi, r
		hi *= 2
	}
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		r, err := newest(mid)
		if errors.As(err, &trunc) {
			return anchorFromTruncation(trunc, newest)
		}
		if err != nil {
			return JournalAnchor{}, err
		}
		if r == nil {
			hi = mid
		} else {
			lo, best = mid, r
		}
	}
	return JournalAnchor{Seq: best.Seq, Ref: best}, nil
}

// anchorFromTruncation anchors on the head a truncation error reports. When
// the head record is still retained it is fetched as the fingerprint; when
// the journal was pruned to nothing (floor past head) there is none.
func anchorFromTruncation(trunc *JournalTruncatedError, newest func(int64) (*JournalRecord, error)) (JournalAnchor, error) {
	if trunc.Floor > trunc.Head {
		return JournalAnchor{Seq: trunc.Head}, nil
	}
	r, err := newest(trunc.Head - 1)
	if err != nil {
		return JournalAnchor{}, err
	}
	if r == nil {
		return JournalAnchor{Seq: trunc.Head}, nil
	}
	return JournalAnchor{Seq: r.Seq, Ref: r}, nil
}
