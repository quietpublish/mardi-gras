package gastown

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/gastown/gcclient"
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
	if _, err := d.Vitals(ctx); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Vitals err = %v, want ErrUnsupported", err)
	}
	if err := d.Unsling(ctx, "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Unsling err = %v, want ErrUnsupported", err)
	}
	if err := d.CascadeClose(ctx, "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CascadeClose err = %v, want ErrUnsupported", err)
	}
	if err := d.ConvoyLand(ctx, "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ConvoyLand err = %v, want ErrUnsupported", err)
	}
}

func TestGCDriverSlingRequiresTarget(t *testing.T) {
	d, _ := NewGCDriver("http://127.0.0.1:8080", "")
	err := d.Sling(context.Background(), SlingRequest{IssueIDs: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "target") {
		t.Errorf("Sling without target err = %v, want a 'target required' error", err)
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
	// An explicit http(s) URL is used verbatim (trailing slash trimmed).
	t.Setenv(EnvGCAPI, "http://10.0.0.5:9000/")
	if got := GCBaseURL(); got != "http://10.0.0.5:9000" {
		t.Errorf("GCBaseURL = %q, want trailing slash trimmed", got)
	}
}

func TestGCParseSupervisorLog(t *testing.T) {
	const line = "gc supervisor: starting\nSupervisor API listening on http://127.0.0.1:8372\nmore\n"
	if got := gcParseSupervisorLog(line); got != "http://127.0.0.1:8372" {
		t.Errorf("parse = %q, want http://127.0.0.1:8372", got)
	}
	// A restart appends a new line on a new port — the latest wins.
	if got := gcParseSupervisorLog(line + "Supervisor API listening on http://127.0.0.1:9999\n"); got != "http://127.0.0.1:9999" {
		t.Errorf("parse latest = %q, want http://127.0.0.1:9999", got)
	}
	if got := gcParseSupervisorLog("no listen line"); got != "" {
		t.Errorf("parse none = %q, want empty", got)
	}
}

// TestSelectDriver covers the one branch of the public entry point that does
// not depend on what is installed on the host: an explicit MG_GC_API.
//
// The unset case is deliberately NOT asserted here. SelectDriver() calls
// Detect(), so its answer legitimately varies with the machine — on a host with
// gc installed and no gt it is now "gascity", which is the whole point of the
// evidence precedence. Those rules are pinned deterministically in
// TestSelectDriverPrefersEvidence, which injects the environment instead.
func TestSelectDriver(t *testing.T) {
	t.Setenv(EnvGCAPI, "http://127.0.0.1:8080")
	if d := SelectDriver(); d.Backend() != "gascity" {
		t.Errorf("with %s: Backend() = %q, want gascity", EnvGCAPI, d.Backend())
	}
}

// TestSelectDriverPrefersEvidence pins the precedence rules. Before this,
// SelectDriver branched on MG_GC_API alone, so a user inside a real Gas City
// workspace with gc on PATH and no gt installed got a Gas Town driver that
// shelled out to a binary that was not there — while mg had already computed
// the evidence in detect.go and never read it.
func TestSelectDriverPrefersEvidence(t *testing.T) {
	t.Setenv(EnvGCAPI, "")
	t.Setenv(EnvGCCity, "")

	cases := []struct {
		name string
		env  Env
		want string
	}{
		{"no evidence at all falls back to gastown", Env{}, "gastown"},
		{"gt on PATH wins", Env{Available: true, GCAvailable: true, GCCityPath: "/tmp/city"}, "gastown"},
		{"inside a Gas Town session wins", Env{Active: true, GCCityPath: "/tmp/city"}, "gastown"},
		{"gc on PATH, no gt evidence", Env{GCAvailable: true}, "gascity"},
		{"city.toml above cwd, no gt evidence", Env{GCCityPath: "/tmp/city"}, "gascity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectDriver(tc.env).Backend(); got != tc.want {
				t.Errorf("selectDriver(%+v).Backend() = %q, want %q", tc.env, got, tc.want)
			}
		})
	}
}

// TestSelectDriverExplicitAPIOutranksGasTownEvidence keeps MG_GC_API as the
// operator's override: naming a supervisor must win even inside a Gas Town rig.
func TestSelectDriverExplicitAPIOutranksGasTownEvidence(t *testing.T) {
	t.Setenv(EnvGCAPI, "http://127.0.0.1:8080")
	t.Setenv(EnvGCCity, "")
	env := Env{Available: true, Active: true}
	if got := selectDriver(env).Backend(); got != "gascity" {
		t.Errorf("with %s set: Backend() = %q, want gascity", EnvGCAPI, got)
	}
}

