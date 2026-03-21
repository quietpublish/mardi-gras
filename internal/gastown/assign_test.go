package gastown

import "testing"

func TestAssignArgs(t *testing.T) {
	calls, restore := mockCombinedCapture([]byte("✅ Created mg-99 and hooked to mardi_gras/crew/monet\n"), nil)
	defer restore()

	_, err := Assign("monet", "Fix the bug", "", "", "", false)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	// gt assign monet -- Fix the bug
	expected := []string{"gt", "assign", "--", "monet", "Fix the bug"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range expected {
		if args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, args[i], a)
		}
	}
}

func TestAssignWithNudge(t *testing.T) {
	calls, restore := mockCombinedCapture([]byte("✅ Created mg-99\n"), nil)
	defer restore()

	_, err := Assign("monet", "Fix the bug", "", "", "", true)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	args := (*calls)[0]
	// gt assign --nudge -- monet Fix the bug
	expected := []string{"gt", "assign", "--nudge", "--", "monet", "Fix the bug"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range expected {
		if args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, args[i], a)
		}
	}
}

func TestAssignWithAllFlags(t *testing.T) {
	calls, restore := mockCombinedCapture([]byte("✅ Created mg-99\n"), nil)
	defer restore()

	_, err := Assign("monet", "Fix the bug", "bug", "1", "backend", true)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	args := (*calls)[0]
	// gt assign -t bug -p 1 -l backend --nudge -- monet Fix the bug
	expected := []string{"gt", "assign", "-t", "bug", "-p", "1", "-l", "backend", "--nudge", "--", "monet", "Fix the bug"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range expected {
		if args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, args[i], a)
		}
	}
}

func TestAssignReturnsOutput(t *testing.T) {
	output := "✅ Created mg-99 and hooked to mardi_gras/crew/monet\n"
	defer mockCombined([]byte(output), nil)()

	got, err := Assign("monet", "Fix the bug", "", "", "", false)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	if got != output {
		t.Errorf("got %q, want %q", got, output)
	}
}

func TestAssignError(t *testing.T) {
	defer mockCombined([]byte("Error: crew member 'nobody' not found\n"), errTest)()

	_, err := Assign("nobody", "Fix the bug", "", "", "", false)
	if err == nil {
		t.Fatal("expected error from Assign()")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "exit status 1" }
