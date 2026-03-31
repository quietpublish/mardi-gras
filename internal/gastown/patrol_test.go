package gastown

import "testing"

func TestPatrolScanProblemsNil(t *testing.T) {
	problems := PatrolScanProblems(nil)
	if len(problems) != 0 {
		t.Fatalf("expected 0 problems for nil scan, got %d", len(problems))
	}
}

func TestPatrolScanProblemsNoFindings(t *testing.T) {
	scan := &PatrolScanResult{
		Rig:         "test_rig",
		Zombies:     PatrolFinding{Checked: 3, Found: 0},
		Stalls:      PatrolFinding{Checked: 3, Found: 0},
		Completions: PatrolFinding{Checked: 3, Found: 0},
	}
	problems := PatrolScanProblems(scan)
	if len(problems) != 0 {
		t.Fatalf("expected 0 problems for clean scan, got %d", len(problems))
	}
}

func TestPatrolScanProblemsZombiesDetected(t *testing.T) {
	scan := &PatrolScanResult{
		Rig:     "test_rig",
		Zombies: PatrolFinding{Checked: 5, Found: 2},
		Stalls:  PatrolFinding{Checked: 5, Found: 0},
	}
	problems := PatrolScanProblems(scan)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "patrol_zombie" {
		t.Fatalf("expected patrol_zombie, got %q", problems[0].Type)
	}
	if problems[0].Severity != "error" {
		t.Fatalf("expected error severity, got %q", problems[0].Severity)
	}
	if problems[0].RigName != "test_rig" {
		t.Fatalf("expected rig test_rig, got %q", problems[0].RigName)
	}
}

func TestPatrolScanProblemsStallsDetected(t *testing.T) {
	scan := &PatrolScanResult{
		Rig:     "test_rig",
		Zombies: PatrolFinding{Checked: 3, Found: 0},
		Stalls:  PatrolFinding{Checked: 3, Found: 1},
	}
	problems := PatrolScanProblems(scan)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "patrol_stall" {
		t.Fatalf("expected patrol_stall, got %q", problems[0].Type)
	}
	if problems[0].Severity != "warn" {
		t.Fatalf("expected warn severity, got %q", problems[0].Severity)
	}
}

func TestPatrolScanProblemsBothZombiesAndStalls(t *testing.T) {
	scan := &PatrolScanResult{
		Rig:     "test_rig",
		Zombies: PatrolFinding{Checked: 5, Found: 1},
		Stalls:  PatrolFinding{Checked: 5, Found: 2},
	}
	problems := PatrolScanProblems(scan)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems, got %d", len(problems))
	}
}

func TestPatrolScanProblemsWithDetails(t *testing.T) {
	scan := &PatrolScanResult{
		Rig:     "test_rig",
		Zombies: PatrolFinding{Checked: 3, Found: 1},
		Stalls:  PatrolFinding{Checked: 3, Found: 0},
		Details: []PatrolDetail{
			{Type: "zombie", Agent: "Toast", Rig: "test_rig", Role: "polecat", HookBead: "mg-001", Detail: "Not running, has hooked work"},
			{Type: "completion", Agent: "Muffin", Rig: "test_rig", Detail: "Completed work on mg-002"},
		},
	}
	problems := PatrolScanProblems(scan)
	// 1 summary zombie + 1 detail zombie (completions are skipped)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (summary + detail zombie, completion skipped), got %d", len(problems))
	}
	if problems[1].Detail != "Not running, has hooked work" {
		t.Fatalf("expected detail from patrol detail, got %q", problems[1].Detail)
	}
	if problems[1].Agent.Name != "Toast" {
		t.Fatalf("expected agent name Toast, got %q", problems[1].Agent.Name)
	}
	if problems[1].Agent.Role != "polecat" {
		t.Fatalf("expected agent role polecat, got %q", problems[1].Agent.Role)
	}
	if problems[1].Agent.HookBead != "mg-001" {
		t.Fatalf("expected agent HookBead mg-001, got %q", problems[1].Agent.HookBead)
	}
}
