package gastown

import (
	"errors"
	"testing"
)

func TestFindOrphansNil(t *testing.T) {
	orphans := FindOrphans(nil, "mardi_gras")
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans from nil status, got %d", len(orphans))
	}
}

func TestFindOrphansDeadPolecat(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-001", WorkTitle: "Fix auth"},
			{Name: "matt", Role: "crew", Rig: "mardi_gras",
				Running: true, HasWork: true, State: "working"},
		},
	}
	orphans := FindOrphans(status, "mardi_gras")
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].IssueID != "mg-001" {
		t.Errorf("expected issue mg-001, got %q", orphans[0].IssueID)
	}
	if orphans[0].AgentName != "obsidian" {
		t.Errorf("expected agent obsidian, got %q", orphans[0].AgentName)
	}
}

func TestFindOrphansIgnoresOtherRigs(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "other_rig",
				Running: false, HookBead: "or-001"},
		},
	}
	orphans := FindOrphans(status, "mardi_gras")
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans for wrong rig, got %d", len(orphans))
	}
}

func TestFindOrphansIgnoresRunningAgents(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: true, HookBead: "mg-001", HasWork: true, State: "working"},
		},
	}
	orphans := FindOrphans(status, "mardi_gras")
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans for running agents, got %d", len(orphans))
	}
}

func TestFindOrphansIgnoresUnhookedDead(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "quartz", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: ""},
		},
	}
	orphans := FindOrphans(status, "mardi_gras")
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans for unhooked dead agent, got %d", len(orphans))
	}
}

func TestFindOrphansMultiple(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-001", WorkTitle: "Fix auth"},
			{Name: "quartz", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-002", WorkTitle: "Add tests"},
			{Name: "matt", Role: "crew", Rig: "mardi_gras",
				Running: false, HookBead: "mg-003", WorkTitle: "Review PR"},
		},
	}
	orphans := FindOrphans(status, "mardi_gras")
	if len(orphans) != 3 {
		t.Fatalf("expected 3 orphans, got %d", len(orphans))
	}
}

func TestFindDeadRigsNil(t *testing.T) {
	rigs := FindDeadRigs(nil)
	if len(rigs) != 0 {
		t.Errorf("expected 0 dead rigs from nil, got %d", len(rigs))
	}
}

func TestFindDeadRigsHealthy(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "mardi_gras", PolecatCount: 2},
		},
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: true, HasWork: true, State: "working"},
		},
	}
	rigs := FindDeadRigs(status)
	if len(rigs) != 0 {
		t.Errorf("expected 0 dead rigs for healthy rig, got %d", len(rigs))
	}
}

func TestFindDeadRigsZeroPolecatsWithOrphans(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "mardi_gras", PolecatCount: 0},
		},
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-001"},
		},
	}
	rigs := FindDeadRigs(status)
	if len(rigs) != 1 {
		t.Fatalf("expected 1 dead rig, got %d", len(rigs))
	}
	if rigs[0] != "mardi_gras" {
		t.Errorf("expected mardi_gras, got %q", rigs[0])
	}
}

func TestFindDeadRigsZeroPolecatsNoOrphans(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "mardi_gras", PolecatCount: 0},
		},
		Agents: []AgentRuntime{
			{Name: "matt", Role: "crew", Rig: "mardi_gras",
				Running: true, State: "idle"},
		},
	}
	rigs := FindDeadRigs(status)
	if len(rigs) != 0 {
		t.Errorf("expected 0 dead rigs when no orphans, got %d", len(rigs))
	}
}

func TestReleaseIssueArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := ReleaseIssue("mg-42", "rig crashed")
	if err != nil {
		t.Fatalf("ReleaseIssue() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt release mg-42 --reason rig crashed
	if len(args) != 5 || args[0] != "gt" || args[1] != "release" || args[2] != "mg-42" || args[3] != "--reason" || args[4] != "rig crashed" {
		t.Errorf("args = %v", args)
	}
}

func TestReleaseIssueNoReason(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := ReleaseIssue("mg-42", "")
	if err != nil {
		t.Fatalf("ReleaseIssue() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt release mg-42 (no --reason)
	if len(args) != 3 || args[1] != "release" || args[2] != "mg-42" {
		t.Errorf("args = %v, want [gt release mg-42]", args)
	}
}

func TestReleaseIssueError(t *testing.T) {
	_, restore := mockExecCapture(errors.New("not found"))
	defer restore()
	err := ReleaseIssue("mg-42", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
