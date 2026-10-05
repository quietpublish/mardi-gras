package components

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestApprovalDialogDefaultApprove(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run a command?", []string{"ls", "-la"}, "/work", "", nil, 80, 24)
	_, cmd := ad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from enter")
	}
	res, ok := cmd().(ApprovalDialogResult)
	if !ok {
		t.Fatalf("expected ApprovalDialogResult, got %T", cmd())
	}
	if res.Cancelled {
		t.Fatal("unexpected Cancelled")
	}
	if res.Decision != "approved" {
		t.Fatalf("Decision = %q, want approved (default selection)", res.Decision)
	}
}

func TestApprovalDialogNavigateToDeny(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"rm", "-rf", "/"}, "/work", "danger", nil, 80, 24)
	// Decisions: approved, approved_for_session, denied, abort. Two downs → denied.
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_, cmd := ad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	res := cmd().(ApprovalDialogResult)
	if res.Decision != "denied" {
		t.Fatalf("Decision = %q, want denied", res.Decision)
	}
}

func TestApprovalDialogClampsSelection(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"ls"}, "/work", "", nil, 80, 24)
	// Up at top stays at top.
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	// Many downs clamp at the last entry (abort).
	for range 10 {
		ad, _ = ad.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	_, cmd := ad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	res := cmd().(ApprovalDialogResult)
	if res.Decision != "abort" {
		t.Fatalf("Decision = %q, want abort (clamped to last)", res.Decision)
	}
}

func TestApprovalDialogEscCancels(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"ls"}, "/work", "", nil, 80, 24)
	_, cmd := ad.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	res := cmd().(ApprovalDialogResult)
	if !res.Cancelled {
		t.Fatal("expected Cancelled=true from esc")
	}
}

func TestApprovalDialogViewExec(t *testing.T) {
	ad := NewApprovalDialog("exec", "Allow?", []string{"git", "push"}, "/repo", "deploy", nil, 80, 24)
	v := ad.View()
	if !strings.Contains(v, "RUN A COMMAND") {
		t.Fatalf("exec view missing heading:\n%s", v)
	}
	if !strings.Contains(v, "git push") {
		t.Fatalf("exec view missing command:\n%s", v)
	}
	if !strings.Contains(v, "/repo") {
		t.Fatalf("exec view missing cwd:\n%s", v)
	}
	if !strings.Contains(v, "deploy") {
		t.Fatalf("exec view missing reason:\n%s", v)
	}
}

func TestApprovalDialogViewPatch(t *testing.T) {
	ad := NewApprovalDialog("patch", "Allow?", nil, "", "implement", []string{"a.go", "b.go"}, 80, 24)
	v := ad.View()
	if !strings.Contains(v, "APPLY A PATCH") {
		t.Fatalf("patch view missing heading:\n%s", v)
	}
	if !strings.Contains(v, "a.go") || !strings.Contains(v, "b.go") {
		t.Fatalf("patch view missing file list:\n%s", v)
	}
	if !strings.Contains(v, "2 file(s)") {
		t.Fatalf("patch view missing file count:\n%s", v)
	}
}

