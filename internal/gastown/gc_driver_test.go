package gastown

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if _, err := d.MailInbox(ctx, false); !errors.Is(err, ErrUnsupported) {
		t.Errorf("MailInbox err = %v, want ErrUnsupported", err)
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
