package data

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSourceLabelJSONL(t *testing.T) {
	tests := []struct {
		name string
		src  Source
		want string
	}{
		{
			name: "JSONL with path",
			src:  Source{Mode: SourceJSONL, Path: "/foo/.beads/issues.jsonl"},
			want: "issues.jsonl",
		},
		{
			name: "JSONL empty path",
			src:  Source{Mode: SourceJSONL},
			want: "issues.jsonl",
		},
		{
			name: "CLI mode",
			src:  Source{Mode: SourceCLI},
			want: "bd list",
		},
		{
			name: "CLI mode ignores path",
			src:  Source{Mode: SourceCLI, Path: "/foo/bar"},
			want: "bd list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.src.Label()
			if got != tt.want {
				t.Errorf("Source.Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckBdVersionKnownBroken(t *testing.T) {
	got := parseBdVersionWarning("bd version 0.59.0")
	if got == "" {
		t.Fatal("expected warning for v0.59.0, got empty string")
	}
	if got != "bd v0.59.0 has a known bug where --json is ignored; upgrade to v0.60.0+" {
		t.Errorf("unexpected warning: %q", got)
	}
}

func TestCheckBdVersionOK(t *testing.T) {
	got := parseBdVersionWarning("bd version 0.58.0")
	if got != "" {
		t.Errorf("expected no warning for v0.58.0, got %q", got)
	}
}

func TestCheckBdVersionUnparseable(t *testing.T) {
	cases := []string{
		"",
		"garbled output here",
		"bd",
		"\x00\xff",
	}
	for _, input := range cases {
		got := parseBdVersionWarning(input)
		if got != "" {
			t.Errorf("parseBdVersionWarning(%q) = %q, want empty", input, got)
		}
	}
}

func TestBdListArgs(t *testing.T) {
	args := bdListArgs()
	got := strings.Join(args, " ")
	want := "list --json --limit 0 --all"
	if got != want {
		t.Fatalf("bdListArgs() = %q, want %q", got, want)
	}
}

func TestParseIssuesCLIOutputRejectsWrongSinglePrefix(t *testing.T) {
	out := mustMarshalIssues(t, []Issue{
		{ID: "vv-12", Title: "wrong project", Status: StatusOpen, Priority: PriorityMedium, IssueType: TypeBug},
	})

	_, err := parseIssuesCLIOutput(out, "mg")
	if err == nil {
		t.Fatal("expected prefix validation error, got nil")
	}
	if !strings.Contains(err.Error(), `expects "mg"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `"vv" issues`) {
		t.Fatalf("expected wrong prefix in error, got %v", err)
	}
}

func TestParseIssuesCLIOutputAllowsExpectedAndHQPrefixes(t *testing.T) {
	out := mustMarshalIssues(t, []Issue{
		{ID: "hq-1", Title: "hq item", Status: StatusOpen, Priority: PriorityLow, IssueType: TypeTask},
		{ID: "mg-2", Title: "local item", Status: StatusOpen, Priority: PriorityMedium, IssueType: TypeBug},
	})

	issues, err := parseIssuesCLIOutput(out, "mg")
	if err != nil {
		t.Fatalf("parseIssuesCLIOutput() error = %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("len(issues) = %d, want 2", len(issues))
	}
}

func TestFetchIssuesCLIUsesFlatWithRealBDInvocation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake bd test is not supported on Windows")
	}

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	beadsDir := filepath.Join(projectDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("issue-prefix: mg\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	argsPath := filepath.Join(tmpDir, "bd-args.txt")
	t.Setenv("FAKE_BD_ARGS_FILE", argsPath)
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	script := `#!/bin/sh
printf '%s
' "$@" > "$FAKE_BD_ARGS_FILE"
cat <<'EOF'
[{"id":"mg-2","title":"CLI issue","status":"open","priority":2,"issue_type":"task","created_at":"2026-03-01T00:00:00Z","created_by":"system","updated_at":"2026-03-01T00:00:00Z"}]
EOF
`
	fakeBD := filepath.Join(tmpDir, "bd")
	if err := os.WriteFile(fakeBD, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	issues, err := FetchIssuesCLI(projectDir)
	if err != nil {
		t.Fatalf("FetchIssuesCLI() error = %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "mg-2" {
		t.Fatalf("FetchIssuesCLI() returned %+v, want single mg-2 issue", issues)
	}

	argsRaw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	gotArgs := strings.Fields(string(argsRaw))
	wantArgs := bdListArgs()
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("argv len = %d, want %d (%q)", len(gotArgs), len(wantArgs), string(argsRaw))
	}
	for i, want := range wantArgs {
		if gotArgs[i] != want {
			t.Fatalf("argv[%d] = %q, want %q (full: %q)", i, gotArgs[i], want, string(argsRaw))
		}
	}
}

func mustMarshalIssues(t *testing.T, issues []Issue) []byte {
	t.Helper()
	out, err := json.Marshal(issues)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return out
}

func TestFetchContextHappy(t *testing.T) {
	contextJSON := `{
		"beads_dir": "/project/.beads",
		"repo_root": "/project",
		"is_redirected": false,
		"backend": "dolt",
		"dolt_mode": "embedded",
		"database": "beads",
		"role": "crew",
		"bd_version": "0.60.0"
	}`
	defer mockRun([]byte(contextJSON), nil)()
	ctx, err := FetchContext()
	if err != nil {
		t.Fatalf("FetchContext() error = %v", err)
	}
	if ctx.BeadsDir != "/project/.beads" {
		t.Errorf("BeadsDir = %q", ctx.BeadsDir)
	}
	if ctx.Backend != "dolt" {
		t.Errorf("Backend = %q", ctx.Backend)
	}
	if ctx.BdVersion != "0.60.0" {
		t.Errorf("BdVersion = %q", ctx.BdVersion)
	}
}

func TestFetchContextExecError(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()
	_, err := FetchContext()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchContextMalformedJSON(t *testing.T) {
	defer mockRun([]byte(`{bad json`), nil)()
	_, err := FetchContext()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestFetchCurrentIssueIDHappy(t *testing.T) {
	defer mockRun([]byte(`{"id": "mg-42"}`), nil)()
	id, err := FetchCurrentIssueID()
	if err != nil {
		t.Fatalf("FetchCurrentIssueID() error = %v", err)
	}
	if id != "mg-42" {
		t.Errorf("ID = %q, want mg-42", id)
	}
}

func TestFetchCurrentIssueIDNoCurrent(t *testing.T) {
	defer mockRun(nil, errors.New("exit 1"))()
	id, err := FetchCurrentIssueID()
	if err != nil {
		t.Fatalf("FetchCurrentIssueID() should not return error for no-current, got %v", err)
	}
	if id != "" {
		t.Errorf("ID = %q, want empty", id)
	}
}

func TestFetchIssueDetailHappy(t *testing.T) {
	defer mockRun([]byte(bdShowDetailIssue), nil)()
	issue, err := FetchIssueDetail("proj-042")
	if err != nil {
		t.Fatalf("FetchIssueDetail() error = %v", err)
	}
	if issue.ID != "proj-042" {
		t.Errorf("ID = %q, want proj-042", issue.ID)
	}
	if issue.Notes == "" {
		t.Error("Notes should not be empty")
	}
}

func TestFetchIssueDetailEmptyArray(t *testing.T) {
	defer mockRun([]byte(`[]`), nil)()
	_, err := FetchIssueDetail("proj-999")
	if err == nil {
		t.Fatal("expected error for empty array, got nil")
	}
}

func TestFetchIssueDetailExecError(t *testing.T) {
	defer mockRun(nil, errors.New("not found"))()
	_, err := FetchIssueDetail("proj-999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchDoctorDiagnosticsHappy(t *testing.T) {
	doctorJSON := `{
		"overall_ok": false,
		"summary": "1 issue found",
		"diagnostics": [
			{
				"name": "dolt_server",
				"status": "error",
				"severity": "blocking",
				"category": "Core System",
				"explanation": "Dolt server is not running"
			}
		]
	}`
	// bd doctor exits non-zero when problems found but has valid stdout
	defer mockRun([]byte(doctorJSON), nil)()
	result, err := FetchDoctorDiagnostics()
	if err != nil {
		t.Fatalf("FetchDoctorDiagnostics() error = %v", err)
	}
	if result.OK {
		t.Error("OK should be false")
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result.Diagnostics))
	}
}

func TestFetchDoctorDiagnosticsExecErrorNoOutput(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()
	_, err := FetchDoctorDiagnostics()
	if err == nil {
		t.Fatal("expected error when no output, got nil")
	}
}
