package data

import (
	"strings"
	"testing"
)

func TestValidateIssueID(t *testing.T) {
	valid := []string{
		"mg-42",
		"bd-a1b2",
		"my-app-xyz123",
		"a-1",
		// Child issues: beads ids its children by appending a dotted segment,
		// so anything that cannot be said here cannot be edited in mg at all.
		"infra-h0xb.9",
		"infra-h0xb.16",
		"infra-qfam",
		"a-b.c",
		// Grandchildren nest the same way.
		"infra-h0xb.9.1",
		// bd's prefix may carry underscores and upper case: the default prefix
		// is the directory name (`my_project`), and `--prefix MyApp` is legal.
		"my_project-hbu",
		"my_project-hbu.1",
		"MyApp-d0g",
		"MG-42",
		"web_app-api-x1",
	}
	for _, id := range valid {
		if err := ValidateIssueID(id); err != nil {
			t.Errorf("ValidateIssueID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []string{
		"",
		"mg",
		"-mg-42",
		"mg-",
		"mg 42",
		"--delete-all",
		"../../../etc/passwd",
		strings.Repeat("a", 65) + "-1",
		// Accepting the dot must not have opened a way in for these.
		"-x",
		"../etc",
		"a/../b",
		".hidden",
		"a.",
		"a..b",
		"a.-b",
		// No separator, so no hash: a bare prefix is not an id.
		"a_b",
		// Widening the character set must not admit these either.
		"_a-1",
		"9a-1",
		"Ä-1",
		"a-1;rm",
		"a-1 b",
	}
	for _, id := range invalid {
		if err := ValidateIssueID(id); err == nil {
			t.Errorf("ValidateIssueID(%q) = nil, want error", id)
		}
	}
}

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"plain text", "hello world", 100, "hello world"},
		{"preserves newlines", "line1\nline2", 100, "line1\nline2"},
		{"preserves tabs", "col1\tcol2", 100, "col1\tcol2"},
		{"strips null bytes", "hello\x00world", 100, "helloworld"},
		{"strips bell", "hello\x07world", 100, "helloworld"},
		{"strips escape", "hello\x1bworld", 100, "helloworld"},
		{"strips mixed control", "\x01\x02hello\x03\x04", 100, "hello"},
		{"truncates to maxLen", "abcdefghij", 5, "abcde"},
		{"empty string", "", 100, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeText(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("sanitizeText(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestAddCommentRejectsInvalidID(t *testing.T) {
	// No mock needed — validation should reject before exec
	err := AddComment("--delete-all", "body")
	if err == nil {
		t.Fatal("expected validation error for flag-like issue ID")
	}
}

func TestAddLabelRejectsInvalidID(t *testing.T) {
	err := AddLabel("INVALID", "backend")
	if err == nil {
		t.Fatal("expected validation error for issue ID with no separator")
	}
}

func TestAddDependencyRejectsInvalidDependsOn(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := AddDependency("mg-42", "--force")
	if err == nil {
		t.Fatal("expected validation error for invalid depends-on ID")
	}
	if len(*calls) != 0 {
		t.Error("exec should not have been called after validation failure")
	}
}

func TestSetStatusRejectsInvalidID(t *testing.T) {
	err := SetStatus("../etc", StatusOpen)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// A child issue's dotted id has to reach bd verbatim. This is the path the TUI
// edit form takes (editIssueCmd -> SetStatus/UpdateTitle/...), so a rejected
// dot here is not a cosmetic problem: it is mg refusing to edit any child issue.
func TestMutatorsPassDottedChildIDToBd(t *testing.T) {
	for _, id := range []string{"infra-h0xb.9", "infra-h0xb.16", "infra-h0xb.9.1", "my_project-hbu.1", "MyApp-d0g"} {
		calls, restore := mockExecCapture(nil)
		err := SetStatus(id, StatusOpen)
		got := *calls
		restore()
		if err != nil {
			t.Fatalf("SetStatus(%q) = %v, want nil", id, err)
		}
		if len(got) != 1 || len(got[0]) != 4 || got[0][1] != "update" || got[0][2] != id {
			t.Errorf("SetStatus(%q) ran %v, want bd update %s --status=open", id, got, id)
		}
	}
}