// gcMailServer answers the Phase 3 mail/formula endpoints for a pinned city
// and records the X-GC-Request header seen on the most recent mutation.
func gcMailServer(t *testing.T, lastCSRF *string) *httptest.Server {
	t.Helper()
	recordCSRF := func(r *http.Request) { *lastCSRF = r.Header.Get("X-GC-Request") }
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/formulas", func(w http.ResponseWriter, r *http.Request) {
		// The endpoint 400s without a scope; mg must send city scope.
		if q := r.URL.Query(); q.Get("scope_kind") != "city" || q.Get("scope_ref") != "mardi_gras" {
			t.Errorf("formulas: missing/wrong scope params: %q", r.URL.RawQuery)
		}
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

// gcSessionServer answers the sessions list + submit/kill for a pinned city,
// recording the X-GC-Request header and the session id each mutation hit.
func gcSessionServer(t *testing.T, lastCSRF, hitID *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/sessions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// obsidian is running (id va-9); quartz is a stopped duplicate name.
		_, _ = w.Write([]byte(`{"items":[` +
			`{"id":"va-stale","session_name":"obsidian","title":"obsidian","state":"closed","provider":"claude","template":"t","created_at":"2026-06-12T10:00:00Z","attached":false,"running":false},` +
			`{"id":"va-9","session_name":"obsidian","title":"obsidian","state":"active","provider":"claude","template":"t","created_at":"2026-06-12T10:00:00Z","attached":true,"running":true}` +
			`],"total":2}`))
	})
	record := func(w http.ResponseWriter, r *http.Request, id string) {
		*lastCSRF = r.Header.Get("X-GC-Request")
		*hitID = id
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}
	mux.HandleFunc("/v0/city/mardi_gras/session/va-9/submit", func(w http.ResponseWriter, r *http.Request) { record(w, r, "va-9") })
	mux.HandleFunc("/v0/city/mardi_gras/session/va-9/kill", func(w http.ResponseWriter, r *http.Request) { record(w, r, "va-9") })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGCDriverNudge(t *testing.T) {
	var csrf, hit string
	srv := gcSessionServer(t, &csrf, &hit)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if err := d.Nudge(context.Background(), "obsidian", "wake up"); err != nil {
		t.Fatalf("Nudge: %v", err)
	}
	if hit != "va-9" { // resolved to the running session, not the stale one
		t.Errorf("submitted to session %q, want va-9 (the running match)", hit)
	}
	if csrf == "" {
		t.Error("X-GC-Request not sent on nudge")
	}
}

func TestGCDriverDecommission(t *testing.T) {
	var csrf, hit string
	srv := gcSessionServer(t, &csrf, &hit)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if err := d.Decommission(context.Background(), "obsidian"); err != nil {
		t.Fatalf("Decommission: %v", err)
	}
	if hit != "va-9" {
		t.Errorf("killed session %q, want va-9", hit)
	}
	if csrf == "" {
		t.Error("X-GC-Request not sent on decommission")
	}
}

func TestGCDriverNudgeNoSession(t *testing.T) {
	var csrf, hit string
	srv := gcSessionServer(t, &csrf, &hit)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	err := d.Nudge(context.Background(), "ghost", "hi")
	if err == nil || !strings.Contains(err.Error(), "no session") {
		t.Errorf("Nudge(unknown) err = %v, want a 'no session' error", err)
	}
}

func TestGCSessionMatches(t *testing.T) {
	s := func(v string) *string { return &v }
	sess := gcclient.SessionResponse{SessionName: "mayor", Title: "Mayor Agent", Alias: s("hizzoner")}
	for _, want := range []string{"mayor", "MAYOR", "Mayor Agent", "hizzoner"} {
		if !gcSessionMatches(sess, strings.ToLower(want)) {
			t.Errorf("gcSessionMatches should match %q", want)
		}
	}
	if gcSessionMatches(sess, "deacon") {
		t.Error("gcSessionMatches should not match unrelated name")
	}
	if gcSessionMatches(sess, "") {
		t.Error("gcSessionMatches should not match empty target")
	}
}

func TestGCDriverConvoyListAndStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/convoys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"cv-1","title":"Release batch","status":"open","issue_type":"convoy"}],"total":1}`))
	})
	mux.HandleFunc("/v0/city/mardi_gras/convoy/cv-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"convoy":{"id":"cv-1","title":"Release batch","status":"open","issue_type":"convoy"},` +
			`"children":[{"id":"b-1","title":"task A","status":"closed","issue_type":"task","assignee":"obsidian"},` +
			`{"id":"b-2","title":"task B","status":"open","issue_type":"task"}],"progress":{"total":2,"closed":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	ctx := context.Background()

	list, err := d.ConvoyList(ctx)
	if err != nil {
		t.Fatalf("ConvoyList: %v", err)
	}
	if len(list) != 1 || list[0].ID != "cv-1" || list[0].Title != "Release batch" || list[0].Status != "open" {
		t.Errorf("ConvoyList = %+v", list)
	}
	// ConvoyList enriches each convoy with progress from its detail.
	if list[0].Total != 2 || list[0].Completed != 1 {
		t.Errorf("ConvoyList progress = %d/%d, want 1/2", list[0].Completed, list[0].Total)
	}

	cd, err := d.ConvoyStatus(ctx, "cv-1")
	if err != nil {
		t.Fatalf("ConvoyStatus: %v", err)
	}
	if cd.Total != 2 || cd.Completed != 1 || cd.ProgressPct != 50 {
		t.Errorf("progress = total %d completed %d pct %v", cd.Total, cd.Completed, cd.ProgressPct)
	}
	if len(cd.Tracked) != 2 || cd.Tracked[0].ID != "b-1" || cd.Tracked[0].Worker != "obsidian" || cd.Tracked[0].Status != "closed" {
		t.Errorf("tracked = %+v", cd.Tracked)
	}
}

func TestGCDriverConvoyCreateClose(t *testing.T) {
	var csrf string
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/convoys", func(w http.ResponseWriter, r *http.Request) {
		csrf = r.Header.Get("X-GC-Request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"cv-new","title":"batch","status":"open","issue_type":"convoy"}`))
	})
	mux.HandleFunc("/v0/city/mardi_gras/convoy/cv-new/close", func(w http.ResponseWriter, r *http.Request) {
		csrf = r.Header.Get("X-GC-Request")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	ctx := context.Background()

	id, err := d.ConvoyCreate(ctx, "batch", []string{"b-1", "b-2"})
	if err != nil {
		t.Fatalf("ConvoyCreate: %v", err)
	}
	if id != "cv-new" {
		t.Errorf("ConvoyCreate id = %q, want cv-new", id)
	}
	if csrf == "" {
		t.Error("X-GC-Request not sent on convoy create")
	}
	csrf = ""
	if err := d.ConvoyClose(ctx, "cv-new"); err != nil {
		t.Fatalf("ConvoyClose: %v", err)
	}
	if csrf == "" {
		t.Error("X-GC-Request not sent on convoy close")
	}
}

func TestGCDriverSling(t *testing.T) {
	var csrf, gotTarget, gotBead string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrf = r.Header.Get("X-GC-Request")
		var body struct {
			Target string `json:"target"`
			Bead   string `json:"bead"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotTarget, gotBead = body.Target, body.Bead
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	err := d.Sling(context.Background(), SlingRequest{IssueIDs: []string{"b-7"}, Target: "mayor", Formula: "shiny"})
	if err != nil {
		t.Fatalf("Sling: %v", err)
	}
	if gotTarget != "mayor" || gotBead != "b-7" {
		t.Errorf("sling body target=%q bead=%q, want mayor/b-7", gotTarget, gotBead)
	}
	if csrf == "" {
		t.Error("X-GC-Request not sent on sling")
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

// gcRespErr reads the raw problem+json body rather than a generated typed
// field, because which typed error field oapi-codegen emits changes when the
// spec is regenerated (a `default` response becomes explicit status codes).
func TestGCRespErrParsesProblemBody(t *testing.T) {
	body := []byte(`{"title":"Not Found","status":404,"detail":"city \"nope\" is not running"}`)
	got := gcRespErr(404, body)
	if !strings.Contains(got, `city "nope" is not running`) {
		t.Errorf("gcRespErr() = %q, want it to carry the problem detail", got)
	}
	if !strings.Contains(got, "404") {
		t.Errorf("gcRespErr() = %q, want it to carry the status code", got)
	}
}

func TestGCRespErrFallsBackToStatus(t *testing.T) {
	for _, body := range [][]byte{nil, {}, []byte("not json"), []byte(`{"detail":""}`)} {
		got := gcRespErr(503, body)
		if got != "status 503" {
			t.Errorf("gcRespErr(503, %q) = %q, want %q", body, got, "status 503")
		}
	}
}

func TestGCMutationErrSucceedsOn2xx(t *testing.T) {
	if err := gcMutationErr("gc nudge", 204, nil); err != nil {
		t.Errorf("gcMutationErr(204) = %v, want nil", err)
	}
	err := gcMutationErr("gc nudge", 422, []byte(`{"detail":"session is gone"}`))
	if err == nil || !strings.Contains(err.Error(), "session is gone") {
		t.Errorf("gcMutationErr(422) = %v, want an error carrying the detail", err)
	}
}

// --- Assign + ConvoyCreateFromEpic (Gas City v1.4.1 endpoints) --------------

// Gas City takes the assignee inline on create, so a bead is never briefly
// unowned the way a create-then-assign pair would leave it.
func TestGCDriverAssignCreatesBeadWithAssignee(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/beads", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-GC-Request") == "" {
			t.Error("create bead: missing X-GC-Request header")
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"mg-42","title":"wire the thing","status":"open"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d, err := NewGCDriver(srv.URL, "mardi_gras")
	if err != nil {
		t.Fatalf("NewGCDriver: %v", err)
	}
	out, err := d.Assign(context.Background(), "obsidian", "wire the thing", "task", "1", "ui", false)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if !strings.Contains(out, "mg-42") || !strings.Contains(out, "obsidian") {
		t.Errorf("Assign() = %q, want it to name the new bead and the crew member", out)
	}
	if gotBody["assignee"] != "obsidian" {
		t.Errorf("assignee = %v, want obsidian", gotBody["assignee"])
	}
	if gotBody["title"] != "wire the thing" {
		t.Errorf("title = %v", gotBody["title"])
	}
	if gotBody["type"] != "task" {
		t.Errorf("type = %v, want task", gotBody["type"])
	}
	// gt passes priority as a CLI string; the API wants a number.
	if p, ok := gotBody["priority"].(float64); !ok || p != 1 {
		t.Errorf("priority = %v, want numeric 1", gotBody["priority"])
	}
}

// A non-numeric priority is dropped rather than failing the whole create — gt
// accepts free-form strings there.
func TestGCDriverAssignDropsUnparseablePriority(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/beads", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"mg-43","title":"t","status":"open"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if _, err := d.Assign(context.Background(), "obsidian", "t", "", "high", "", false); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if _, present := gotBody["priority"]; present {
		t.Errorf("priority should be omitted when unparseable, got %v", gotBody["priority"])
	}
}

// ConvoyCreateFromEpic walks the epic's graph and must not enrol the epic itself.
func TestGCDriverConvoyCreateFromEpicExcludesRoot(t *testing.T) {
	var convoyBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/beads/graph/mg-epic", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"root":{"id":"mg-epic","title":"epic","status":"open"},
			"beads":[{"id":"mg-epic","title":"epic","status":"open"},
			         {"id":"mg-1","title":"one","status":"open"},
			         {"id":"mg-2","title":"two","status":"open"}],
			"deps":[]}`))
	})
	mux.HandleFunc("/v0/city/mardi_gras/convoys", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&convoyBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"cv-9","title":"parade"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	id, err := d.ConvoyCreateFromEpic(context.Background(), "parade", "mg-epic")
	if err != nil {
		t.Fatalf("ConvoyCreateFromEpic: %v", err)
	}
	if id != "cv-9" {
		t.Errorf("id = %q, want cv-9", id)
	}
	items, _ := convoyBody["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v, want exactly the two members (epic excluded)", convoyBody["items"])
	}
	for _, it := range items {
		if it == "mg-epic" {
			t.Error("the epic itself must not be enrolled in its own convoy")
		}
	}
}

