package gastown

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
		if err := validateIssueID(id); err != nil {
			t.Errorf("validateIssueID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []string{
		"",
		"mg",
		"-mg-42",
		"MG-42",
		"--delete-all",
		strings.Repeat("a", 65) + "-1",
	}
	for _, id := range invalid {
		if err := validateIssueID(id); err == nil {
			t.Errorf("validateIssueID(%q) = nil, want error", id)
		}
	}
}

func TestSlingRejectsInvalidID(t *testing.T) {
	err := Sling("--force")
	if err == nil {
		t.Fatal("expected validation error for flag-like issue ID")
	}
}

func TestCascadeCloseRejectsInvalidID(t *testing.T) {
	err := CascadeClose("INVALID")
	if err == nil {
		t.Fatal("expected validation error for uppercase issue ID")
	}
}

func TestReleaseIssueRejectsInvalidID(t *testing.T) {
	err := ReleaseIssue("../etc", "reason")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConvoyCreateRejectsInvalidIssueID(t *testing.T) {
	_, err := ConvoyCreate("my-convoy", []string{"mg-42", "--bad"})
	if err == nil {
		t.Fatal("expected validation error for invalid issue ID in list")
	}
}

func TestNudgeSanitizesMessage(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := Nudge("polecat-1", "wake up\x00\x07please")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	args := (*calls)[0]
	// Message should have control chars stripped
	for _, a := range args {
		if strings.ContainsAny(a, "\x00\x07") {
			t.Errorf("control characters not stripped from args: %v", args)
		}
	}
}

func TestMailSendSanitizesInputs(t *testing.T) {
	calls, restore := mockCombinedCapture([]byte("ok"), nil)
	defer restore()
	err := MailSend("polecat-1", "subj\x00ect", "bo\x07dy")
	if err != nil {
		t.Fatalf("MailSend() error = %v", err)
	}
	args := (*calls)[0]
	for _, a := range args {
		if strings.ContainsAny(a, "\x00\x07") {
			t.Errorf("control characters not stripped from args: %v", args)
		}
	}
}
