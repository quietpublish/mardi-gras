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
		// Parented beads issues carry a dotted id (`infra-h0xb.9`) and must be
		// accepted, or every child issue is unreachable from the gastown side.
		"infra-h0xb.9",
		"infra-h0xb.16",
		"infra-qfam",
		"a-b.c",
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
		if err := validateIssueID(id); err != nil {
			t.Errorf("validateIssueID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []string{
		"",
		"mg",
		"-mg-42",
		"--delete-all",
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