func TestApprovalDialogVerdictMovesUntouchedCursor(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"rm", "-rf", "build"}, "/work", "", nil, 80, 24)
	ad.SetPending(true)
	if out := ansi.Strip(ad.View()); !strings.Contains(out, "evaluating") {
		t.Fatalf("pending should show:\n%s", out)
	}
	ad.SetVerdict(&ApprovalVerdict{Summary: "high risk", Detail: "destructive 92%", Intent: "edit-in-workspace", Level: 2})
	out := ansi.Strip(ad.View())
	for _, want := range []string{"high risk", "destructive 92%", "intent edit-in-workspace"} {
		if !strings.Contains(out, want) {
			t.Errorf("view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "evaluating") {
		t.Fatal("pending marker should clear once the verdict lands")
	}
	if ad.Selected() != "denied" {
		t.Fatalf("a high-risk verdict should park the cursor on Deny, got %q", ad.Selected())
	}
	// The user can still approve: the verdict only moved the default.
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if ad.Selected() != "approved" {
		t.Fatalf("human override failed: %q", ad.Selected())
	}
}

func TestApprovalDialogVerdictEdgeCaseTouchedCursorStays(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"go", "test"}, "/work", "", nil, 80, 24)
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'j', Text: "j"}) // user moved to approve_for_session
	ad.SetVerdict(&ApprovalVerdict{Summary: "high risk", Level: 2})
	if ad.Selected() != "approved_for_session" {
		t.Fatalf("a verdict must not move a cursor the user already placed, got %q", ad.Selected())
	}
	ad2 := NewApprovalDialog("exec", "Run?", []string{"go", "test"}, "/work", "", nil, 80, 24)
	ad2.SetVerdict(&ApprovalVerdict{Summary: "low risk", Level: 0})
	if ad2.Selected() != "approved" {
		t.Fatalf("a routine verdict leaves the default on Approve once, got %q", ad2.Selected())
	}
}

func TestApprovalDialogDenyHit(t *testing.T) {
	ad := NewApprovalDialog("exec", "Run?", []string{"git", "push", "--force"}, "/work", "", nil, 80, 24)
	ad.SetDenyHit("force-push", "git push --force")
	out := ansi.Strip(ad.View())
	if !strings.Contains(out, "DENY-LIST: force-push") || strings.Contains(out, "Approve for this session") {
		t.Fatalf("deny hit should banner and withhold session approval:\n%s", out)
	}
	if !ad.Denied() || ad.Selected() != "denied" {
		t.Fatalf("denied %v selected %q", ad.Denied(), ad.Selected())
	}
	// Approve once is still reachable: two ups from Deny in the shortened list.
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if ad.Selected() != "approved" {
		t.Fatalf("Approve once should sit directly above Deny without session approval, got %q", ad.Selected())
	}
	// A later verdict never re-moves the cursor on a deny-listed request.
	ad.SetVerdict(&ApprovalVerdict{Summary: "low risk", Level: 0})
	if ad.Selected() != "approved" {
		t.Fatal("verdict moved the cursor on a deny-listed request")
	}
	// enter on the shortened list resolves to the right value.
	ad, _ = ad.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_, cmd := ad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if res := cmd().(ApprovalDialogResult); res.Decision != "denied" {
		t.Fatalf("decision = %q", res.Decision)
	}
}

func TestApprovalDialogWrapsToContentWidth(t *testing.T) {
	// A long reason and Jev reading overran the box, which re-wrapped them
	// flush left (mg-xge.2 live run).
	ad := NewApprovalDialog("exec", "", []string{"/bin/zsh", "-lc", "curl -sI https://example.com"}, "/var/folders/xx/T/mgcodex", strings.Repeat("May I run the requested curl command with network access? ", 3), nil, 60, 30)
	for _, line := range strings.Split(ansi.Strip(ad.View()), "\n") {
		if w := ansi.StringWidth(line); w > 60 {
			t.Fatalf("line is %d wide, content width is 60: %q", w, line)
		}
	}
	for _, line := range strings.Split(ansi.Strip(ad.View()), "\n") {
		if line != "" && !strings.HasPrefix(line, "  ") {
			t.Fatalf("wrapped lines should keep the two-space indent: %q", line)
		}
	}
}

func TestApprovalDialogEdgeCaseLongPathFitsNarrowBox(t *testing.T) {
	// A path segment longer than the line was left whole, and the box
	// re-wrapped it into stray blank lines.
	ad := NewApprovalDialog("exec", "", []string{"ls"}, "/var/folders/3p/zxbppzc94nd55h_sv3mcm_gh0000gn/T/mgcodex.7uFTpvOskX", "", nil, 24, 30)
	for _, line := range strings.Split(ansi.Strip(ad.View()), "\n") {
		if w := ansi.StringWidth(line); w > 24 {
			t.Fatalf("line is %d wide at width 24: %q", w, line)
		}
	}
}
