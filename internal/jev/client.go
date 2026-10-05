package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultURL is the hosted System One endpoint.
	DefaultURL = "https://api.typesafe.ai/v1/systemone"
	// DefaultModel is the model route every request names unless configured.
	DefaultModel = "jev-latest"

	// maxErrorBody bounds how much of an error response is kept in the error.
	maxErrorBody = 240
	// maxRetryAfter caps how long a Retry-After header may hold a retry.
	maxRetryAfter = 2 * time.Second
)

// ErrNoQuestions is returned by Evaluate when there is nothing to ask.
var ErrNoQuestions = errors.New("jev: no questions")

// Doer is the one method of *http.Client the client uses; tests substitute it.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Evaluator is what the app depends on, so tests can pass a fake.
type Evaluator interface {
	Evaluate(ctx context.Context, state any, questions map[string]Question) (*Result, error)
}

// Client talks to one System One endpoint.
type Client struct {
	url     string
	apiKey  string
	model   string
	http    Doer
	timeout time.Duration // per attempt
	sleep   func(time.Duration)
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithURL points the client at an API-compatible server. A bare host, with
// or without a scheme, gets the /v1/systemone path appended, so a self-hosted
// server can be named by address alone.
func WithURL(u string) ClientOption { return func(c *Client) { c.url = Endpoint(u) } }

// WithModel sets the model route (default DefaultModel).
func WithModel(m string) ClientOption { return func(c *Client) { c.model = m } }

// WithDoer substitutes the HTTP transport.
func WithDoer(d Doer) ClientOption { return func(c *Client) { c.http = d } }

// WithTimeout bounds each attempt (default the short command tier).
func WithTimeout(d time.Duration) ClientOption { return func(c *Client) { c.timeout = d } }

// NewClient builds a client for the hosted endpoint, or the one WithURL
// names. apiKey may be empty for a self-hosted server with no auth.
func NewClient(apiKey string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		url:     DefaultURL,
		apiKey:  apiKey,
		model:   DefaultModel,
		timeout: Timeout(),
		sleep:   time.Sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	u, err := url.Parse(c.url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("jev: invalid endpoint URL %q (want http(s)://host[/path])", c.url)
	}
	if c.http == nil {
		// Twice the attempt timeout is a backstop for a transport that
		// ignores the request context; the context is the real limit.
		c.http = &http.Client{Timeout: 2 * c.timeout}
	}
	return c, nil
}

// Endpoint normalizes a server address: "localhost:8080" or "http://host/"
// become "http://host:port/v1/systemone"; a URL that already has a path is
// used as given.
func Endpoint(addr string) string {
	scheme, rest := "http://", addr
	if i := strings.Index(addr, "://"); i >= 0 {
		scheme, rest = addr[:i+3], addr[i+3:]
	}
	if i := strings.IndexByte(rest, '/'); i < 0 || rest[i:] == "/" {
		return scheme + strings.TrimSuffix(rest, "/") + "/v1/systemone"
	}
	return scheme + rest
}

// URL returns the endpoint the client posts to.
func (c *Client) URL() string { return c.url }

// Model returns the model route the client names.
func (c *Client) Model() string { return c.model }

// StatusError is a non-2xx response. Body holds at most the first few
// hundred bytes of the response, and never the request.
type StatusError struct {
	Code       int
	Body       string
	RetryAfter time.Duration // from the Retry-After header, if any
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("jev: HTTP %d", e.Code)
	}
	return fmt.Sprintf("jev: HTTP %d: %s", e.Code, e.Body)
}

// Permanent reports a response that no retry and no later call will fix:
// a rejected key or a wrong endpoint. Callers should stop for the session.
func (e *StatusError) Permanent() bool {
	return e.Code == http.StatusUnauthorized || e.Code == http.StatusForbidden || e.Code == http.StatusNotFound
}

// retryable reports a response worth one more attempt: rate limiting, the
// vendor's overloaded signal (529), or a server error.
func (e *StatusError) retryable() bool {
	return e.Code == http.StatusTooManyRequests || e.Code == 529 || e.Code >= 500
}

// Evaluate asks every question in questions about state in one round trip.
// It retries once on a transport error, a timeout, 429, 529 or a 5xx, and
// returns a *StatusError for any other non-2xx response.
func (c *Client) Evaluate(ctx context.Context, state any, questions map[string]Question) (*Result, error) {
	if len(questions) == 0 {
		return nil, ErrNoQuestions
	}
	body, err := json.Marshal(request{State: state, Model: c.model, Questions: questions})
	if err != nil {
		return nil, fmt.Errorf("jev: encode request: %w", err)
	}
	data, err := c.post(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("jev: unexpected response: %w", err)
	}
	if resp.Answers == nil {
		return nil, errors.New("jev: unexpected response: no answers")
	}
	answers := make(map[string]Answer, len(resp.Answers))
	for name, w := range resp.Answers {
		answers[name] = w.normalize(questions[name])
	}
	return &Result{Model: resp.Model, Answers: answers, Usage: resp.Usage}, nil
}

// Ask is Evaluate with one question.
func (c *Client) Ask(ctx context.Context, state any, q Question) (*Answer, error) {
	res, err := c.Evaluate(ctx, state, map[string]Question{"q": q})
	if err != nil {
		return nil, err
	}
	a, ok := res.Answers["q"]
	if !ok {
		return nil, errors.New("jev: unexpected response: no answer")
	}
	return &a, nil
}

// post sends body, giving each attempt its own timeout and retrying once
// when the failure looks transient. The caller's ctx bounds the whole call.
func (c *Client) post(ctx context.Context, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		data, err := c.once(ctx, body)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, err
		}
		wait, ok := retryWait(err)
		if !ok {
			return nil, err
		}
		if attempt == 0 {
			c.sleep(wait)
		}
	}
	return nil, lastErr
}

// retryWait decides whether err is worth one more attempt and how long to
// wait first: the server's Retry-After when it gave one (capped), else a
// short jitter so a burst of callers does not retry in lockstep.
func retryWait(err error) (time.Duration, bool) {
	var se *StatusError
	if errors.As(err, &se) {
		if !se.retryable() {
			return 0, false
		}
		if se.RetryAfter > 0 {
			return min(se.RetryAfter, maxRetryAfter), true
		}
	}
	// Transport errors and attempt timeouts fall through to the jitter.
	return 200*time.Millisecond + time.Duration(rand.Int64N(int64(300*time.Millisecond))), true
}

func (c *Client) once(ctx context.Context, body []byte) ([]byte, error) {
	actx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(actx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jev: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mardi-gras")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("jev: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &StatusError{
			Code:       resp.StatusCode,
			Body:       errorBody(data),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	return data, nil
}

// errorBody keeps the start of an error response, on one line, for the
// error text. The API key travels in a header, so a response never holds
// it; the bound is for readability and log hygiene.
func errorBody(data []byte) string {
	s := strings.Join(strings.Fields(string(data)), " ")
	if len(s) > maxErrorBody {
		s = s[:maxErrorBody] + "…"
	}
	return s
}

// parseRetryAfter reads a delay-seconds Retry-After; HTTP dates are rare on
// APIs and are ignored.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
