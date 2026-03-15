package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// --- Initialization tests ---

func TestDoctorNewDefaults(t *testing.T) {
	d := NewDoctor(80, 24)
	if d.HasResult() {
		t.Fatal("new Doctor should not have a result")
	}
}

func TestDoctorSetSize(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetSize(120, 40)
	if d.width != 120 {
		t.Fatalf("width = %d, want 120", d.width)
	}
	if d.height != 40 {
		t.Fatalf("height = %d, want 40", d.height)
	}
}

// --- SetResult tests ---

func TestDoctorSetResultNil(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetResult(nil)
	if d.HasResult() {
		t.Fatal("SetResult(nil) should leave HasResult() false")
	}
}

func TestDoctorSetResultPopulates(t *testing.T) {
	d := NewDoctor(80, 24)
	result := &data.DoctorResult{
		OK:      true,
		Summary: "All checks passed",
	}
	d.SetResult(result)
	if !d.HasResult() {
		t.Fatal("SetResult with non-nil should make HasResult() true")
	}
}

func TestDoctorSetResultResetsScroll(t *testing.T) {
	d := NewDoctor(80, 24)
	result := &data.DoctorResult{
		OK:      false,
		Summary: "2 issues found",
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "check-a", Status: "error"},
			{Name: "check-b", Status: "warning"},
			{Name: "check-c", Status: "ok"},
		},
	}
	d.SetResult(result)

	// Move cursor down
	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if d.cursor == 0 {
		t.Fatal("cursor should have moved away from 0")
	}

	// Set a new result — cursor should reset
	d.SetResult(result)
	if d.cursor != 0 {
		t.Fatalf("cursor should reset to 0 after SetResult, got %d", d.cursor)
	}
}

// --- Navigation tests ---

func TestDoctorJDownMoveCursor(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetResult(&data.DoctorResult{
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "check-a", Status: "error"},
			{Name: "check-b", Status: "warning"},
			{Name: "check-c", Status: "ok"},
		},
	})

	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if d.cursor != 1 {
		t.Fatalf("after j, cursor = %d, want 1", d.cursor)
	}

	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if d.cursor != 2 {
		t.Fatalf("after j j, cursor = %d, want 2", d.cursor)
	}

	// Should not go past end
	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if d.cursor != 2 {
		t.Fatalf("cursor should clamp at end, got %d", d.cursor)
	}
}

func TestDoctorKUpMoveCursor(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetResult(&data.DoctorResult{
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "check-a", Status: "error"},
			{Name: "check-b", Status: "warning"},
		},
	})

	// Move down then back up
	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	d, _ = d.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if d.cursor != 0 {
		t.Fatalf("after j then k, cursor = %d, want 0", d.cursor)
	}

	// Should not go below 0
	d, _ = d.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if d.cursor != 0 {
		t.Fatalf("cursor should stop at 0, got %d", d.cursor)
	}
}

func TestDoctorGJumpsToTop(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetResult(&data.DoctorResult{
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "a", Status: "ok"},
			{Name: "b", Status: "ok"},
			{Name: "c", Status: "ok"},
		},
	})

	d, _ = d.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if d.cursor != 2 {
		t.Fatalf("after G, cursor = %d, want 2", d.cursor)
	}

	d, _ = d.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if d.cursor != 0 {
		t.Fatalf("after g, cursor = %d, want 0", d.cursor)
	}
}

func TestDoctorGGJumpsToBottom(t *testing.T) {
	d := NewDoctor(80, 24)
	d.SetResult(&data.DoctorResult{
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "a", Status: "ok"},
			{Name: "b", Status: "ok"},
			{Name: "c", Status: "ok"},
			{Name: "d", Status: "ok"},
		},
	})

	d, _ = d.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if d.cursor != 3 {
		t.Fatalf("after G, cursor = %d, want 3", d.cursor)
	}
}

