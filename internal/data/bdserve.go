package data

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// `bd serve` (bd 1.3.0+, preview) serves the same events journal over HTTP
// on loopback, including a server-sent-events stream of new records. With
// MG_BD_SERVE=<url> set, mg reads that stream instead of probing with
// `bd events tail`: changes arrive in about a second, with no bd process per
// check. It only runs against a workspace in Dolt server mode (bd serve
// refuses embedded), and mg falls back to CLI probes whenever the stream is
// down.

// ServeEvent is one thing a bd serve stream reports.
type ServeEvent struct {
	Connected bool            // the stream opened: CLI probes can pause
	Records   []JournalRecord // new records, in seq order
	// Err ends the stream. A *JournalTruncatedError or ErrJournalReset means
	// the anchor is gone (re-find the head); ErrServeJournalDisabled and
	// ErrServeRefused are described on those errors; anything else is a
	// dropped connection worth retrying.
	Err error
	// RetryAfter is the server's requested wait before reconnecting, if any.
	RetryAfter time.Duration
}

var (
	// ErrServeJournalDisabled: bd serve reports the journal off. It reads
	// the setting once at startup, so it may need a restart after the
	// journal is turned on.
	ErrServeJournalDisabled = errors.New("bd serve: the events journal is disabled")
	// ErrServeRefused: bd serve won't talk to mg (wrong URL, auth,
	// workspace mismatch). Retrying won't help.
	ErrServeRefused = errors.New("bd serve refused the connection")
)

// serveIdle drops a stream that goes quiet past a few of bd serve's 20s
// heartbeats. A variable so tests can shorten it.
var serveIdle = 65 * time.Second

// WatchServeJournal opens one bd serve stream of records after anchor, and
// closes the channel when it ends; the caller decides when to reconnect.
// projectID, when known, is sent so a bd serve for another workspace refuses
// rather than feeding this one's reloads. Only the paged read in
// verifyServeAnchor enforces it: bd 1.3.0's watch stream ignores the header.
func WatchServeJournal(ctx context.Context, baseURL, projectID string, anchor JournalAnchor) <-chan ServeEvent {
	out := make(chan ServeEvent, 64)
	idle := serveIdle
	go func() {
		defer close(out)
		err := watchServe(ctx, baseURL, projectID, anchor, idle, out)
		if err != nil && ctx.Err() == nil {
			ev := ServeEvent{Err: err}
			var retry *serveRetryError
			if errors.As(err, &retry) {
				ev.Err, ev.RetryAfter = retry.err, retry.after
			}
			select {
			case out <- ev:
			case <-ctx.Done():
			}
		}
	}()
	return out
}

// serveRetryError carries a Retry-After alongside the error it explains.
type serveRetryError struct {
	err   error
	after time.Duration
}

func (e *serveRetryError) Error() string { return e.err.Error() }
func (e *serveRetryError) Unwrap() error { return e.err }

func watchServe(ctx context.Context, baseURL, projectID string, anchor JournalAnchor, idle time.Duration, out chan<- ServeEvent) error {
	// No Timeout: it would cut the stream. ctx and the idle watchdog end it.
	client := &http.Client{}
	base := strings.TrimRight(baseURL, "/")
	// Run before every connection, even when the anchor hasn't moved: it is
	// also the wrong-workspace check (see WatchServeJournal).
	if err := verifyServeAnchor(ctx, client, base, projectID, anchor); err != nil {
		return err
	}

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	q := url.Values{"since": {strconv.FormatInt(anchor.Seq, 10)}}
	req, err := http.NewRequestWithContext(connCtx, http.MethodGet, base+"/v0/beads/events:watch?"+q.Encode(), http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	setServeHeaders(req, projectID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return serveStatusError(resp)
	}
	select {
	case out <- ServeEvent{Connected: true}:
	case <-ctx.Done():
		return nil
	}

	lines := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(idle)
		defer timer.Stop()
		for {
			select {
			case <-lines:
				timer.Reset(idle) // Go 1.23+ timers: Reset alone discards any pending fire
			case <-timer.C:
				cancel()
				return
			case <-connCtx.Done():
				return
			}
		}
	}()

	// ReadString, not a Scanner: a record with a long description is one
	// data line longer than a Scanner's default buffer.
	r := bufio.NewReader(resp.Body)
	var event, data string
	var batch []JournalRecord
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("bd serve stream: %w", err)
		}
		select {
		case lines <- struct{}{}:
		default:
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, ":"): // heartbeat comment
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		case line == "" && data != "":
			if event == "truncated" {
				return journalError([]byte(data), errors.New("bd serve: truncated"))
			}
			var rec JournalRecord
			if json.Unmarshal([]byte(data), &rec) == nil && rec.Seq > 0 {
				batch = append(batch, rec)
			}
			event, data = "", ""
			// Send once nothing more is already buffered, so a burst goes
			// out as one batch.
			if len(batch) > 0 && (r.Buffered() == 0 || len(batch) >= JournalProbeLimit) {
				select {
				case out <- ServeEvent{Records: batch}:
				case <-ctx.Done():
					return nil
				}
				batch = nil
			}
		case line == "":
			event = ""
		}
	}
}

// verifyServeAnchor makes sure the record the anchor names is still the one
// at that seq, as ProbeJournal does for the CLI, so a reset journal can't
// read as caught up forever. It uses the paged endpoint, which also reports
// the head.
func verifyServeAnchor(ctx context.Context, client *http.Client, base, projectID string, anchor JournalAnchor) error {
	since := anchor.Seq
	if anchor.Ref != nil {
		since = anchor.Seq - 1
	}
	q := url.Values{"since": {strconv.FormatInt(since, 10)}, "limit": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v0/beads/events?"+q.Encode(), http.NoBody)
	if err != nil {
		return err
	}
	setServeHeaders(req, projectID)
	reqCtx, cancel := context.WithTimeout(ctx, timeoutShort)
	defer cancel()
	resp, err := client.Do(req.WithContext(reqCtx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return serveStatusError(resp)
	}
	var page struct {
		Head    int64           `json:"head"`
		Records []JournalRecord `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return fmt.Errorf("bd serve events: %w", err)
	}
	switch {
	case anchor.Ref != nil && (len(page.Records) == 0 || !page.Records[0].sameRecord(*anchor.Ref)):
		return ErrJournalReset
	case anchor.Ref == nil && page.Head < anchor.Seq:
		return ErrJournalReset
	}
	return nil
}

func setServeHeaders(req *http.Request, projectID string) {
	if projectID != "" {
		req.Header.Set("Bd-Project-Id", projectID)
	}
}

// serveStatusError maps a non-200 bd serve response onto mg's journal
// errors.
func serveStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusGone:
		return journalError(body, fmt.Errorf("bd serve: HTTP 410"))
	case http.StatusConflict:
		return ErrServeJournalDisabled
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return fmt.Errorf("%w: HTTP %d: %s", ErrServeRefused, resp.StatusCode, strings.TrimSpace(string(body)))
	case http.StatusServiceUnavailable:
		after := 30 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			after = time.Duration(s) * time.Second
		}
		return &serveRetryError{err: fmt.Errorf("bd serve busy: HTTP 503"), after: after}
	}
	return fmt.Errorf("bd serve: HTTP %d", resp.StatusCode)
}
