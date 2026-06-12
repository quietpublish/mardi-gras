package gastown

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	gcCitiesJSON = `{"items":[{"name":"mardi_gras","path":"/x","running":true}],"total":1}`
	gcAgentsJSON = `{"items":[{"name":"obsidian","running":true,"suspended":false,"state":"working",` +
		`"available":true,"pool":"polecat","rig":"mardi_gras","active_bead":"mg-42",` +
		`"activity":"implementing","model":"opus","provider":"anthropic",` +
		`"display_name":"obi","session":{"name":"sess-1","attached":true}}],"total":1}`
)

// gcTestServer stands up a fake supervisor that answers the read endpoints.
func gcTestServer(t *testing.T, citiesBody, agentsBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/cities", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(citiesBody))
	})
	mux.HandleFunc("/v0/city/mardi_gras/agents", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(agentsBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGCDriverBackend(t *testing.T) {
	d, err := NewGCDriver("http://127.0.0.1:8080", "")
	if err != nil {
		t.Fatalf("NewGCDriver: %v", err)
	}
	if got := d.Backend(); got != "gascity" {
		t.Errorf("Backend() = %q, want %q", got, "gascity")
	}
}

func TestGCDriverSupports(t *testing.T) {
	d, _ := NewGCDriver("http://127.0.0.1:8080", "")
	for _, f := range []Feature{FeatureVitals, FeatureCosts, FeaturePatrol, FeatureSSE} {
		if d.Supports(f) {
			t.Errorf("Supports(%d) = true, want false", f)
		}
	}
}

func TestGCDriverStatus(t *testing.T) {
	srv := gcTestServer(t, gcCitiesJSON, gcAgentsJSON)
	d, err := NewGCDriver(srv.URL, "")
	if err != nil {
		t.Fatalf("NewGCDriver: %v", err)
	}
	status, err := d.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(status.Agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(status.Agents))
	}
	a := status.Agents[0]
	checks := map[string]struct{ got, want string }{
		"Name":       {a.Name, "obsidian"},
		"Role":       {a.Role, "polecat"},
		"Rig":        {a.Rig, "mardi_gras"},
		"State":      {a.State, "working"},
		"HookBead":   {a.HookBead, "mg-42"},
		"WorkTitle":  {a.WorkTitle, "implementing"},
		"AgentInfo":  {a.AgentInfo, "anthropic/opus"},
		"AgentAlias": {a.AgentAlias, "obi"},
		"Session":    {a.Session, "sess-1"},
		"Address":    {a.Address, "obsidian"},
	}
	for field, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", field, c.got, c.want)
		}
	}
	if !a.Running || !a.HasWork {
		t.Errorf("Running=%v HasWork=%v, want both true", a.Running, a.HasWork)
	}
	if len(status.Rigs) != 1 || status.Rigs[0].Name != "mardi_gras" || status.Rigs[0].PolecatCount != 1 {
		t.Errorf("rigs = %+v, want one mardi_gras with PolecatCount 1", status.Rigs)
	}
}

func TestGCDriverStatusPinnedCitySkipsCitiesCall(t *testing.T) {
	// No /v0/cities handler: a pinned city must not call it.
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/cities", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("pinned city should not query /v0/cities")
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/v0/city/mardi_gras/agents", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(gcAgentsJSON))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if _, err := d.Status(context.Background()); err != nil {
		t.Fatalf("Status: %v", err)
	}
}

func TestGCDriverStatusNoRunningCityFallsBackToFirst(t *testing.T) {
	cities := `{"items":[{"name":"mardi_gras","path":"/x","running":false}],"total":1}`
	srv := gcTestServer(t, cities, gcAgentsJSON)
	d, _ := NewGCDriver(srv.URL, "")
	status, err := d.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(status.Agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(status.Agents))
	}
}

func TestGCDriverStatusServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if _, err := d.Status(context.Background()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestGCDriverUnsupportedOps(t *testing.T) {
	d, _ := NewGCDriver("http://127.0.0.1:8080", "")
	ctx := context.Background()
	if err := d.Sling(ctx, SlingRequest{IssueIDs: []string{"x"}}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Sling err = %v, want ErrUnsupported", err)
	}
	if _, err := d.Vitals(ctx); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Vitals err = %v, want ErrUnsupported", err)
	}
	if err := d.Unsling(ctx, "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Unsling err = %v, want ErrUnsupported", err)
	}
	if err := d.Nudge(ctx, "t", "m"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Nudge err = %v, want ErrUnsupported", err)
	}
}

