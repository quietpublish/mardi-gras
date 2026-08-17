package gastown

import "testing"

func TestDetectProblemsNil(t *testing.T) {
	problems := DetectProblems(nil, BackendGasTown)
	if len(problems) != 0 {
		t.Errorf("expected 0 problems from nil status, got %d", len(problems))
	}
}

func TestDetectProblemsStalled(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", Role: "polecat", HasWork: true, State: "idle"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "stalled" {
		t.Errorf("expected type 'stalled', got %q", problems[0].Type)
	}
	if problems[0].Severity != "warn" {
		t.Errorf("expected severity 'warn', got %q", problems[0].Severity)
	}
	if problems[0].Agent.Name != "Toast" {
		t.Errorf("expected agent 'Toast', got %q", problems[0].Agent.Name)
	}
}

func TestDetectProblemsBackoff(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Muffin", Role: "polecat", State: "backoff"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "backoff" {
		t.Errorf("expected type 'backoff', got %q", problems[0].Type)
	}
}

func TestDetectProblemsZombie(t *testing.T) {
	// Zombie on a rig WITH polecats — should still emit zombie.
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "myrig", PolecatCount: 1},
		},
		Agents: []AgentRuntime{
			{Name: "Stale", Role: "polecat", Rig: "myrig", Running: false, HookBead: "bd-e5f6"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "zombie" {
		t.Errorf("expected type 'zombie', got %q", problems[0].Type)
	}
	if problems[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", problems[0].Severity)
	}
}

func TestDetectProblemsStuck(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Granite", Role: "polecat", State: "stuck"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Type != "stuck" {
		t.Errorf("expected type 'stuck', got %q", problems[0].Type)
	}
	if problems[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", problems[0].Severity)
	}
	if problems[0].Agent.Name != "Granite" {
		t.Errorf("expected agent 'Granite', got %q", problems[0].Agent.Name)
	}
}

func TestDetectProblemsHealthy(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", Role: "polecat", Running: true, HasWork: true, State: "working"},
			{Name: "Muffin", Role: "polecat", Running: true, State: "idle"},
			{Name: "Witness", Role: "witness", Running: true, State: "working"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 0 {
		t.Errorf("expected 0 problems for healthy agents, got %d", len(problems))
	}
}

func TestDetectProblemsDeadRig(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "mardi_gras", PolecatCount: 0},
		},
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-001", WorkTitle: "Fix auth"},
			{Name: "quartz", Role: "polecat", Rig: "mardi_gras",
				Running: false, HookBead: "mg-002", WorkTitle: "Add tests"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)

	// Should emit one dead_rig, NOT two zombies.
	var deadRig, zombie int
	for _, p := range problems {
		switch p.Type {
		case "dead_rig":
			deadRig++
		case "zombie":
			zombie++
		}
	}
	if deadRig != 1 {
		t.Errorf("expected 1 dead_rig problem, got %d", deadRig)
	}
	if zombie != 0 {
		t.Errorf("expected 0 zombie problems (suppressed by dead_rig), got %d", zombie)
	}

	// Verify orphans are attached to the dead_rig problem.
	for _, p := range problems {
		if p.Type == "dead_rig" {
			if len(p.Orphans) != 2 {
				t.Errorf("expected 2 orphans on dead_rig, got %d", len(p.Orphans))
			}
			if p.RigName != "mardi_gras" {
				t.Errorf("expected rig name mardi_gras, got %q", p.RigName)
			}
		}
	}
}

func TestDetectProblemsDeadRigDoesNotSuppressOtherRigZombie(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{
			{Name: "dead_rig", PolecatCount: 0},
			{Name: "live_rig", PolecatCount: 2},
		},
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "dead_rig",
				Running: false, HookBead: "dr-001"},
			{Name: "quartz", Role: "polecat", Rig: "live_rig",
				Running: false, HookBead: "lr-001"},
		},
	}
	problems := DetectProblems(status, BackendGasTown)

	types := map[string]int{}
	for _, p := range problems {
		types[p.Type]++
	}
	if types["dead_rig"] != 1 {
		t.Errorf("expected 1 dead_rig, got %d", types["dead_rig"])
	}
	if types["zombie"] != 1 {
		t.Errorf("expected 1 zombie (from live_rig), got %d", types["zombie"])
	}
}

