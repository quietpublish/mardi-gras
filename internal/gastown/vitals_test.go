package gastown

import (
	"errors"
	"testing"
)

func TestParseVitalsHappyPath(t *testing.T) {
	raw := `Dolt Servers
  * :13409  production  PID 15103
    8.0 MB  1/1000 conn  0ms
Databases
  mardi-gras: 42 open, 18 closed
Backups
  Local:  2026-03-01 12:00 (1h ago)
  JSONL:  2026-03-01 11:30 (1.5h ago)`

	v := ParseVitals(raw)

	if v.Raw != "" {
		t.Errorf("expected Raw to be empty on successful parse, got %q", v.Raw)
	}

	if len(v.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(v.Servers))
	}
	s := v.Servers[0]
	if s.Port != ":13409" {
		t.Errorf("expected port :13409, got %q", s.Port)
	}
	if s.Label != "production" {
		t.Errorf("expected label production, got %q", s.Label)
	}
	if s.PID != 15103 {
		t.Errorf("expected PID 15103, got %d", s.PID)
	}
	if !s.Running {
		t.Error("expected server to be running")
	}
	if s.DiskUsage != "8.0 MB" {
		t.Errorf("expected disk usage '8.0 MB', got %q", s.DiskUsage)
	}
	if s.Connections != "1/1000 conn" {
		t.Errorf("expected connections '1/1000 conn', got %q", s.Connections)
	}
	if s.Latency != "0ms" {
		t.Errorf("expected latency '0ms', got %q", s.Latency)
	}

	if !v.Backups.LocalOK {
		t.Error("expected local backup to be OK")
	}
	if !v.Backups.JSONLOK {
		t.Error("expected JSONL backup to be OK")
	}
	if v.Backups.LocalLabel == "" {
		t.Error("expected local label to be non-empty")
	}
	if v.Backups.JSONLLabel == "" {
		t.Error("expected JSONL label to be non-empty")
	}
}

func TestParseVitalsServerDown(t *testing.T) {
	raw := `Dolt Servers
  * :13409  production  stopped
Backups
  Local:  not found
  JSONL:  not available`

	v := ParseVitals(raw)

	if len(v.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(v.Servers))
	}
	if v.Servers[0].Running {
		t.Error("expected server to be stopped")
	}
	if v.Servers[0].Port != ":13409" {
		t.Errorf("expected port :13409, got %q", v.Servers[0].Port)
	}

	if v.Backups.LocalOK {
		t.Error("expected local backup NOT OK")
	}
	if v.Backups.JSONLOK {
		t.Error("expected JSONL backup NOT OK")
	}
}

func TestParseVitalsBackupsNotConfigured(t *testing.T) {
	raw := `Dolt Servers
  * :3307  default  PID 999
    12 GB  5/1000 conn  2ms
Backups
  Local:  missing
  JSONL:  error: permission denied`

	v := ParseVitals(raw)

	if v.Backups.LocalOK {
		t.Error("expected local backup NOT OK for 'missing'")
	}
	if v.Backups.JSONLOK {
		t.Error("expected JSONL backup NOT OK for 'error'")
	}
}

func TestParseVitalsUnknownFormat(t *testing.T) {
	raw := `Something unexpected
here that does not match
any known section format`

	v := ParseVitals(raw)

	if v.Raw == "" {
		t.Error("expected Raw to contain fallback text")
	}
	if len(v.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(v.Servers))
	}
}

func TestParseVitalsEmpty(t *testing.T) {
	v := ParseVitals("")
	if len(v.Servers) != 0 {
		t.Errorf("expected 0 servers for empty input, got %d", len(v.Servers))
	}
	if v.Raw != "" {
		t.Errorf("expected empty Raw for empty input, got %q", v.Raw)
	}
}

func TestParseVitalsMultipleServers(t *testing.T) {
	raw := `Dolt Servers
  * :13409  production  PID 15103
    8.0 MB  1/1000 conn  0ms
  * :3307  development  PID 2001
    120 MB  3/1000 conn  1ms
Backups
  Local:  2026-03-01 12:00 (1h ago)
  JSONL:  2026-03-01 11:30 (1.5h ago)`

	v := ParseVitals(raw)

	if len(v.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(v.Servers))
	}
	if v.Servers[0].Port != ":13409" {
		t.Errorf("first server port: expected :13409, got %q", v.Servers[0].Port)
	}
	if v.Servers[1].Port != ":3307" {
		t.Errorf("second server port: expected :3307, got %q", v.Servers[1].Port)
	}
	if v.Servers[1].Label != "development" {
		t.Errorf("second server label: expected development, got %q", v.Servers[1].Label)
	}
}

func TestParseVitalsUnicodeBullets(t *testing.T) {
	raw := `Dolt Servers
  ● :3307  production  PID 54898  8.0 MB  1/1000 conn  0s
  ○ :13485 test zombie PID 84619

Databases (4 registered, 2 orphan)
  Rig          Total  Open  Closed     %
  beads_hq         0     0       0     -
  beads_mg        48    12      33   68%

Backups
  Local:  not found
  JSONL:  not available`

	v := ParseVitals(raw)

	if v.Raw != "" {
		t.Errorf("expected Raw to be empty on successful parse, got %q", v.Raw)
	}

	if len(v.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(v.Servers))
	}

	s0 := v.Servers[0]
	if s0.Port != ":3307" {
		t.Errorf("server 0 port: expected :3307, got %q", s0.Port)
	}
	if s0.Label != "production" {
		t.Errorf("server 0 label: expected production, got %q", s0.Label)
	}
	if s0.PID != 54898 {
		t.Errorf("server 0 PID: expected 54898, got %d", s0.PID)
	}
	if !s0.Running {
		t.Error("server 0: expected running (● bullet)")
	}
	if s0.DiskUsage != "8.0 MB" {
		t.Errorf("server 0 disk: expected '8.0 MB', got %q", s0.DiskUsage)
	}

	s1 := v.Servers[1]
	if s1.Port != ":13485" {
		t.Errorf("server 1 port: expected :13485, got %q", s1.Port)
	}
	if s1.Running {
		t.Error("server 1: expected stopped (○ bullet)")
	}
	if s1.PID != 84619 {
		t.Errorf("server 1 PID: expected 84619, got %d", s1.PID)
	}

	if v.Backups.LocalOK {
		t.Error("expected local backup NOT OK")
	}
	if v.Backups.JSONLOK {
		t.Error("expected JSONL backup NOT OK")
	}
}

func TestFetchVitalsHappy(t *testing.T) {
	vitalsOutput := "Dolt Servers:\n  ● :3307 beads PID 1234 42 MB 5 conn 2ms\nBackups:\n  Local: 10 min ago\n  JSONL: 5 min ago\n"
	defer mockRun([]byte(vitalsOutput), nil)()
	vitals, err := FetchVitals()
	if err != nil {
		t.Fatalf("FetchVitals() error = %v", err)
	}
	if len(vitals.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(vitals.Servers))
	}
	if !vitals.Servers[0].Running {
		t.Error("expected server to be running")
	}
}

func TestFetchVitalsExecError(t *testing.T) {
	defer mockRun(nil, errors.New("gt not found"))()
	_, err := FetchVitals()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