func TestDoctorNavigationEmptyNoop(t *testing.T) {
	d := NewDoctor(80, 24)
	// No result set — navigation should be a no-op
	d, cmd := d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil {
		t.Fatal("expected nil cmd on empty navigation")
	}
	if d.cursor != 0 {
		t.Fatalf("cursor should remain 0, got %d", d.cursor)
	}

	// With empty diagnostics
	d.SetResult(&data.DoctorResult{Diagnostics: nil})
	d, _ = d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if d.cursor != 0 {
		t.Fatalf("cursor should remain 0 with empty diagnostics, got %d", d.cursor)
	}
}

// --- View rendering tests ---

func TestDoctorViewNoResult(t *testing.T) {
	d := NewDoctor(80, 24)
	view := d.View()
	if !strings.Contains(view, "DIAGNOSTICS") {
		t.Fatal("view with no result should contain 'DIAGNOSTICS'")
	}
}

func TestDoctorViewAllOK(t *testing.T) {
	d := NewDoctor(100, 30)
	d.SetResult(&data.DoctorResult{
		OK:      true,
		Summary: "All checks passed",
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "dolt-server", Status: "ok", Category: "Core System"},
			{Name: "git-clean", Status: "ok", Category: "Git Integration"},
		},
	})

	view := d.View()
	if !strings.Contains(view, "All checks passed") {
		t.Fatal("all-OK view should contain success message")
	}
}

func TestDoctorViewWithErrors(t *testing.T) {
	d := NewDoctor(100, 30)
	d.SetResult(&data.DoctorResult{
		OK:      false,
		Summary: "1 error found",
		Diagnostics: []data.DoctorDiagnostic{
			{
				Name:        "dolt-server",
				Status:      "error",
				Severity:    "blocking",
				Category:    "Core System",
				Explanation: "Dolt server is not running",
			},
		},
	})

	view := d.View()
	if !strings.Contains(view, "Core System") {
		t.Fatal("view should show category")
	}
	if !strings.Contains(view, "Dolt server is not running") {
		t.Fatal("view should show explanation")
	}
	if !strings.Contains(view, "blocking") {
		t.Fatal("view should show severity")
	}
}

func TestDoctorViewShowsCommands(t *testing.T) {
	d := NewDoctor(100, 30)
	d.SetResult(&data.DoctorResult{
		OK:      false,
		Summary: "1 issue",
		Diagnostics: []data.DoctorDiagnostic{
			{
				Name:     "dolt-server",
				Status:   "error",
				Commands: []string{"dolt sql-server --port 3307", "bd init --force"},
			},
		},
	})

	view := d.View()
	if !strings.Contains(view, "dolt sql-server --port 3307") {
		t.Fatal("view should show first command")
	}
	if !strings.Contains(view, "bd init --force") {
		t.Fatal("view should show second command")
	}
}

func TestDoctorViewShowsSummary(t *testing.T) {
	d := NewDoctor(100, 30)
	d.SetResult(&data.DoctorResult{
		OK:      false,
		Summary: "3 checks failed, 5 passed",
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "a", Status: "error"},
		},
	})

	view := d.View()
	if !strings.Contains(view, "3 checks failed, 5 passed") {
		t.Fatal("view should show the summary string")
	}
}

func TestDoctorViewMixedStatuses(t *testing.T) {
	d := NewDoctor(100, 30)
	d.SetResult(&data.DoctorResult{
		OK:      false,
		Summary: "mixed",
		Diagnostics: []data.DoctorDiagnostic{
			{Name: "dolt-server", Status: "ok", Explanation: "Server running"},
			{Name: "git-clean", Status: "error", Explanation: "Working tree dirty"},
			{Name: "issue-prefix", Status: "warning", Explanation: "Prefix mismatch"},
		},
	})

	view := d.View()
	if !strings.Contains(view, "dolt-server") {
		t.Fatal("view should render ok diagnostic")
	}
	if !strings.Contains(view, "git-clean") {
		t.Fatal("view should render error diagnostic")
	}
	if !strings.Contains(view, "issue-prefix") {
		t.Fatal("view should render warning diagnostic")
	}
	if !strings.Contains(view, "Server running") {
		t.Fatal("view should show ok explanation")
	}
	if !strings.Contains(view, "Working tree dirty") {
		t.Fatal("view should show error explanation")
	}
	if !strings.Contains(view, "Prefix mismatch") {
		t.Fatal("view should show warning explanation")
	}
}
