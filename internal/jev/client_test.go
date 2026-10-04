package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient points a client at srv with no retry sleep.
func newTestClient(t *testing.T, srv *httptest.Server, opts ...ClientOption) *Client {
	t.Helper()
	opts = append([]ClientOption{WithURL(srv.URL), WithTimeout(2 * time.Second)}, opts...)
	c, err := NewClient("test-key", opts...)
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(time.Duration) {}
	return c
}

func TestEvaluate(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"model":"jev-latest","answers":{
			"urgent":{"type":"noul","noul":0.91,"confidence":0.8},
			"team":{"type":"choice","choice":"billing","probabilities":{"billing":0.7,"tech":0.3},"confidence":0.75},
			"mood":{"type":"score","score":1.4,"probabilities":{"Calm":0.2,"Angry":0.8},"confidence":0.6}
		},"usage":{"input_tokens":120,"output_tokens":0}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.Evaluate(context.Background(), map[string]any{"text": "refund me"}, map[string]Question{
		"urgent": NewNoul("Is this urgent?"),
		"team":   NewChoice("Which team?", Option{Name: "billing", Desc: "payments"}, Option{Name: "tech"}),
		"mood":   NewScore("How angry?", "Calm", "Angry"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q", gotAuth)
	}

	var req map[string]any
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, gotBody)
	}
	if req["model"] != "jev-latest" {
		t.Errorf("model = %v", req["model"])
	}
	if req["state"].(map[string]any)["text"] != "refund me" {
		t.Errorf("state = %v", req["state"])
	}
	// Choice options marshal as an ordered object; a missing Desc is null.
	if !strings.Contains(gotBody, `"criteria":{"billing":"payments","tech":null}`) {
		t.Errorf("choice criteria not an ordered object:\n%s", gotBody)
	}
	if !strings.Contains(gotBody, `"criteria":["Calm","Angry"]`) {
		t.Errorf("score criteria not the level list:\n%s", gotBody)
	}
	qs := req["questions"].(map[string]any)
	if _, has := qs["urgent"].(map[string]any)["criteria"]; has {
		t.Errorf("noul must not send criteria: %v", qs["urgent"])
	}

	if !res.Answers["urgent"].Yes(0.9) || res.Answers["urgent"].Yes(0.95) {
		t.Errorf("urgent = %+v", res.Answers["urgent"])
	}
	if res.Answers["team"].Choice != "billing" || res.Answers["team"].Probabilities["tech"] != 0.3 {
		t.Errorf("team = %+v", res.Answers["team"])
	}
	if res.Answers["mood"].Score != 1.4 || res.Answers["mood"].Confidence != 0.6 {
		t.Errorf("mood = %+v", res.Answers["mood"])
	}
	if res.Usage.InputTokens != 120 {
		t.Errorf("usage = %+v", res.Usage)
	}
}

func TestEvaluateEdgeCaseNoQuestions(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.Evaluate(context.Background(), nil, nil); !errors.Is(err, ErrNoQuestions) {
		t.Fatalf("err = %v, want ErrNoQuestions", err)
	}
	if calls != 0 {
		t.Fatal("an empty question set must not make a request")
	}
}

func TestEvaluateEdgeCasePermanentErrorNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid api key"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.Ask(context.Background(), "s", NewNoul("q"))
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusUnauthorized || !se.Permanent() {
		t.Fatalf("err = %v, want permanent 401 StatusError", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, a 401 must not be retried", calls)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error text leaks the key: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error text should carry the response body: %v", err)
	}
}

