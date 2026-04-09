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
		"MG-42",
		"mg-",
		"mg 42",
		"--delete-all",
		"../../../etc/passwd",
		strings.Repeat("a", 65) + "-1",
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
		t.Fatal("expected validation error for uppercase issue ID")
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
