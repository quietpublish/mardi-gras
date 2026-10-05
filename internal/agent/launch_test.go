package agent

import (
	"errors"
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
// is a writeShim no-op, so exec.LookPath resolves them deterministically
// regardless of what's installed locally. It also clears MG_AGENT_CMD: a seat
// that always launches through a wrapper must not decide these results. A test
// that wants a wrapper sets it after calling this. Returns the temp dir.
func withFakePath(t *testing.T, binaries ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range binaries {
		writeShim(t, dir, name)
	}
	t.Setenv("PATH", dir)
	t.Setenv(AgentCommandEnv, "")
	// Sanity-check: every requested binary must resolve through LookPath now.
	for _, name := range binaries {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("fake %s not found on rewritten PATH: %v", name, err)
		}
	}
	return dir
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

func TestDetectRuntimeWrapperAloneIsAnAgent(t *testing.T) {
	// A sandbox that exposes only the wrapper, no agent binary at all.
	withFakePath(t /* no fakes */)
	t.Setenv(AgentCommandEnv, writeShim(t, t.TempDir(), "agent-shim"))

	cases := map[string]Runtime{
		"":       RuntimeClaude, // no preference: the default flag set
		"claude": RuntimeClaude,
		"cursor": RuntimeCursor,
		"codex":  RuntimeCodex,
	}
	for pref, want := range cases {
		t.Setenv("MG_AGENT_RUNTIME", pref)
		if got := DetectRuntime(); got != want {
			t.Errorf("MG_AGENT_RUNTIME=%q with only a wrapper: DetectRuntime() = %q, want %q", pref, got, want)
		}
		if !Available() {
			t.Errorf("MG_AGENT_RUNTIME=%q: a configured wrapper must make agents available", pref)
		}
	}
}

func TestDetectRuntimeWrapperHonoursPreferenceOverPath(t *testing.T) {
	// The wrapper is what runs, so codex being absent from PATH does not
	// matter: the preference picks the flag set handed to the wrapper.
	withFakePath(t, "claude")
	t.Setenv(AgentCommandEnv, writeShim(t, t.TempDir(), "agent-shim"))
	t.Setenv("MG_AGENT_RUNTIME", "codex")
	if got := DetectRuntime(); got != RuntimeCodex {
		t.Errorf("wrapper + MG_AGENT_RUNTIME=codex should select codex, got %q", got)
	}
}

func TestDetectRuntimeWrapperWithoutPreferenceKeepsPathOrder(t *testing.T) {
	withFakePath(t, "cursor-agent")
	t.Setenv(AgentCommandEnv, writeShim(t, t.TempDir(), "agent-shim"))
	t.Setenv("MG_AGENT_RUNTIME", "")
	if got := DetectRuntime(); got != RuntimeCursor {
		t.Errorf("wrapper without a preference should keep the PATH detection order, got %q", got)
	}
}

func TestCommandCodexArgs(t *testing.T) {
	dir := withFakePath(t, "codex")
	t.Setenv("MG_AGENT_RUNTIME", "codex")

	cmd, err := Command("hello world", "/tmp/project")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if cmd.Dir != "/tmp/project" {
		t.Errorf("expected Dir=%q, got %q", "/tmp/project", cmd.Dir)
	}
	want := []string{filepath.Join(dir, "codex"), "--sandbox", "workspace-write", "-a", "on-request", "-C", "/tmp/project", "hello world"}
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

// writeShim drops an executable no-op script at dir/name and returns its path.
func writeShim(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write shim %s: %v", name, err)
	}
	return path
}

func TestAgentCommandDefaultsToRuntimeBinary(t *testing.T) {
	dir := withFakePath(t, "claude", "cursor-agent", "codex")
	cases := map[Runtime]string{
		RuntimeClaude: "claude",
		RuntimeCursor: "cursor-agent",
		RuntimeCodex:  "codex",
		// No runtime detected: the historical hardcoded default, kept so an
		// empty argv[0] never reaches tmux.
		Runtime(""): "claude",
	}
	for rt, name := range cases {
		got, err := agentCommand(rt)
		if err != nil {
			t.Fatalf("agentCommand(%q): %v", rt, err)
		}
		// Absolute for the same reason as a wrapper: tmux resolves a bare
		// name against the server's PATH, not mg's.
		if want := filepath.Join(dir, name); got != want {
			t.Errorf("agentCommand(%q) = %q, want %q", rt, got, want)
		}
	}
}

func TestAgentCommandMissingRuntimeBinaryIsAnError(t *testing.T) {
	withFakePath(t /* no fakes */)
	if got, err := agentCommand(RuntimeCursor); err == nil {
		t.Errorf("agentCommand with cursor-agent off PATH = %q, want an error", got)
	}
}

func TestAgentCommandQualifiesBareWrapperName(t *testing.T) {
	// The crux of the tmux path: split-window's command is resolved by the
	// tmux SERVER against the server's PATH, so a bare wrapper name must be
	// qualified here or the pane runs whatever the server finds first.
	dir := withFakePath(t /* no fakes */)
	shim := writeShim(t, dir, "agent-shim")
	t.Setenv(AgentCommandEnv, "agent-shim")

	got, err := agentCommand(RuntimeClaude)
	if err != nil {
		t.Fatalf("agentCommand(bare name): %v", err)
	}
	if got != shim {
		t.Errorf("agentCommand(bare name) = %q, want the resolved path %q", got, shim)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("agentCommand must return an absolute path, got %q", got)
	}
}

func TestAgentCommandAbsolutizesRelativeWrapperPath(t *testing.T) {
	// exec.LookPath returns a value containing a slash verbatim, so
	// "./agent-shim" stays relative unless it is made absolute here — and a
	// relative path means something else to the tmux server, whose cwd is not
	// mg's.
	dir := withFakePath(t /* no fakes */)
	shim := writeShim(t, dir, "agent-shim")
	t.Chdir(dir)
	t.Setenv(AgentCommandEnv, "./agent-shim")

	got, err := agentCommand(RuntimeClaude)
	if err != nil {
		t.Fatalf("agentCommand(./agent-shim): %v", err)
	}
	if got != shim {
		t.Errorf("agentCommand(./agent-shim) = %q, want %q", got, shim)
	}
}

func TestAgentCommandAcceptsWrapperFoundViaRelativePathEntry(t *testing.T) {
	// With "." on PATH, LookPath finds a bare name but reports exec.ErrDot
	// alongside the relative result. The user named this wrapper explicitly,
	// so it is accepted — and made absolute, which is what the tmux server
	// needs.
	dir := withFakePath(t /* no fakes */)
	shim := writeShim(t, dir, "agent-shim")
	t.Chdir(dir)
	t.Setenv("PATH", ".")
	t.Setenv(AgentCommandEnv, "agent-shim")

	got, err := agentCommand(RuntimeClaude)
	if err != nil {
		t.Fatalf("agentCommand(bare name via ./ on PATH): %v", err)
	}
	if got != shim {
		t.Errorf("agentCommand(bare name via ./ on PATH) = %q, want %q", got, shim)
	}
}

func TestAgentCommandAcceptsWhitespacePaddedValue(t *testing.T) {
	withFakePath(t /* no fakes */)
	shim := writeShim(t, t.TempDir(), "agent-shim")
	t.Setenv(AgentCommandEnv, "  "+shim+"\n")

	got, err := agentCommand(RuntimeClaude)
	if err != nil {
		t.Fatalf("agentCommand(padded value): %v", err)
	}
	if got != shim {
		t.Errorf("agentCommand should trim the configured value, got %q want %q", got, shim)
	}
}

func TestAgentCommandMissingWrapperFailsClosed(t *testing.T) {
	// claude is right there on PATH, and must NOT be used: the wrapper may be
	// the only thing keeping the agent off credentials or the network.
	withFakePath(t, "claude")
	t.Setenv(AgentCommandEnv, "definitely-not-a-binary-xyz")

	got, err := agentCommand(RuntimeClaude)
	if err == nil {
		t.Fatalf("unresolvable MG_AGENT_CMD must be an error, got %q", got)
	}
	if !strings.Contains(err.Error(), AgentCommandEnv) {
		t.Errorf("error should name %s so the toast is actionable, got %q", AgentCommandEnv, err)
	}
}

func TestAgentCommandNonExecutableWrapperFailsClosed(t *testing.T) {
	withFakePath(t, "claude")
	path := filepath.Join(t.TempDir(), "agent-shim")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(AgentCommandEnv, path)

	if got, err := agentCommand(RuntimeClaude); err == nil {
		t.Errorf("a non-executable MG_AGENT_CMD must be an error, got %q", got)
	}
}

func TestAgentCommandBlankValueIsUnset(t *testing.T) {
	dir := withFakePath(t, "claude")
	t.Setenv(AgentCommandEnv, "   ")

	got, err := agentCommand(RuntimeClaude)
	if err != nil {
		t.Fatalf("agentCommand with blank MG_AGENT_CMD: %v", err)
	}
	if want := filepath.Join(dir, "claude"); got != want {
		t.Errorf("a whitespace-only MG_AGENT_CMD is unset, got %q want %q", got, want)
	}
}

func TestAgentArgv(t *testing.T) {
	const bin, prompt, dir = "/opt/agent", "do it", "/tmp/project"
	tests := []struct {
		rt     Runtime
		inTmux bool
		want   []string
	}{
		{RuntimeClaude, false, []string{bin, prompt}},
		{RuntimeClaude, true, []string{bin, "--teammate-mode", "tmux", prompt}},
		{Runtime(""), true, []string{bin, "--teammate-mode", "tmux", prompt}},
		{RuntimeCursor, false, []string{bin, "-f", "-p", prompt}},
		{RuntimeCursor, true, []string{bin, "-f", "-p", prompt}},
		{RuntimeCodex, false, []string{bin, "--sandbox", "workspace-write", "-a", "on-request", "-C", dir, prompt}},
		{RuntimeCodex, true, []string{bin, "--no-alt-screen", "--sandbox", "workspace-write", "-a", "on-request", "-C", dir, prompt}},
	}
	for _, tc := range tests {
		got := agentArgv(tc.rt, bin, prompt, dir, tc.inTmux)
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("agentArgv(%q, inTmux=%v) = %q, want %q", tc.rt, tc.inTmux, got, tc.want)
		}
	}
}

