// Package data defines Beads issue types and provides data loading (JSONL and
// CLI), filtering, sorting, mutations, focus mode, and dependency evaluation.
package data

import (
	"fmt"
	"strings"
	"time"
)

// Status represents the state of a Beads issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// IssueType represents the category of a Beads issue.
type IssueType string

const (
	TypeTask      IssueType = "task"
	TypeBug       IssueType = "bug"
	TypeFeature   IssueType = "feature"
	TypeChore     IssueType = "chore"
	TypeEpic      IssueType = "epic"
	TypeSpike     IssueType = "spike"
	TypeStory     IssueType = "story"
	TypeMilestone IssueType = "milestone"
)

// Priority ranges from 0 (critical) to 4 (backlog).
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// ParadeStatus maps issues to their parade float group.
type ParadeStatus int

const (
	ParadeRolling      ParadeStatus = iota // in_progress
	ParadeLinedUp                          // open, not blocked
	ParadeStalled                          // open, blocked
	ParadePastTheStand                     // closed
)

// Dependency represents a relationship between two issues.
type Dependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

// DefaultBlockingTypes is the set of dependency types that count as blockers.
var DefaultBlockingTypes = map[string]bool{"blocks": true, "conditional-blocks": true}

// DepStatus classifies a single dependency edge after evaluation.
type DepStatus int

const (
	DepBlocking    DepStatus = iota // unresolved blocker exists
	DepResolved                     // blocker exists but is closed
	DepMissing                      // depends_on_id not found in map
	DepNonBlocking                  // dep type not in blockingTypes set
)

// DepEdge is a single evaluated dependency relationship.
type DepEdge struct {
	Type        string
	DependsOnID string
	Status      DepStatus
}

// DepEval is the result of evaluating all dependencies for an issue.
type DepEval struct {
	Edges         []DepEdge
	BlockingIDs   []string  // exist + not closed
	ResolvedIDs   []string  // exist + closed
	MissingIDs    []string  // not found in issueMap
	NonBlocking   []DepEdge // dep type not in blockingTypes
	IsBlocked     bool
	NextBlockerID string // first of BlockingIDs, else first of MissingIDs
}

