package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestAvailable(t *testing.T) {
	// Just verify it runs without panic; result depends on environment.
	_ = Available()
}

// withFakePath rewrites PATH to a temp dir containing fake executables named
// in `binaries`, then restores the original PATH on cleanup. Each fake binary
// is a no-op shell script with the executable bit set, so exec.LookPath
// resolves them deterministically regardless of what's installed locally.
func withFakePath(t *testing.T, binaries ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range binaries {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
	// Sanity-check: every requested binary must resolve through LookPath now.
	for _, name := range binaries {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("fake %s not found on rewritten PATH: %v", name, err)
		}
	}
}

func TestDetectRuntimeDefaultsToClaude(t *testing.T) {
	t.Setenv("MG_AGENT_RUNTIME", "")
	withFakePath(t, "claude", "cursor-agent")
	if got := DetectRuntime(); got != RuntimeClaude {
		t.Errorf("default detection should prefer claude, got %q", got)
	}
}

func TestDetectRuntimeFallsBackToCursor(t *testing.T) {
	t.Setenv("MG_AGENT_RUNTIME", "")
	withFakePath(t, "cursor-agent")
	if got := DetectRuntime(); got != RuntimeCursor {
		t.Errorf("expected cursor fallback when only cursor-agent is on PATH, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideToCursor(t *testing.T) {
	withFakePath(t, "claude", "cursor-agent")
	t.Setenv("MG_AGENT_RUNTIME", "cursor")
	if got := DetectRuntime(); got != RuntimeCursor {
		t.Errorf("MG_AGENT_RUNTIME=cursor should select cursor even when claude is on PATH, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideAcceptsCursorAgentAlias(t *testing.T) {
	withFakePath(t, "claude", "cursor-agent")
	t.Setenv("MG_AGENT_RUNTIME", "cursor-agent")
	if got := DetectRuntime(); got != RuntimeCursor {
		t.Errorf("MG_AGENT_RUNTIME=cursor-agent should select cursor, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideToClaude(t *testing.T) {
	withFakePath(t, "claude", "cursor-agent")
	t.Setenv("MG_AGENT_RUNTIME", "CLAUDE")
	if got := DetectRuntime(); got != RuntimeClaude {
		t.Errorf("MG_AGENT_RUNTIME=CLAUDE (case-insensitive) should select claude, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideFallsBackWhenBinaryMissing(t *testing.T) {
	// User asks for cursor, but only claude is installed.
	withFakePath(t, "claude")
	t.Setenv("MG_AGENT_RUNTIME", "cursor")
	if got := DetectRuntime(); got != RuntimeClaude {
		t.Errorf("expected fallback to claude when cursor-agent is missing, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideUnknownValueIgnored(t *testing.T) {
	withFakePath(t, "claude", "cursor-agent")
	t.Setenv("MG_AGENT_RUNTIME", "copilot")
	if got := DetectRuntime(); got != RuntimeClaude {
		t.Errorf("unknown MG_AGENT_RUNTIME value should fall through to default order, got %q", got)
	}
}

func TestDetectRuntimeNoRuntimeAvailable(t *testing.T) {
	t.Setenv("MG_AGENT_RUNTIME", "")
	withFakePath(t /* no fakes */)
	if got := DetectRuntime(); got != "" {
		t.Errorf("expected empty Runtime when neither binary is on PATH, got %q", got)
	}
}

func TestDetectRuntimeFallsBackToCodex(t *testing.T) {
	t.Setenv("MG_AGENT_RUNTIME", "")
	withFakePath(t, "codex")
	if got := DetectRuntime(); got != RuntimeCodex {
		t.Errorf("expected codex when it is the only binary on PATH, got %q", got)
	}
}

func TestDetectRuntimeDefaultOrderPrefersCursorOverCodex(t *testing.T) {
	t.Setenv("MG_AGENT_RUNTIME", "")
	withFakePath(t, "cursor-agent", "codex")
	if got := DetectRuntime(); got != RuntimeCursor {
		t.Errorf("default order should pick cursor-agent before codex, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideToCodex(t *testing.T) {
	withFakePath(t, "claude", "cursor-agent", "codex")
	t.Setenv("MG_AGENT_RUNTIME", "codex")
	if got := DetectRuntime(); got != RuntimeCodex {
		t.Errorf("MG_AGENT_RUNTIME=codex should select codex even when claude is on PATH, got %q", got)
	}
}

func TestDetectRuntimeEnvOverrideCodexFallsBackWhenMissing(t *testing.T) {
	withFakePath(t, "claude")
	t.Setenv("MG_AGENT_RUNTIME", "codex")
	if got := DetectRuntime(); got != RuntimeClaude {
		t.Errorf("expected fallback to claude when codex is missing, got %q", got)
	}
}

func TestCommandCodexArgs(t *testing.T) {
	withFakePath(t, "codex")
	t.Setenv("MG_AGENT_RUNTIME", "codex")

	cmd := Command("hello world", "/tmp/project")
	if cmd.Dir != "/tmp/project" {
		t.Errorf("expected Dir=%q, got %q", "/tmp/project", cmd.Dir)
	}
	want := []string{"codex", "--sandbox", "workspace-write", "-a", "on-request", "-C", "/tmp/project", "hello world"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(cmd.Args), cmd.Args)
	}
	for i, w := range want {
		if cmd.Args[i] != w {
			t.Errorf("arg[%d] = %q, want %q", i, cmd.Args[i], w)
		}
	}
}

func TestRuntimeLabel(t *testing.T) {
	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimeClaude, "Claude Code"},
		{RuntimeCursor, "Cursor"},
		{RuntimeCodex, "Codex"},
		{Runtime(""), "unknown"},
		{Runtime("frobnicate"), "unknown"},
	}
	for _, tc := range tests {
		got := tc.runtime.RuntimeLabel()
		if got != tc.want {
			t.Errorf("Runtime(%q).RuntimeLabel() = %q, want %q", tc.runtime, got, tc.want)
		}
	}
}

func TestBuildPromptFull(t *testing.T) {
	now := time.Now()
	issue := data.Issue{
		ID:                 "mg-001",
		Title:              "Deploy authentication service",
		Description:        "Set up OAuth2 flow for the API gateway.",
		Status:             data.StatusOpen,
		Priority:           data.PriorityCritical,
		IssueType:          data.TypeFeature,
		Owner:              "alice",
		Assignee:           "bob",
		CreatedAt:          now,
		UpdatedAt:          now,
		Notes:              "Needs review from security team.",
		AcceptanceCriteria: "All endpoints require valid JWT.",
		Dependencies: []data.Dependency{
			{IssueID: "mg-001", DependsOnID: "mg-002", Type: "blocks"},
		},
	}

	blocker := data.Issue{
		ID:        "mg-002",
		Title:     "Set up CI pipeline",
		Status:    data.StatusInProgress,
		Priority:  data.PriorityHigh,
		IssueType: data.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}

	issueMap := map[string]*data.Issue{
		"mg-001": &issue,
		"mg-002": &blocker,
	}

	deps := issue.EvaluateDependencies(issueMap, data.DefaultBlockingTypes)
	prompt := BuildPrompt(issue, deps, issueMap)

	for _, want := range []string{
		"mg-001",
		"Deploy authentication service",
		"Set up OAuth2 flow",
		"Owner: alice",
		"Assignee: bob",
		"### Notes",
		"Needs review from security team.",
		"### Acceptance Criteria",
		"All endpoints require valid JWT.",
		"Blocked by: mg-002",
		"Set up CI pipeline",
		"bd update mg-001 --status=in_progress",
		"bd close mg-001",
		"P0",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n\nGot:\n%s", want, prompt)
		}
	}
}

func TestBuildPromptMinimal(t *testing.T) {
	now := time.Now()
	issue := data.Issue{
		ID:        "mg-010",
		Title:     "Fix typo in README",
		Status:    data.StatusOpen,
		Priority:  data.PriorityBacklog,
		IssueType: data.TypeChore,
		CreatedAt: now,
		UpdatedAt: now,
	}

	issueMap := map[string]*data.Issue{"mg-010": &issue}
	deps := issue.EvaluateDependencies(issueMap, data.DefaultBlockingTypes)
	prompt := BuildPrompt(issue, deps, issueMap)

	if !strings.Contains(prompt, "mg-010") {
		t.Error("prompt missing issue ID")
	}
	if !strings.Contains(prompt, "Fix typo in README") {
		t.Error("prompt missing title")
	}

	// Optional sections should be absent.
	for _, absent := range []string{
		"### Notes",
		"### Acceptance Criteria",
		"### Dependencies",
		"Owner:",
		"Assignee:",
	} {
		if strings.Contains(prompt, absent) {
			t.Errorf("prompt should not contain %q for minimal issue\n\nGot:\n%s", absent, prompt)
		}
	}
}

func TestBuildPromptDependencies(t *testing.T) {
	now := time.Now()
	issue := data.Issue{
		ID:        "mg-020",
		Title:     "Main task",
		Status:    data.StatusOpen,
		Priority:  data.PriorityMedium,
		IssueType: data.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
		Dependencies: []data.Dependency{
			{IssueID: "mg-020", DependsOnID: "mg-021", Type: "blocks"},
			{IssueID: "mg-020", DependsOnID: "mg-022", Type: "blocks"},
			{IssueID: "mg-020", DependsOnID: "mg-ghost", Type: "blocks"},
			{IssueID: "mg-020", DependsOnID: "mg-023", Type: "related-to"},
		},
	}

	blocking := data.Issue{
		ID: "mg-021", Title: "Open blocker", Status: data.StatusOpen,
		Priority: data.PriorityMedium, IssueType: data.TypeTask,
		CreatedAt: now, UpdatedAt: now,
	}
	resolved := data.Issue{
		ID: "mg-022", Title: "Done blocker", Status: data.StatusClosed,
		Priority: data.PriorityMedium, IssueType: data.TypeTask,
		CreatedAt: now, UpdatedAt: now,
	}
	related := data.Issue{
		ID: "mg-023", Title: "Related item", Status: data.StatusOpen,
		Priority: data.PriorityLow, IssueType: data.TypeTask,
		CreatedAt: now, UpdatedAt: now,
	}

	issueMap := map[string]*data.Issue{
		"mg-020": &issue,
		"mg-021": &blocking,
		"mg-022": &resolved,
		"mg-023": &related,
	}

	deps := issue.EvaluateDependencies(issueMap, data.DefaultBlockingTypes)
	prompt := BuildPrompt(issue, deps, issueMap)

	for _, want := range []string{
		"Blocked by: mg-021 (Open blocker)",
		"Missing: mg-ghost (not found)",
		"Resolved: mg-022 (Done blocker) -- closed",
		"Related: mg-023 (Related item) -- related-to",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n\nGot:\n%s", want, prompt)
		}
	}
}

func TestCommandDir(t *testing.T) {
	cmd := Command("hello world", "/tmp/project")

	if cmd.Dir != "/tmp/project" {
		t.Errorf("expected Dir=%q, got %q", "/tmp/project", cmd.Dir)
	}

	rt := DetectRuntime()
	switch rt {
	case RuntimeClaude:
		if len(cmd.Args) != 2 {
			t.Fatalf("expected 2 args for claude, got %d: %v", len(cmd.Args), cmd.Args)
		}
		if cmd.Args[0] != "claude" {
			t.Errorf("expected Args[0]=%q, got %q", "claude", cmd.Args[0])
		}
		if cmd.Args[1] != "hello world" {
			t.Errorf("expected Args[1]=%q, got %q", "hello world", cmd.Args[1])
		}
	case RuntimeCursor:
		if len(cmd.Args) != 4 {
			t.Fatalf("expected 4 args for cursor-agent, got %d: %v", len(cmd.Args), cmd.Args)
		}
		if cmd.Args[0] != "cursor-agent" {
			t.Errorf("expected Args[0]=%q, got %q", "cursor-agent", cmd.Args[0])
		}
		if cmd.Args[1] != "-f" || cmd.Args[2] != "-p" {
			t.Errorf("expected [-f -p] flags, got %v", cmd.Args[1:3])
		}
		if cmd.Args[3] != "hello world" {
			t.Errorf("expected Args[3]=%q, got %q", "hello world", cmd.Args[3])
		}
	case RuntimeCodex:
		if len(cmd.Args) != 8 {
			t.Fatalf("expected 8 args for codex, got %d: %v", len(cmd.Args), cmd.Args)
		}
		if cmd.Args[0] != "codex" {
			t.Errorf("expected Args[0]=%q, got %q", "codex", cmd.Args[0])
		}
		if cmd.Args[len(cmd.Args)-1] != "hello world" {
			t.Errorf("expected prompt at last arg, got %q", cmd.Args[len(cmd.Args)-1])
		}
	default:
		t.Skip("no agent runtime on PATH")
	}
}
