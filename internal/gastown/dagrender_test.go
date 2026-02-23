package gastown

import "testing"

func TestLayoutDAGLinear(t *testing.T) {
	dag := &DAGInfo{
		TierGroups: [][]string{{"s1"}, {"s2"}, {"s3"}},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Implement", Status: "in_progress", Tier: 1},
			"s3": {ID: "s3", Title: "Submit", Status: "ready", Tier: 2},
		},
	}

	rows := LayoutDAG(dag)
	// Expected: single, connector, single, connector, single
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(rows))
	}
	if rows[0].Kind != RowSingle {
		t.Errorf("row 0 kind = %d, want RowSingle", rows[0].Kind)
	}
	if rows[1].Kind != RowConnector {
		t.Errorf("row 1 kind = %d, want RowConnector", rows[1].Kind)
	}
	if rows[2].Kind != RowSingle {
		t.Errorf("row 2 kind = %d, want RowSingle", rows[2].Kind)
	}
	if rows[3].Kind != RowConnector {
		t.Errorf("row 3 kind = %d, want RowConnector", rows[3].Kind)
	}
	if rows[4].Kind != RowSingle {
		t.Errorf("row 4 kind = %d, want RowSingle", rows[4].Kind)
	}
	if rows[0].Nodes[0].Title != "Design" {
		t.Errorf("row 0 node title = %q, want %q", rows[0].Nodes[0].Title, "Design")
	}
}

func TestLayoutDAGParallel(t *testing.T) {
	dag := &DAGInfo{
		TierGroups: [][]string{{"s1"}, {"s2", "s3"}, {"s4"}},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Impl A", Status: "in_progress", Tier: 1, Parallel: true},
			"s3": {ID: "s3", Title: "Impl B", Status: "in_progress", Tier: 1, Parallel: true},
			"s4": {ID: "s4", Title: "Test", Status: "blocked", Tier: 2},
		},
	}

	rows := LayoutDAG(dag)
	// Expected: single, connector, parallel, connector, single
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(rows))
	}
	if rows[2].Kind != RowParallel {
		t.Errorf("row 2 kind = %d, want RowParallel", rows[2].Kind)
	}
	if len(rows[2].Nodes) != 2 {
		t.Errorf("row 2 nodes = %d, want 2", len(rows[2].Nodes))
	}
}

func TestLayoutDAGFiveParallel(t *testing.T) {
	// rule-of-five style: 5 parallel review aspects
	dag := &DAGInfo{
		TierGroups: [][]string{{"s1"}, {"r1", "r2", "r3", "r4", "r5"}, {"s2"}},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Implement", Status: "done", Tier: 0},
			"r1": {ID: "r1", Title: "Correctness", Status: "in_progress", Tier: 1, Parallel: true},
			"r2": {ID: "r2", Title: "Security", Status: "ready", Tier: 1, Parallel: true},
			"r3": {ID: "r3", Title: "Performance", Status: "ready", Tier: 1, Parallel: true},
			"r4": {ID: "r4", Title: "Maintainability", Status: "ready", Tier: 1, Parallel: true},
			"r5": {ID: "r5", Title: "Testing", Status: "ready", Tier: 1, Parallel: true},
			"s2": {ID: "s2", Title: "Submit", Status: "blocked", Tier: 2},
		},
	}

	rows := LayoutDAG(dag)
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(rows))
	}
	if rows[2].Kind != RowParallel {
		t.Errorf("row 2 kind = %d, want RowParallel", rows[2].Kind)
	}
	if len(rows[2].Nodes) != 5 {
		t.Errorf("row 2 nodes = %d, want 5", len(rows[2].Nodes))
	}
}

func TestLayoutDAGNil(t *testing.T) {
	rows := LayoutDAG(nil)
	if len(rows) != 0 {
		t.Errorf("nil DAG should give 0 rows, got %d", len(rows))
	}
}

func TestLayoutDAGEmpty(t *testing.T) {
	dag := &DAGInfo{
		TierGroups: [][]string{},
		Nodes:      map[string]*DAGNode{},
	}
	rows := LayoutDAG(dag)
	if len(rows) != 0 {
		t.Errorf("empty DAG should give 0 rows, got %d", len(rows))
	}
}

func TestLayoutDAGSkipsEmptyTiers(t *testing.T) {
	dag := &DAGInfo{
		TierGroups: [][]string{{"s1"}, {}, {"s2"}},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "A", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "B", Status: "ready", Tier: 2},
		},
	}

	rows := LayoutDAG(dag)
	// Empty tier skipped: single, connector, single
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
}

func TestLayoutDAGSkipsMissingNodes(t *testing.T) {
	dag := &DAGInfo{
		TierGroups: [][]string{{"s1", "missing"}},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "A", Status: "done", Tier: 0},
		},
	}

	rows := LayoutDAG(dag)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	// "missing" node not in Nodes map → only s1 present → RowSingle
	if rows[0].Kind != RowSingle {
		t.Errorf("row kind = %d, want RowSingle", rows[0].Kind)
	}
}

func TestCriticalPathSet(t *testing.T) {
	dag := &DAGInfo{
		CriticalPath: []string{"s1", "s3", "s5"},
	}
	set := CriticalPathSet(dag)
	if !set["s1"] {
		t.Error("s1 should be on critical path")
	}
	if !set["s3"] {
		t.Error("s3 should be on critical path")
	}
	if set["s2"] {
		t.Error("s2 should not be on critical path")
	}
}

func TestCriticalPathSetNil(t *testing.T) {
	set := CriticalPathSet(nil)
	if len(set) != 0 {
		t.Errorf("nil DAG should give empty set, got %d", len(set))
	}
}

func TestCriticalPathTitles(t *testing.T) {
	dag := &DAGInfo{
		CriticalPath: []string{"s1", "s3", "s5"},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Design"},
			"s3": {ID: "s3", Title: "Implement B"},
			"s5": {ID: "s5", Title: "Submit"},
		},
	}
	titles := CriticalPathTitles(dag)
	if len(titles) != 3 {
		t.Fatalf("got %d titles, want 3", len(titles))
	}
	if titles[0] != "Design" {
		t.Errorf("titles[0] = %q, want %q", titles[0], "Design")
	}
	if titles[1] != "Implement B" {
		t.Errorf("titles[1] = %q, want %q", titles[1], "Implement B")
	}
	if titles[2] != "Submit" {
		t.Errorf("titles[2] = %q, want %q", titles[2], "Submit")
	}
}

func TestCriticalPathTitlesMissingNode(t *testing.T) {
	dag := &DAGInfo{
		CriticalPath: []string{"s1", "missing"},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Design"},
		},
	}
	titles := CriticalPathTitles(dag)
	if len(titles) != 2 {
		t.Fatalf("got %d titles, want 2", len(titles))
	}
	// Missing node falls back to ID
	if titles[1] != "missing" {
		t.Errorf("titles[1] = %q, want %q", titles[1], "missing")
	}
}

func TestCriticalPathString(t *testing.T) {
	dag := &DAGInfo{
		CriticalPath: []string{"s1", "s3"},
		Nodes: map[string]*DAGNode{
			"s1": {ID: "s1", Title: "Design"},
			"s3": {ID: "s3", Title: "Submit"},
		},
	}
	s := CriticalPathString(dag)
	if s != "Design → Submit" {
		t.Errorf("CriticalPathString = %q, want %q", s, "Design → Submit")
	}
}

func TestCriticalPathStringNil(t *testing.T) {
	s := CriticalPathString(nil)
	if s != "" {
		t.Errorf("nil DAG should give empty string, got %q", s)
	}
}