func TestCodexCommand(t *testing.T) {
	t.Run("no wrapper, codex on PATH", func(t *testing.T) {
		dir := withFakePath(t, "codex")
		got, err := codexCommand()
		if err != nil {
			t.Fatalf("codexCommand: %v", err)
		}
		if want := filepath.Join(dir, "codex"); got != want {
			t.Errorf("codexCommand() = %q, want %q", got, want)
		}
	})
	t.Run("no wrapper, no codex", func(t *testing.T) {
		withFakePath(t, "claude")
		if _, err := codexCommand(); !errors.Is(err, ErrCodexUnavailable) {
			t.Errorf("codexCommand() error = %v, want ErrCodexUnavailable", err)
		}
	})
	t.Run("wrapper standing in for codex", func(t *testing.T) {
		withFakePath(t /* no fakes */)
		shim := writeShim(t, t.TempDir(), "agent-shim")
		t.Setenv(AgentCommandEnv, shim)
		t.Setenv("MG_AGENT_RUNTIME", "codex")
		got, err := codexCommand()
		if err != nil {
			t.Fatalf("codexCommand: %v", err)
		}
		if got != shim {
			t.Errorf("codexCommand() = %q, want the wrapper %q", got, shim)
		}
	})
	t.Run("wrapper standing in for another runtime", func(t *testing.T) {
		// codex IS on PATH, and must not be used behind the wrapper's back.
		withFakePath(t, "codex")
		t.Setenv(AgentCommandEnv, writeShim(t, t.TempDir(), "agent-shim"))
		t.Setenv("MG_AGENT_RUNTIME", "claude")
		got, err := codexCommand()
		if err == nil {
			t.Fatalf("codexCommand() = %q, want an error: the wrapper is configured for claude", got)
		}
		if !strings.Contains(err.Error(), AgentCommandEnv) {
			t.Errorf("error should name %s, got %q", AgentCommandEnv, err)
		}
	})
	t.Run("unresolvable wrapper standing in for codex", func(t *testing.T) {
		withFakePath(t, "codex")
		t.Setenv(AgentCommandEnv, "definitely-not-a-binary-xyz")
		t.Setenv("MG_AGENT_RUNTIME", "codex")
		if got, err := codexCommand(); err == nil {
			t.Errorf("codexCommand() = %q, want an error", got)
		}
	})
}