// An epic with no members is a clear error, not an empty convoy.
func TestGCDriverConvoyCreateFromEpicEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/mardi_gras/beads/graph/mg-solo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"root":{"id":"mg-solo","title":"solo","status":"open"},
			"beads":[{"id":"mg-solo","title":"solo","status":"open"}],"deps":[]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d, _ := NewGCDriver(srv.URL, "mardi_gras")
	if _, err := d.ConvoyCreateFromEpic(context.Background(), "parade", "mg-solo"); err == nil {
		t.Fatal("expected an error for an epic with no members")
	}
}

// TestBothDriversFetchCommentsFromBeads pins that comments are Beads data, not
// orchestrator data. GCDriver used to return ErrUnsupported here, which cost
// Gas City users the whole comments panel for issues whose comments bd still
// held and mg could still read. Both drivers must issue the same `bd` call.
func TestBothDriversFetchCommentsFromBeads(t *testing.T) {
	const payload = `[{"id":"c1","author":"ada","text":"first","created_at":"2026-08-29T00:00:00Z"}]`

	for _, tc := range []struct {
		name   string
		driver Driver
	}{
		{"gastown", NewGTDriver()},
		{"gascity", &GCDriver{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, restore := mockRunCapture([]byte(payload), nil)
			defer restore()

			comments, err := tc.driver.Comments(context.Background(), "mg-1")
			if err != nil {
				t.Fatalf("Comments() err = %v, want nil", err)
			}
			if len(comments) != 1 {
				t.Fatalf("Comments() returned %d comments, want 1", len(comments))
			}
			if len(*calls) != 1 {
				t.Fatalf("made %d subprocess calls, want 1", len(*calls))
			}
			got := (*calls)[0]
			if got[0] != "bd" || got[1] != "comments" {
				t.Errorf("invoked %v, want a `bd comments` call", got)
			}
		})
	}
}
