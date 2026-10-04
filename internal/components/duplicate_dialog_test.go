package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func dupDialog() DuplicateDialog {
	return NewDuplicateDialog("CI pipeline times out", []DuplicateCandidate{
		{ID: "mg-2", Title: "Fix CI pipeline timeout", Status: "in_progress", Prob: 0.91},
		{ID: "mg-17", Title: "Flaky integration stage", Status: "open", Prob: 0.54},
	}, 80)
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

func result(t *testing.T, cmd tea.Cmd) DuplicateDialogResult {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a result command")
	}
	r, ok := cmd().(DuplicateDialogResult)
	if !ok {
		t.Fatalf("got %T", r)
	}
	return r
}

func TestDuplicateDialogView(t *testing.T) {
	out := ansi.Strip(dupDialog().View())
	for _, want := range []string{"CI pipeline times out", "91%", "mg-2", "Fix CI pipeline timeout", "(in_progress)", "54%", "mg-17"} {
		if !strings.Contains(out, want) {
			t.Errorf("view lacks %q:\n%s", want, out)
		}
	}
}

func TestDuplicateDialogUpdate(t *testing.T) {
	d := dupDialog()

	if r := result(t, mustCmd(d.Update(key("enter")))); r.Action != DuplicateJump || r.TargetID != "mg-2" {
		t.Fatalf("enter = %+v, want jump to the best match", r)
	}
	if r := result(t, mustCmd(d.Update(key("c")))); r.Action != DuplicateCreate || r.TargetID != "mg-2" {
		t.Fatalf("c = %+v", r)
	}
	if r := result(t, mustCmd(d.Update(key("esc")))); r.Action != DuplicateCancel {
		t.Fatalf("esc = %+v", r)
	}

	d, cmd := d.Update(key("j"))
	if cmd != nil || d.Selected().ID != "mg-17" {
		t.Fatalf("j should move the cursor, got %+v", d.Selected())
	}
	if r := result(t, mustCmd(d.Update(key("l")))); r.Action != DuplicateLink || r.TargetID != "mg-17" {
		t.Fatalf("l = %+v, want link to the selected candidate", r)
	}
	d, _ = d.Update(key("j"))
	if d.Selected().ID != "mg-17" {
		t.Fatal("cursor ran past the last candidate")
	}
	d, _ = d.Update(key("k"))
	d, _ = d.Update(key("k"))
	if d.Selected().ID != "mg-2" {
		t.Fatal("cursor ran past the first candidate")
	}
}

func TestDuplicateDialogEdgeCaseNoCandidates(t *testing.T) {
	d := NewDuplicateDialog("x", nil, 40)
	if r := result(t, mustCmd(d.Update(key("enter")))); r.TargetID != "" {
		t.Fatalf("empty dialog should have no target: %+v", r)
	}
}

func mustCmd(_ DuplicateDialog, cmd tea.Cmd) tea.Cmd { return cmd }
