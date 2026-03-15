package gastown

import (
	"errors"
	"testing"
)

func TestMoleculeDAGHappy(t *testing.T) {
	dagJSON := `{
		"root_id": "mg-100",
		"root_title": "Platform migration",
		"total_nodes": 3,
		"tiers": 2,
		"critical_path": ["mg-100.1", "mg-100.2"],
		"nodes": {
			"mg-100.1": {"id": "mg-100.1", "title": "Auth service", "status": "done", "tier": 0},
			"mg-100.2": {"id": "mg-100.2", "title": "Billing service", "status": "in_progress", "tier": 1, "dependencies": ["mg-100.1"]},
			"mg-100.3": {"id": "mg-100.3", "title": "Migrate tests", "status": "ready", "tier": 1}
		},
		"tier_groups": [["mg-100.1"], ["mg-100.2", "mg-100.3"]]
	}`
	defer mockRun([]byte(dagJSON), nil)()
	dag, err := MoleculeDAG("mg-100")
	if err != nil {
		t.Fatalf("MoleculeDAG() error = %v", err)
	}
	if dag.RootID != "mg-100" {
		t.Errorf("RootID = %q", dag.RootID)
	}
	if dag.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", dag.TotalNodes)
	}
	if len(dag.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(dag.Nodes))
	}
	if dag.ActiveStepID() != "mg-100.2" {
		t.Errorf("ActiveStepID = %q, want mg-100.2", dag.ActiveStepID())
	}
}

func TestMoleculeDAGExecError(t *testing.T) {
	defer mockRun(nil, errors.New("gt not found"))()
	_, err := MoleculeDAG("mg-100")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMoleculeProgressFetchHappy(t *testing.T) {
	progJSON := `{
		"root_id": "mg-100",
		"root_title": "Platform migration",
		"total_steps": 5,
		"done_steps": 2,
		"in_progress_steps": 1,
		"ready_steps": ["mg-100.3"],
		"blocked_steps": ["mg-100.4"],
		"percent_complete": 40,
		"complete": false
	}`
	defer mockRun([]byte(progJSON), nil)()
	prog, err := MoleculeProgressFetch("mg-100")
	if err != nil {
		t.Fatalf("MoleculeProgressFetch() error = %v", err)
	}
	if prog.TotalSteps != 5 {
		t.Errorf("TotalSteps = %d, want 5", prog.TotalSteps)
	}
	if prog.DoneSteps != 2 {
		t.Errorf("DoneSteps = %d, want 2", prog.DoneSteps)
	}
	if prog.Percent != 40 {
		t.Errorf("Percent = %d, want 40", prog.Percent)
	}
	if prog.Complete {
		t.Error("expected Complete=false")
	}
}

func TestMoleculeProgressFetchExecError(t *testing.T) {
	defer mockRun(nil, errors.New("gt not found"))()
	_, err := MoleculeProgressFetch("mg-100")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMoleculeStepDoneHappy(t *testing.T) {
	resultJSON := `{
		"step_id": "mg-100.1",
		"molecule_id": "mg-100",
		"step_closed": true,
		"next_step_id": "mg-100.2",
		"next_step_title": "Billing service",
		"complete": false,
		"action": "continue"
	}`
	defer mockRun([]byte(resultJSON), nil)()
	result, err := MoleculeStepDone("mg-100.1")
	if err != nil {
		t.Fatalf("MoleculeStepDone() error = %v", err)
	}
	if result.StepID != "mg-100.1" {
		t.Errorf("StepID = %q", result.StepID)
	}
	if !result.StepClosed {
		t.Error("expected StepClosed=true")
	}
	if result.Action != "continue" {
		t.Errorf("Action = %q, want continue", result.Action)
	}
	if result.NextStepID != "mg-100.2" {
		t.Errorf("NextStepID = %q, want mg-100.2", result.NextStepID)
	}
}

func TestMoleculeStepDoneExecError(t *testing.T) {
	defer mockRun(nil, errors.New("gt not found"))()
	_, err := MoleculeStepDone("mg-100.1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
