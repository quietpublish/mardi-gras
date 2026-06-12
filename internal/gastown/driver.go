package gastown

import (
	"context"
	"errors"
)

// ErrUnsupported is returned by a Driver for operations the backing
// orchestrator cannot perform (e.g. vitals/costs/patrol on Gas City).
// Callers should gate the corresponding UI on Driver.Supports and treat
// ErrUnsupported as "hide this feature", not as a failure to surface.
var ErrUnsupported = errors.New("operation not supported by this driver")

// Feature identifies an optional capability a Driver may or may not provide.
// The UI consults Driver.Supports to decide whether to render a panel/section.
type Feature int

const (
	// FeatureVitals is `gt vitals` (Dolt/backup health). Gas Town only.
	FeatureVitals Feature = iota
	// FeatureCosts is `gt costs` (spend dashboard). Gas Town only.
	FeatureCosts
	// FeaturePatrol is `gt patrol scan` (zombie/stall diagnostics). Gas Town only.
	FeaturePatrol
	// FeatureSSE is a live server-sent-events status stream (Gas City; Phase 4).
	FeatureSSE
)

// SlingRequest collapses the several `gt sling` variants (single/multiple,
// plain/agent-override/formula) into one shape. A Gas Town driver fans it
// back out to the matching CLI call; a Gas City driver maps it to a single
// POST /v0/city/{city}/sling. Agent and Formula are mutually exclusive
// (Formula wins if both are set, mirroring the existing call sites).
type SlingRequest struct {
	IssueIDs []string // one or more issues to dispatch
	Agent    string   // optional --agent runtime override (e.g. "codex")
	Formula  string   // optional formula name (full workflow)

	// Reserved for the Gas City driver; unused by the Gas Town driver.
	Target string
	Rig    string
	Title  string
	Force  bool
}

// Driver is the seam between mg and the multi-agent orchestrator. The Gas
// Town implementation (GTDriver) shells out to the `gt` CLI; a future Gas
// City implementation (GCDriver) will speak the Supervisor HTTP API. The app
// holds exactly one Driver, selected at startup.
//
// Every method takes a context.Context so an HTTP driver gets
// cancellation/timeout for free. GTDriver currently ignores it (its wrapped
// CLI calls manage their own timeouts); that is the intended Phase 1 state.
//
// Pure analytics over already-fetched data (ComputeVelocity, ComputeScorecards,
// PredictConvoys, LayoutDAG, …), recovery helpers (FindOrphans, FindDeadRigs,
// RecoverRig), local event-log reads (LoadRecentEvents), and the local-tmux
// HandoffInTmux are deliberately NOT on this interface — they are
// driver-agnostic and stay package-level functions.
type Driver interface {
	// Backend reports the orchestrator name ("gastown" | "gascity").
	Backend() string
	// Supports reports whether this driver can perform the given feature.
	Supports(feature Feature) bool

	// Reads.
	Status(ctx context.Context) (*TownStatus, error)
	Formulas(ctx context.Context) ([]string, error)
	Comments(ctx context.Context, issueID string) ([]Comment, error)

	// Dispatch / lifecycle.
	Sling(ctx context.Context, req SlingRequest) error
	Unsling(ctx context.Context, issueID string) error
	Nudge(ctx context.Context, target, message string) error
	Decommission(ctx context.Context, address string) error
	CascadeClose(ctx context.Context, issueID string) error
	Assign(ctx context.Context, crewMember, title, issueType, priority, label string, nudge bool) (string, error)

	// Convoys.
	ConvoyList(ctx context.Context) ([]ConvoyDetail, error)
	ConvoyStatus(ctx context.Context, convoyID string) (*ConvoyDetail, error)
	ConvoyCreate(ctx context.Context, name string, issueIDs []string) (string, error)
	ConvoyCreateFromEpic(ctx context.Context, name, epicID string) (string, error)
	ConvoyClose(ctx context.Context, convoyID string) error
	ConvoyLand(ctx context.Context, convoyID string) error
	ConvoyWatch(ctx context.Context, convoyID string) error
	ConvoyUnwatch(ctx context.Context, convoyID string) error

	// Mail.
	MailInbox(ctx context.Context, unreadOnly bool) ([]MailMessage, error)
	MailRead(ctx context.Context, messageID string) (*MailMessage, error)
	MailReply(ctx context.Context, messageID, body string) error
	MailSend(ctx context.Context, address, subject, body string) error
	MailArchive(ctx context.Context, messageID string) error
	MailMarkRead(ctx context.Context, messageID string) error
	MailMarkAllRead(ctx context.Context) error

	// Molecule / workflow DAG.
	MoleculeDAG(ctx context.Context, rootID string) (*DAGInfo, error)
	MoleculeProgress(ctx context.Context, rootID string) (*MoleculeProgress, error)
	MoleculeStepDone(ctx context.Context, stepID string) (*StepDoneResult, error)

	// Health / analytics with no Gas City equivalent — a GCDriver returns
	// ErrUnsupported and the UI hides the corresponding section.
	Vitals(ctx context.Context) (*Vitals, error)
	Costs(ctx context.Context) (*CostsOutput, error)
	PatrolScan(ctx context.Context) (*PatrolScanResult, error)
}
