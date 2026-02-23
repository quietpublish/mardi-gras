package gastown

import "strings"

// DAGRowKind identifies the type of rendered DAG row.
type DAGRowKind int

const (
	RowSingle    DAGRowKind = iota // single node in a tier
	RowParallel                    // multiple nodes in a tier (parallel)
	RowConnector                   // flow connector between tiers
)

// DAGRow represents one renderable row in a laid-out molecule DAG.
type DAGRow struct {
	Kind  DAGRowKind
	Nodes []*DAGNode // 1 for RowSingle, N for RowParallel, 0 for RowConnector
}

// LayoutDAG converts a DAGInfo into an ordered slice of renderable rows.
// Tiers produce node rows, with flow connector rows inserted between them.
// Tiers with multiple nodes produce a RowParallel (branching).
func LayoutDAG(dag *DAGInfo) []DAGRow {
	if dag == nil || len(dag.TierGroups) == 0 {
		return nil
	}

	var rows []DAGRow
	firstTier := true

	for _, tier := range dag.TierGroups {
		if len(tier) == 0 {
			continue
		}

		// Collect nodes for this tier
		var nodes []*DAGNode
		for _, id := range tier {
			if node, ok := dag.Nodes[id]; ok {
				nodes = append(nodes, node)
			}
		}
		if len(nodes) == 0 {
			continue
		}

		// Flow connector between tiers
		if !firstTier {
			rows = append(rows, DAGRow{Kind: RowConnector})
		}
		firstTier = false

		if len(nodes) == 1 {
			rows = append(rows, DAGRow{Kind: RowSingle, Nodes: nodes})
		} else {
			rows = append(rows, DAGRow{Kind: RowParallel, Nodes: nodes})
		}
	}

	return rows
}

// CriticalPathSet returns a set of node IDs on the critical path for fast lookup.
func CriticalPathSet(dag *DAGInfo) map[string]bool {
	set := make(map[string]bool)
	if dag == nil {
		return set
	}
	for _, id := range dag.CriticalPath {
		set[id] = true
	}
	return set
}

// CriticalPathTitles returns the critical path as a list of node titles.
func CriticalPathTitles(dag *DAGInfo) []string {
	if dag == nil {
		return nil
	}
	var titles []string
	for _, id := range dag.CriticalPath {
		if node, ok := dag.Nodes[id]; ok {
			titles = append(titles, node.Title)
		} else {
			titles = append(titles, id)
		}
	}
	return titles
}

// CriticalPathString formats the critical path as "title → title → title".
func CriticalPathString(dag *DAGInfo) string {
	titles := CriticalPathTitles(dag)
	if len(titles) == 0 {
		return ""
	}
	return strings.Join(titles, " → ")
}
