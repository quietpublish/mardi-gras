package gastown

import "context"

// GTDriver is the Gas Town implementation of Driver. Each method is a thin
// adapter over the package's existing `gt` CLI wrappers, so the gt behavior
// (argument shapes, validation, timeouts, output sanitization) is unchanged.
//
// The context argument is accepted to satisfy the Driver seam but is not yet
// wired into the underlying CLI calls, which manage their own timeouts via
// runWithTimeout/execWithTimeout. Plumbing ctx-based cancellation into those
// helpers is deferred (it would change gt behavior); the HTTP-based GCDriver
// added in a later phase uses ctx directly.
type GTDriver struct{}

// Compile-time assurance that GTDriver satisfies the Driver interface.
var _ Driver = GTDriver{}

// NewGTDriver returns the Gas Town driver.
func NewGTDriver() Driver { return GTDriver{} }

func (GTDriver) Backend() string { return "gastown" }

func (GTDriver) Supports(feature Feature) bool {
	switch feature {
	case FeatureVitals, FeatureCosts, FeaturePatrol:
		return true
	case FeatureSSE:
		return false
	default:
		return false
	}
}

// Reads.

func (GTDriver) Status(_ context.Context) (*TownStatus, error) { return FetchStatus() }

func (GTDriver) Formulas(_ context.Context) ([]string, error) { return ListFormulas() }

func (GTDriver) Comments(_ context.Context, issueID string) ([]Comment, error) {
	return FetchComments(issueID)
}

// Dispatch / lifecycle.

// Sling fans the unified request back out to the matching gt sling variant,
// preserving the exact single-vs-multiple and plain/agent/formula behavior of
// the original call sites. Formula takes precedence over Agent when both set.
func (GTDriver) Sling(_ context.Context, req SlingRequest) error {
	switch {
	case req.Formula != "":
		if len(req.IssueIDs) == 1 {
			return SlingWithFormula(req.IssueIDs[0], req.Formula)
		}
		return SlingMultipleWithFormula(req.IssueIDs, req.Formula)
	case req.Agent != "":
		if len(req.IssueIDs) == 1 {
			return SlingWithAgent(req.IssueIDs[0], req.Agent)
		}
		return SlingMultipleWithAgent(req.IssueIDs, req.Agent)
	default:
		if len(req.IssueIDs) == 1 {
			return Sling(req.IssueIDs[0])
		}
		return SlingMultiple(req.IssueIDs)
	}
}

func (GTDriver) Unsling(_ context.Context, issueID string) error { return Unsling(issueID) }

func (GTDriver) Nudge(_ context.Context, target, message string) error {
	return Nudge(target, message)
}

func (GTDriver) Decommission(_ context.Context, address string) error { return Decommission(address) }

func (GTDriver) CascadeClose(_ context.Context, issueID string) error { return CascadeClose(issueID) }

func (GTDriver) Assign(_ context.Context, crewMember, title, issueType, priority, label string, nudge bool) (string, error) {
	return Assign(crewMember, title, issueType, priority, label, nudge)
}

// Convoys.

func (GTDriver) ConvoyList(_ context.Context) ([]ConvoyDetail, error) { return ConvoyList() }

func (GTDriver) ConvoyStatus(_ context.Context, convoyID string) (*ConvoyDetail, error) {
	return ConvoyStatus(convoyID)
}

func (GTDriver) ConvoyCreate(_ context.Context, name string, issueIDs []string) (string, error) {
	return ConvoyCreate(name, issueIDs)
}

func (GTDriver) ConvoyCreateFromEpic(_ context.Context, name, epicID string) (string, error) {
	return ConvoyCreateFromEpic(name, epicID)
}

func (GTDriver) ConvoyClose(_ context.Context, convoyID string) error { return ConvoyClose(convoyID) }

func (GTDriver) ConvoyLand(_ context.Context, convoyID string) error { return ConvoyLand(convoyID) }

func (GTDriver) ConvoyWatch(_ context.Context, convoyID string) error { return ConvoyWatch(convoyID) }

func (GTDriver) ConvoyUnwatch(_ context.Context, convoyID string) error {
	return ConvoyUnwatch(convoyID)
}

// Mail.

func (GTDriver) MailInbox(_ context.Context, unreadOnly bool) ([]MailMessage, error) {
	return MailInbox(unreadOnly)
}

func (GTDriver) MailRead(_ context.Context, messageID string) (*MailMessage, error) {
	return MailRead(messageID)
}

func (GTDriver) MailReply(_ context.Context, messageID, body string) error {
	return MailReply(messageID, body)
}

func (GTDriver) MailSend(_ context.Context, address, subject, body string) error {
	return MailSend(address, subject, body)
}

func (GTDriver) MailArchive(_ context.Context, messageID string) error {
	return MailArchive(messageID)
}

func (GTDriver) MailMarkRead(_ context.Context, messageID string) error {
	return MailMarkRead(messageID)
}

func (GTDriver) MailMarkAllRead(_ context.Context) error { return MailMarkAllRead() }

// Molecule / workflow DAG.

func (GTDriver) MoleculeDAG(_ context.Context, rootID string) (*DAGInfo, error) {
	return MoleculeDAG(rootID)
}

func (GTDriver) MoleculeProgress(_ context.Context, rootID string) (*MoleculeProgress, error) {
	return MoleculeProgressFetch(rootID)
}

func (GTDriver) MoleculeStepDone(_ context.Context, stepID string) (*StepDoneResult, error) {
	return MoleculeStepDone(stepID)
}

// Health / analytics (Gas Town only).

func (GTDriver) Vitals(_ context.Context) (*Vitals, error) { return FetchVitals() }

func (GTDriver) Costs(_ context.Context) (*CostsOutput, error) { return FetchCosts() }

func (GTDriver) PatrolScan(_ context.Context) (*PatrolScanResult, error) { return FetchPatrolScan() }