func TestDoctorProblemsFiltersOK(t *testing.T) {
	diags := []DoctorDiagnostic{
		{Name: "git config", Status: "ok", Category: "Git", Explanation: "git is configured"},
		{Name: "dolt server", Status: "error", Category: "Core", Explanation: "dolt is not running", Commands: []string{"dolt sql-server --port 3307"}},
		{Name: "tmux", Status: "warn", Category: "Workflow", Explanation: "tmux not detected"},
	}
	problems := DoctorProblems(diags)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (ok filtered out), got %d", len(problems))
	}

	var errCount, warnCount int
	for _, p := range problems {
		if p.Type != "doctor" {
			t.Errorf("expected type=doctor, got %q", p.Type)
		}
		switch p.Severity {
		case "error":
			errCount++
			if p.Fix != "dolt sql-server --port 3307" {
				t.Errorf("expected first command as Fix, got %q", p.Fix)
			}
			if p.Category != "Core" {
				t.Errorf("expected Category=Core, got %q", p.Category)
			}
		case "warn":
			warnCount++
		}
	}
	if errCount != 1 {
		t.Errorf("expected 1 error severity, got %d", errCount)
	}
	if warnCount != 1 {
		t.Errorf("expected 1 warn severity, got %d", warnCount)
	}
}

func TestDoctorProblemsEmpty(t *testing.T) {
	if got := DoctorProblems(nil); got != nil {
		t.Errorf("nil diags should yield nil problems, got %v", got)
	}
	if got := DoctorProblems([]DoctorDiagnostic{{Status: "ok"}}); got != nil {
		t.Errorf("all-ok diags should yield nil problems, got %v", got)
	}
}

func TestDoctorProblemsNoFix(t *testing.T) {
	// A diagnostic without any Commands must still produce a problem; Fix
	// stays empty rather than panicking on out-of-range access.
	diags := []DoctorDiagnostic{
		{Name: "x", Status: "warn", Explanation: "no fix command provided"},
	}
	problems := DoctorProblems(diags)
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}
	if problems[0].Fix != "" {
		t.Errorf("expected empty Fix when no commands, got %q", problems[0].Fix)
	}
}

func TestDetectProblemsMultiple(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", Role: "polecat", HasWork: true, State: "idle"},        // stalled
			{Name: "Muffin", Role: "polecat", State: "backoff"},                   // backoff
			{Name: "Stale", Role: "polecat", Running: false, HookBead: "bd-e5f6"}, // zombie
		},
	}
	problems := DetectProblems(status, BackendGasTown)
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems, got %d", len(problems))
	}

	types := map[string]bool{}
	for _, p := range problems {
		types[p.Type] = true
	}
	for _, expected := range []string{"stalled", "backoff", "zombie"} {
		if !types[expected] {
			t.Errorf("expected problem type %q not found", expected)
		}
	}
}

// The dead_rig fix is a `gt` command, so it must not be suggested on a backend
// that has no gt. The overlay omits an empty Fix rather than printing bad advice.
func TestDetectProblemsDeadRigFixIsBackendSpecific(t *testing.T) {
	status := &TownStatus{
		Rigs: []RigStatus{{Name: "dead_rig", PolecatCount: 0}},
		Agents: []AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "dead_rig", Running: false, HookBead: "dr-001"},
		},
	}

	tests := []struct {
		backend string
		wantFix string
	}{
		{BackendGasTown, "gt sling <issue> dead_rig"},
		{BackendGasCity, ""},
	}
	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			var found bool
			for _, p := range DetectProblems(status, tt.backend) {
				if p.Type != "dead_rig" {
					continue
				}
				found = true
				if p.Fix != tt.wantFix {
					t.Errorf("backend %q: Fix = %q, want %q", tt.backend, p.Fix, tt.wantFix)
				}
			}
			if !found {
				t.Fatalf("backend %q: no dead_rig problem detected", tt.backend)
			}
		})
	}
}