func TestGCInferRole(t *testing.T) {
	tests := []struct {
		pool, name, want string
	}{
		{"polecat", "obsidian", "polecat"},
		{"crew", "matt", "crew"},
		{"", "mardi_gras-witness", "witness"},
		{"custompool", "", "custompool"},
		{"", "", ""},
		{"Refinery", "x", "refinery"},
	}
	for _, tt := range tests {
		if got := gcInferRole(tt.pool, tt.name); got != tt.want {
			t.Errorf("gcInferRole(%q,%q) = %q, want %q", tt.pool, tt.name, got, tt.want)
		}
	}
}

func TestGCAgentInfo(t *testing.T) {
	s := func(v string) *string { return &v }
	cases := []struct {
		provider, model *string
		want            string
	}{
		{s("anthropic"), s("opus"), "anthropic/opus"},
		{nil, s("opus"), "opus"},
		{s("anthropic"), nil, "anthropic"},
		{nil, nil, ""},
	}
	for _, c := range cases {
		if got := gcAgentInfo(c.provider, c.model); got != c.want {
			t.Errorf("gcAgentInfo = %q, want %q", got, c.want)
		}
	}
}

func TestGCDeriveRigs(t *testing.T) {
	agents := []AgentRuntime{
		{Rig: "alpha", Role: "polecat"},
		{Rig: "alpha", Role: "crew"},
		{Rig: "alpha", Role: "witness"},
		{Rig: "beta", Role: "polecat"},
		{Rig: "", Role: "mayor"}, // no rig → skipped
	}
	rigs := gcDeriveRigs(agents)
	if len(rigs) != 2 {
		t.Fatalf("got %d rigs, want 2", len(rigs))
	}
	if rigs[0].Name != "alpha" || rigs[0].PolecatCount != 1 || rigs[0].CrewCount != 1 || !rigs[0].HasWitness {
		t.Errorf("alpha = %+v", rigs[0])
	}
	if rigs[1].Name != "beta" || rigs[1].PolecatCount != 1 {
		t.Errorf("beta = %+v", rigs[1])
	}
}

func TestGCBaseURL(t *testing.T) {
	t.Setenv(EnvGCAPI, "")
	if got := GCBaseURL(); got != gcDefaultBaseURL {
		t.Errorf("default GCBaseURL = %q, want %q", got, gcDefaultBaseURL)
	}
	t.Setenv(EnvGCAPI, "http://10.0.0.5:9000/")
	if got := GCBaseURL(); got != "http://10.0.0.5:9000" {
		t.Errorf("GCBaseURL = %q, want trailing slash trimmed", got)
	}
}

func TestSelectDriver(t *testing.T) {
	t.Setenv(EnvGCAPI, "")
	if d := SelectDriver(); d.Backend() != "gastown" {
		t.Errorf("without %s: Backend() = %q, want gastown", EnvGCAPI, d.Backend())
	}
	t.Setenv(EnvGCAPI, "http://127.0.0.1:8080")
	if d := SelectDriver(); d.Backend() != "gascity" {
		t.Errorf("with %s: Backend() = %q, want gascity", EnvGCAPI, d.Backend())
	}
}