func TestEvaluateEdgeCaseRetriesOnceOnOverload(t *testing.T) {
	var calls int32
	var slept time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(529)
			return
		}
		_, _ = io.WriteString(w, `{"answers":{"q":{"type":"noul","noul":0.2,"confidence":0.9}}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	c.sleep = func(d time.Duration) { slept = d }
	a, err := c.Ask(context.Background(), "s", NewNoul("q"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Noul != 0.2 || calls != 2 {
		t.Fatalf("answer %+v after %d calls", a, calls)
	}
	if slept != time.Second {
		t.Fatalf("slept %v, want the server's Retry-After of 1s", slept)
	}
}

func TestEvaluateEdgeCaseSecondFailureIsFinal(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "600") // capped, never actually waited in tests
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	var slept time.Duration
	c.sleep = func(d time.Duration) { slept = d }
	_, err := c.Ask(context.Background(), "s", NewNoul("q"))
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want 503", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want exactly one retry", calls)
	}
	if slept != maxRetryAfter {
		t.Fatalf("slept %v, want Retry-After capped at %v", slept, maxRetryAfter)
	}
}

func TestEvaluateEdgeCaseBadRequestNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.Ask(context.Background(), "s", NewNoul("q"))
	var se *StatusError
	if !errors.As(err, &se) || se.Permanent() {
		t.Fatalf("err = %v, want a non-permanent 422", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, a 4xx other than 429 must not be retried", calls)
	}
}

func TestEvaluateEdgeCaseTimeout(t *testing.T) {
	// Deferred LIFO: release the handler before srv.Close waits on it. The
	// handler also drains the body, or the server would never notice the
	// client giving up and r.Context() would never end.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := newTestClient(t, srv, WithTimeout(30*time.Millisecond))
	start := time.Now()
	_, err := c.Ask(context.Background(), "s", NewNoul("q"))
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %v; the attempt timeout did not bound the call", time.Since(start))
	}
}

func TestEvaluateEdgeCaseCallerContextWins(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	c.sleep = func(time.Duration) { cancel() } // the caller gives up during the backoff
	_, err := c.Ask(ctx, "s", NewNoul("q"))
	if err == nil || calls != 1 {
		t.Fatalf("err = %v after %d calls; a cancelled context must stop the retry", err, calls)
	}
}

func TestEvaluateEdgeCaseMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html>not json`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.Ask(context.Background(), "s", NewNoul("q")); err == nil || !strings.Contains(err.Error(), "unexpected response") {
		t.Fatalf("err = %v", err)
	}
}

func TestEvaluateEdgeCaseMissingAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"answers":{}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.Ask(context.Background(), "s", NewNoul("q")); err == nil {
		t.Fatal("Ask must fail when its answer is missing")
	}
}

func TestNewClientEdgeCaseBadURL(t *testing.T) {
	for _, u := range []string{"ftp://host/x", "://bad", ""} {
		if _, err := NewClient("k", WithURL(u)); err == nil {
			t.Errorf("NewClient accepted %q", u)
		}
	}
}

func TestEndpoint(t *testing.T) {
	cases := map[string]string{
		"localhost:8080":                       "http://localhost:8080/v1/systemone",
		"http://host/":                         "http://host/v1/systemone",
		"https://api.example.com":              "https://api.example.com/v1/systemone",
		"https://api.example.com/v2/x":         "https://api.example.com/v2/x",
		"https://api.typesafe.ai/v1/systemone": "https://api.typesafe.ai/v1/systemone",
	}
	for in, want := range cases {
		if got := Endpoint(in); got != want {
			t.Errorf("Endpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromEnv(t *testing.T) {
	env := func(vals map[string]string) func(string) string {
		return func(k string) string { return vals[k] }
	}

	if c, err := fromEnv(env(nil)); c != nil || err != nil {
		t.Fatalf("no key: got %v, %v; want nil, nil", c, err)
	}
	if c, err := fromEnv(env(map[string]string{EnvAPIKey: "k", EnvToggle: "OFF"})); c != nil || err != nil {
		t.Fatalf("MG_JEV=off: got %v, %v; want nil, nil", c, err)
	}
	c, err := fromEnv(env(map[string]string{EnvAPIKey: " k "}))
	if err != nil || c == nil {
		t.Fatalf("key only: %v, %v", c, err)
	}
	if c.URL() != DefaultURL || c.Model() != DefaultModel || c.apiKey != "k" {
		t.Fatalf("defaults: url %q model %q key %q", c.URL(), c.Model(), c.apiKey)
	}
	c, err = fromEnv(env(map[string]string{EnvAPIKey: "k", EnvURL: "localhost:8091", EnvModel: "von"}))
	if err != nil || c.URL() != "http://localhost:8091/v1/systemone" || c.Model() != "von" {
		t.Fatalf("self-host: %v, %v", c, err)
	}
	if _, err := fromEnv(env(map[string]string{EnvAPIKey: "k", EnvURL: "ftp://x"})); err == nil || !strings.Contains(err.Error(), EnvURL) {
		t.Fatalf("bad URL: err = %v, want one naming %s", err, EnvURL)
	}
}

func TestSetCmdTimeout(t *testing.T) {
	t.Cleanup(func() { timeoutShort = defaultTimeoutShort })
	SetCmdTimeout(60)
	if Timeout() != 10*time.Second {
		t.Fatalf("Timeout() = %v after SetCmdTimeout(60), want 10s", Timeout())
	}
	SetCmdTimeout(0)
	if Timeout() != 10*time.Second {
		t.Fatal("SetCmdTimeout(0) must be a no-op")
	}
}
