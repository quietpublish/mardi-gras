package agent

import (
	"os"
	"testing"
)

func TestInTmux(t *testing.T) {
	// Save and restore original value.
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	if !InTmux() {
		t.Error("expected InTmux()=true when TMUX is set")
	}

	os.Unsetenv("TMUX")
	if InTmux() {
		t.Error("expected InTmux()=false when TMUX is unset")
	}
}

func TestWindowName(t *testing.T) {
	tests := []struct {
		issueID string
		want    string
	}{
		{"bd-a1b2", "mg-bd-a1b2"},
		{"mg-001", "mg-mg-001"},
		{"xyz", "mg-xyz"},
	}
	for _, tt := range tests {
		got := WindowName(tt.issueID)
		if got != tt.want {
			t.Errorf("WindowName(%q) = %q, want %q", tt.issueID, got, tt.want)
		}
	}
}

func TestTmuxSocketPath(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	tests := []struct {
		name string
		tmux string
		want string
	}{
		{"standard", "/tmp/tmux-1000/default,12345,0", "/tmp/tmux-1000/default"},
		{"nested inner", "/tmp/tmux-1000/inner,67890,2", "/tmp/tmux-1000/inner"},
		{"empty", "", ""},
		{"no commas", "/tmp/tmux-1000/default", "/tmp/tmux-1000/default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tmux == "" {
				os.Unsetenv("TMUX")
			} else {
				os.Setenv("TMUX", tt.tmux)
			}
			got := tmuxSocketPath()
			if got != tt.want {
				t.Errorf("tmuxSocketPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAgentWindows(t *testing.T) {
	output := "mg-bd-a1b2\t@5\n\t@0\nmg-bd-c3d4\t@8\n\t@1\n"
	agents := parseAgentWindows(output)

	if len(agents) != 2 {
		t.Fatalf("expected 2 agent windows, got %d: %v", len(agents), agents)
	}

	if agents["bd-a1b2"] != "@5" {
		t.Errorf("missing or wrong entry for bd-a1b2: %v", agents)
	}
	if agents["bd-c3d4"] != "@8" {
		t.Errorf("missing or wrong entry for bd-c3d4: %v", agents)
	}
}

func TestParseAgentWindowsEmpty(t *testing.T) {
	agents := parseAgentWindows("")
	if len(agents) != 0 {
		t.Errorf("expected 0 agent windows from empty input, got %d", len(agents))
	}
}

func TestParseAgentWindowsNoAgents(t *testing.T) {
	output := "\t@0\n\t@1\n\t@2\n"
	agents := parseAgentWindows(output)
	if len(agents) != 0 {
		t.Errorf("expected 0 agent windows, got %d: %v", len(agents), agents)
	}
}

func TestPermissionRe(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"  Allow Read tool?  /path/to/file", true},
		{"  Allow Bash tool?  ls -la", true},
		{"  Allow mcp__some_server tool?", true},
		{"    Do you want to allow this action?", true},
		{"Allow Edit tool?", true},
		{"Working on the task...", false},
		{"Allowing access is important", false},
		{"agent is running claude code", false},
		{"", false},
	}
	for _, tt := range tests {
		got := permissionRe.MatchString(tt.line)
		if got != tt.want {
			t.Errorf("permissionRe.MatchString(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