// gcMailServer answers the Phase 3 mail/formula endpoints for a pinned city
// and records the X-GC-Request header seen on the most recent mutation.
func gcMailServer(t *testing.T, lastCSRF *string) *httptest.Server {
	t.Helper()
	recordCSRF := func(r *http.Request) { *lastCSRF = r.Header.Get("X-GC-Request") }
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/formulas", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"name":"shiny","description":"d","version":"1","var_defs":[],"run_count":0,"recent_runs":[]},{"name":"quick","description":"d","version":"1","var_defs":[],"run_count":0,"recent_runs":[]}],"total":2,"partial":false}`))
	})
	mux.HandleFunc("/v0/city/mardi_gras/mail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost { // send-mail
			recordCSRF(r)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"new","from":"me","to":"you","subject":"s","body":"b","created_at":"2026-06-12T10:00:00Z","read":false}`))
			return
		}
		unread := r.URL.Query().Get("status") == "unread"
		body := `{"items":[{"id":"m1","from":"mayor","to":"me","subject":"hi","body":"yo","created_at":"2026-06-12T10:00:00Z","read":false,"thread_id":"t1"}],"total":1}`
		if !unread {
			body = `{"items":[{"id":"m1","from":"mayor","to":"me","subject":"hi","body":"yo","created_at":"2026-06-12T10:00:00Z","read":false},{"id":"m2","from":"x","to":"me","subject":"read","body":"b","created_at":"2026-06-12T09:00:00Z","read":true}],"total":2}`
		}
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/v0/city/mardi_gras/mail/m1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m1","from":"mayor","to":"me","subject":"hi","body":"yo","created_at":"2026-06-12T10:00:00Z","read":false}`))
	})
	for _, p := range []string{"/v0/city/mardi_gras/mail/m1/reply", "/v0/city/mardi_gras/mail/m1/archive", "/v0/city/mardi_gras/mail/m1/read"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			recordCSRF(r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGCDriverFormulas(t *testing.T) {
	srv := gcMailServer(t, new(string))
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	names, err := d.Formulas(context.Background())
	if err != nil {
		t.Fatalf("Formulas: %v", err)
	}
	if len(names) != 2 || names[0] != "shiny" || names[1] != "quick" {
		t.Errorf("Formulas = %v, want [shiny quick]", names)
	}
}

func TestGCDriverMailInbox(t *testing.T) {
	srv := gcMailServer(t, new(string))
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	all, err := d.MailInbox(context.Background(), false)
	if err != nil {
		t.Fatalf("MailInbox: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("MailInbox(all) = %d messages, want 2", len(all))
	}
	m := all[0]
	if m.ID != "m1" || m.From != "mayor" || m.Subject != "hi" || m.Body != "yo" || m.Read {
		t.Errorf("message[0] = %+v", m)
	}
	if m.Time == "" {
		t.Error("message Time not populated from created_at")
	}
	unread, err := d.MailInbox(context.Background(), true)
	if err != nil {
		t.Fatalf("MailInbox(unread): %v", err)
	}
	if len(unread) != 1 {
		t.Errorf("MailInbox(unread) = %d, want 1 (status=unread filter)", len(unread))
	}
}

func TestGCDriverMailRead(t *testing.T) {
	srv := gcMailServer(t, new(string))
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	m, err := d.MailRead(context.Background(), "m1")
	if err != nil {
		t.Fatalf("MailRead: %v", err)
	}
	if m.ID != "m1" || m.Subject != "hi" {
		t.Errorf("MailRead = %+v", m)
	}
}

func TestGCDriverMailMutationsSendCSRFHeader(t *testing.T) {
	var csrf string
	srv := gcMailServer(t, &csrf)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	ctx := context.Background()

	mutations := map[string]func() error{
		"reply":   func() error { return d.MailReply(ctx, "m1", "body") },
		"send":    func() error { return d.MailSend(ctx, "you", "subj", "body") },
		"archive": func() error { return d.MailArchive(ctx, "m1") },
		"read":    func() error { return d.MailMarkRead(ctx, "m1") },
	}
	for name, fn := range mutations {
		csrf = ""
		if err := fn(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if csrf == "" {
			t.Errorf("%s: X-GC-Request header not sent", name)
		}
	}
}

func TestGCDriverMailMarkAllRead(t *testing.T) {
	srv := gcMailServer(t, new(string))
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	// One unread message (m1) in the unread set → one mark-read, no error.
	if err := d.MailMarkAllRead(context.Background()); err != nil {
		t.Fatalf("MailMarkAllRead: %v", err)
	}
}

func TestGCDriverMutationServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"csrf: missing header","status":403}`))
	}))
	t.Cleanup(srv.Close)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	err := d.MailArchive(context.Background(), "m1")
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "csrf") {
		t.Errorf("error = %v, want it to surface the problem detail", err)
	}
}

func TestGCEnabled(t *testing.T) {
	t.Setenv(EnvGCAPI, "")
	if GCEnabled() {
		t.Error("GCEnabled() = true with empty env")
	}
	t.Setenv(EnvGCAPI, "  ")
	if GCEnabled() {
		t.Error("GCEnabled() = true with whitespace env")
	}
	t.Setenv(EnvGCAPI, "http://x")
	if !GCEnabled() {
		t.Error("GCEnabled() = false with set env")
	}
}
