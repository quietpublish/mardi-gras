package gastown

import "testing"

func TestSlingMultipleEmpty(t *testing.T) {
	err := SlingMultiple(nil)
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleWithFormulaEmpty(t *testing.T) {
	err := SlingMultipleWithFormula(nil, "shiny")
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleEmptySlice(t *testing.T) {
	err := SlingMultiple([]string{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleWithFormulaEmptySlice(t *testing.T) {
	err := SlingMultipleWithFormula([]string{}, "shiny")
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := Sling("mg-42")
	if err != nil {
		t.Fatalf("Sling() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	// Should be: gt sling mg-42
	if len(args) != 3 || args[0] != "gt" || args[1] != "sling" || args[2] != "mg-42" {
		t.Errorf("args = %v, want [gt sling mg-42]", args)
	}
}

func TestSlingWithAgentArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SlingWithAgent("mg-42", "codex")
	if err != nil {
		t.Fatalf("SlingWithAgent() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	want := []string{"gt", "sling", "--agent", "codex", "mg-42"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, args[i], w)
		}
	}
}

func TestSlingWithAgentRejectsBadID(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	if err := SlingWithAgent("../etc/passwd", "codex"); err == nil {
		t.Fatal("expected error for invalid issue ID")
	}
	if len(*calls) != 0 {
		t.Errorf("invalid ID should not invoke gt, got %d calls", len(*calls))
	}
}

func TestSlingMultipleWithAgentArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SlingMultipleWithAgent([]string{"mg-1", "mg-2"}, "codex")
	if err != nil {
		t.Fatalf("SlingMultipleWithAgent() error = %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(*calls))
	}
	wantIDs := []string{"mg-1", "mg-2"}
	for i, args := range *calls {
		want := []string{"gt", "sling", "--agent", "codex", wantIDs[i]}
		if len(args) != len(want) {
			t.Fatalf("call[%d] args = %v, want %v", i, args, want)
		}
		for j, w := range want {
			if args[j] != w {
				t.Errorf("call[%d] args[%d] = %q, want %q", i, j, args[j], w)
			}
		}
	}
}

func TestSlingMultipleWithAgentEmpty(t *testing.T) {
	err := SlingMultipleWithAgent(nil, "codex")
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingWithFormulaArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SlingWithFormula("mg-42", "shiny")
	if err != nil {
		t.Fatalf("SlingWithFormula() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt sling shiny --on mg-42
	if len(args) != 5 || args[1] != "sling" || args[2] != "shiny" || args[3] != "--on" || args[4] != "mg-42" {
		t.Errorf("args = %v, want [gt sling shiny --on mg-42]", args)
	}
}

func TestListFormulasHappy(t *testing.T) {
	defer mockRun([]byte(gtFormulaListOutput), nil)()
	names, err := ListFormulas()
	if err != nil {
		t.Fatalf("ListFormulas() error = %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 formulas, got %d", len(names))
	}
	if names[0] != "default" || names[1] != "shiny" || names[2] != "quick-fix" {
		t.Errorf("names = %v", names)
	}
}

func TestListFormulasEmpty(t *testing.T) {
	defer mockRun([]byte(""), nil)()
	names, err := ListFormulas()
	if err != nil {
		t.Fatalf("ListFormulas() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 formulas, got %d", len(names))
	}
}

func TestNudgeWithMessage(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := Nudge("polecat-nux", "wake up")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt nudge polecat-nux -- wake up
	if len(args) != 5 || args[1] != "nudge" || args[2] != "polecat-nux" || args[3] != "--" || args[4] != "wake up" {
		t.Errorf("args = %v", args)
	}
}

func TestNudgeWithoutMessage(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := Nudge("polecat-nux", "")
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt nudge polecat-nux (no message arg)
	if len(args) != 3 || args[1] != "nudge" || args[2] != "polecat-nux" {
		t.Errorf("args = %v, want [gt nudge polecat-nux]", args)
	}
}

func TestDecommissionArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := Decommission("polecat-nux@mardi_gras")
	if err != nil {
		t.Fatalf("Decommission() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt polecat kill polecat-nux@mardi_gras
	if len(args) != 4 || args[1] != "polecat" || args[2] != "kill" || args[3] != "polecat-nux@mardi_gras" {
		t.Errorf("args = %v", args)
	}
}

func TestCascadeCloseArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := CascadeClose("mg-100")
	if err != nil {
		t.Fatalf("CascadeClose() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt close --cascade mg-100
	if len(args) != 4 || args[1] != "close" || args[2] != "--cascade" || args[3] != "mg-100" {
		t.Errorf("args = %v", args)
	}
}