// Issue represents a single Beads issue.
type Issue struct {
	ID                 string                 `json:"id"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description,omitempty"`
	Status             Status                 `json:"status"`
	Priority           Priority               `json:"priority"`
	IssueType          IssueType              `json:"issue_type"`
	Owner              string                 `json:"owner,omitempty"`
	Assignee           string                 `json:"assignee,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	CreatedBy          string                 `json:"created_by,omitempty"`
	UpdatedAt          time.Time              `json:"updated_at"`
	StartedAt          *time.Time             `json:"started_at,omitempty"`
	ClosedAt           *time.Time             `json:"closed_at,omitempty"`
	CloseReason        string                 `json:"close_reason,omitempty"`
	Dependencies       []Dependency           `json:"dependencies,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	Design             string                 `json:"design,omitempty"`
	AcceptanceCriteria string                 `json:"acceptance_criteria,omitempty"`
	Labels             []string               `json:"labels,omitempty"`
	DueAt              *time.Time             `json:"due_at,omitempty"`
	DeferUntil         *time.Time             `json:"defer_until,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	// CommentCount comes free with `bd list --json` (bd v1.1.0+). The parade
	// badges it so discussion is visible without opening the issue; the detail
	// panel's "COMMENTS (n)" header still counts the fetched comments instead,
	// since those arrive from a separate `bd comments` call.
	CommentCount int `json:"comment_count,omitempty"`
}

// EvaluateDependencies is the canonical function for classifying all dependency
// edges on an issue. It de-duplicates by type|dependsOnID, classifies each edge,
// and computes aggregate blocked state.
func (i *Issue) EvaluateDependencies(issueMap map[string]*Issue, blockingTypes map[string]bool) DepEval {
	var eval DepEval
	seen := make(map[string]bool)

	for _, dep := range i.Dependencies {
		key := dep.Type + "|" + dep.DependsOnID
		if seen[key] {
			continue
		}
		seen[key] = true

		edge := DepEdge{Type: dep.Type, DependsOnID: dep.DependsOnID}

		if !blockingTypes[dep.Type] {
			edge.Status = DepNonBlocking
			eval.NonBlocking = append(eval.NonBlocking, edge)
			eval.Edges = append(eval.Edges, edge)
			continue
		}

		target, exists := issueMap[dep.DependsOnID]
		switch {
		case !exists:
			edge.Status = DepMissing
			eval.MissingIDs = append(eval.MissingIDs, dep.DependsOnID)
		case target.Status == StatusClosed:
			edge.Status = DepResolved
			eval.ResolvedIDs = append(eval.ResolvedIDs, dep.DependsOnID)
		default:
			edge.Status = DepBlocking
			eval.BlockingIDs = append(eval.BlockingIDs, dep.DependsOnID)
		}
		eval.Edges = append(eval.Edges, edge)
	}

	eval.IsBlocked = len(eval.BlockingIDs) > 0 || len(eval.MissingIDs) > 0
	if len(eval.BlockingIDs) > 0 {
		eval.NextBlockerID = eval.BlockingIDs[0]
	} else if len(eval.MissingIDs) > 0 {
		eval.NextBlockerID = eval.MissingIDs[0]
	}
	return eval
}

// IsBlocked returns true if this issue depends on an unclosed blocker.
// Delegates to EvaluateDependencies with DefaultBlockingTypes.
func (i *Issue) IsBlocked(issueMap map[string]*Issue) bool {
	return i.EvaluateDependencies(issueMap, DefaultBlockingTypes).IsBlocked
}

// BlockedByIDs returns the IDs of open issues blocking this one.
// Delegates to EvaluateDependencies with DefaultBlockingTypes.
func (i *Issue) BlockedByIDs(issueMap map[string]*Issue) []string {
	eval := i.EvaluateDependencies(issueMap, DefaultBlockingTypes)
	return eval.BlockingIDs
}

// BlocksIDs returns the IDs of issues that this issue blocks.
func (i *Issue) BlocksIDs(allIssues []Issue, blockingTypes map[string]bool) []string {
	var blocked []string
	for _, other := range allIssues {
		for _, dep := range other.Dependencies {
			if blockingTypes[dep.Type] && dep.DependsOnID == i.ID {
				blocked = append(blocked, other.ID)
				break
			}
		}
	}
	return blocked
}

// Age returns the duration since the issue was created.
func (i *Issue) Age() time.Duration {
	return time.Since(i.CreatedAt)
}

// AgeLabel returns a human-readable age string.
func (i *Issue) AgeLabel() string {
	days := int(i.Age().Hours() / 24)
	switch {
	case days == 0:
		hours := int(i.Age().Hours())
		if hours == 0 {
			return "just now"
		}
		return fmt.Sprintf("%dh", hours)
	case days == 1:
		return "1 day"
	case days < 30:
		return fmt.Sprintf("%d days", days)
	default:
		return fmt.Sprintf("%d weeks", days/7)
	}
}

// ParadeGroup determines which parade section this issue belongs to.
// Stalled wins over Rolling: an in_progress issue with unresolved blockers is Stalled.
func (i *Issue) ParadeGroup(issueMap map[string]*Issue, blockingTypes map[string]bool) ParadeStatus {
	switch i.Status {
	case StatusClosed:
		return ParadePastTheStand
	case StatusInProgress:
		if i.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ParadeStalled
		}
		return ParadeRolling
	case StatusOpen:
		if i.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ParadeStalled
		}
		return ParadeLinedUp
	default:
		return ParadeLinedUp
	}
}

// PriorityLabel returns "P0" through "P4".
func PriorityLabel(p Priority) string {
	return fmt.Sprintf("P%d", p)
}

// PriorityName returns the full name for a priority level.
func PriorityName(p Priority) string {
	switch p {
	case PriorityCritical:
		return "Critical"
	case PriorityHigh:
		return "High"
	case PriorityMedium:
		return "Medium"
	case PriorityLow:
		return "Low"
	case PriorityBacklog:
		return "Backlog"
	default:
		return "Unknown"
	}
}

// ParentID returns the parent issue ID based on dot-separated hierarchy.
// "mg-007.2.1" → "mg-007.2", "mg-007" → "".
func (i *Issue) ParentID() string {
	idx := strings.LastIndex(i.ID, ".")
	if idx < 0 {
		return ""
	}
	return i.ID[:idx]
}

// ParentRelationshipID returns the ID this issue names as its parent through a
// `parent-child` dependency, or "" if it has none. Beads writes this edge when
// an issue is created with --parent, alongside the dotted ID; removing the edge
// while keeping the ID is exactly the reparenting case that made dotted-ID
// depth unreliable. Multi-parent is first-wins.
func (i *Issue) ParentRelationshipID() string {
	for _, dep := range i.Dependencies {
		if dep.Type == "parent-child" {
			return dep.DependsOnID
		}
	}
	return ""
}

// ParentRelationshipDepth returns the number of loaded parent-child ancestors.
// Missing parents and cycles stop the walk so an issue is never indented under
// an unrelated visible row.
func (i *Issue) ParentRelationshipDepth(issueMap map[string]*Issue) int {
	depth := 0
	current := i
	seen := map[string]bool{i.ID: true}

	for {
		parentID := current.ParentRelationshipID()
		parent, ok := issueMap[parentID]
		if parentID == "" || !ok || seen[parentID] {
			return depth
		}
		depth++
		seen[parentID] = true
		current = parent
	}
}

// IsOverdue returns true if DueAt is set, in the past, and the issue is not closed.
func (i *Issue) IsOverdue() bool {
	return i.DueAt != nil && i.Status != StatusClosed && i.DueAt.Before(time.Now())
}

// IsDeferred returns true if DeferUntil is set and still in the future.
func (i *Issue) IsDeferred() bool {
	return i.DeferUntil != nil && i.DeferUntil.After(time.Now())
}

// DueLabel returns a human-readable due-date label.
// Overdue: "3d overdue", same day: "due today", future: "2d left".
func (i *Issue) DueLabel() string {
	if i.DueAt == nil {
		return ""
	}
	now := time.Now()
	due := *i.DueAt
	days := int(due.Sub(now).Hours() / 24)

	switch {
	case days < -1:
		return fmt.Sprintf("%dd overdue", -days)
	case days < 0:
		// Less than 24h overdue but past due
		return "due today"
	case days == 0:
		return "due today"
	case days == 1:
		return "1d left"
	default:
		return fmt.Sprintf("%dd left", days)
	}
}

// DeferLabel returns a human-readable defer label.
func (i *Issue) DeferLabel() string {
	if i.DeferUntil == nil {
		return ""
	}
	now := time.Now()
	days := int(i.DeferUntil.Sub(now).Hours() / 24)
	if days <= 0 {
		return ""
	}
	return fmt.Sprintf("deferred %dd", days)
}

// BuildIssueMap creates a lookup map from a slice of issues.
func BuildIssueMap(issues []Issue) map[string]*Issue {
	m := make(map[string]*Issue, len(issues))
	for idx := range issues {
		m[issues[idx].ID] = &issues[idx]
	}
	return m
}

// RelativeAge renders a duration as a compact human age ("42s ago", "5m ago",
// "3h ago", "12d ago", "5w ago"). Hours cap at a day so long gaps don't read
// as "969h ago".
func RelativeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	}
}

// OrderHierarchically reorders one parade section so each child immediately
// follows its parent, and reports the indent depth of every row.
//
// Ordering is depth-first: roots keep the order they arrive in (priority, then
// recency), and each root is followed by its descendants in that same relative
// order. Only issues present in this slice can act as parents, so a child whose
// parent sits in another section — or is filtered out entirely — becomes a root
// at depth 0 rather than being indented under whichever unrelated row happened
// to precede it. That is what makes the indent mean "the row above me is my
// parent" instead of merely "my ID has a dot in it".
//
// Cycles are broken by visiting each issue at most once; an issue reachable
// only through a cycle is emitted as a root so it can never disappear.
func OrderHierarchically(issues []Issue) (ordered []*Issue, depth map[string]int) {
	depth = make(map[string]int, len(issues))
	if len(issues) == 0 {
		return nil, depth
	}

	inSection := make(map[string]*Issue, len(issues))
	for i := range issues {
		inSection[issues[i].ID] = &issues[i]
	}

	children := make(map[string][]*Issue, len(issues))
	var roots []*Issue
	for i := range issues {
		iss := &issues[i]
		parentID := iss.ParentRelationshipID()
		if parentID == "" || parentID == iss.ID {
			roots = append(roots, iss)
			continue
		}
		if _, ok := inSection[parentID]; !ok {
			roots = append(roots, iss) // parent not on screen here
			continue
		}
		children[parentID] = append(children[parentID], iss)
	}

	ordered = make([]*Issue, 0, len(issues))
	visited := make(map[string]bool, len(issues))
	var walk func(iss *Issue, d int)
	walk = func(iss *Issue, d int) {
		if visited[iss.ID] {
			return
		}
		visited[iss.ID] = true
		ordered = append(ordered, iss)
		depth[iss.ID] = d
		for _, child := range children[iss.ID] {
			walk(child, d+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}
	// Anything reachable only through a cycle never got emitted above; surface
	// it at top level rather than dropping it from the parade.
	for i := range issues {
		walk(&issues[i], 0)
	}
	return ordered, depth
}