func TestCommandUsesAgentCommandWrapper(t *testing.T) {
	withFakePath(t, "claude")
	shim := writeShim(t, t.TempDir(), "agent-shim")
	t.Setenv(AgentCommandEnv, shim)

	cmd, err := Command("hello world", "/tmp/project")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{shim, "hello world"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(cmd.Args), cmd.Args)
	}
	for i, w := range want {
		if cmd.Args[i] != w {
			t.Errorf("arg[%d] = %q, want %q", i, cmd.Args[i], w)
		}
	}
	if cmd.Dir != "/tmp/project" {
		t.Errorf("expected Dir=%q, got %q", "/tmp/project", cmd.Dir)
	}
}

func TestCommandDir(t *testing.T) {
	// Pin the wrapper off: this test asserts the bare runtime binary, and an
	// ambient MG_AGENT_CMD (a seat that always routes through a launcher)
	// would otherwise decide the result.
	t.Setenv(AgentCommandEnv, "")

	cmd, err := Command("hello world", "/tmp/project")
	if err != nil {
		t.Skipf("no agent runtime on PATH: %v", err)
	}

	if cmd.Dir != "/tmp/project" {
		t.Errorf("expected Dir=%q, got %q", "/tmp/project", cmd.Dir)
	}

	rt := DetectRuntime()
	switch rt {
	case RuntimeClaude:
		if len(cmd.Args) != 2 {
			t.Fatalf("expected 2 args for claude, got %d: %v", len(cmd.Args), cmd.Args)
		}
		if filepath.Base(cmd.Args[0]) != "claude" || !filepath.IsAbs(cmd.Args[0]) {
			t.Errorf("expected Args[0] to be an absolute path to claude, got %q", cmd.Args[0])
		}
		if cmd.Args[1] != "hello world" {
			t.Errorf("expected Args[1]=%q, got %q", "hello world", cmd.Args[1])
		}
	case RuntimeCursor:
		if len(cmd.Args) != 4 {
			t.Fatalf("expected 4 args for cursor-agent, got %d: %v", len(cmd.Args), cmd.Args)
		}
		if filepath.Base(cmd.Args[0]) != "cursor-agent" || !filepath.IsAbs(cmd.Args[0]) {
			t.Errorf("expected Args[0] to be an absolute path to cursor-agent, got %q", cmd.Args[0])
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
		if filepath.Base(cmd.Args[0]) != "codex" || !filepath.IsAbs(cmd.Args[0]) {
			t.Errorf("expected Args[0] to be an absolute path to codex, got %q", cmd.Args[0])
		}
		if cmd.Args[len(cmd.Args)-1] != "hello world" {
			t.Errorf("expected prompt at last arg, got %q", cmd.Args[len(cmd.Args)-1])
		}
	default:
		t.Skip("no agent runtime on PATH")
	}
}
