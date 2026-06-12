package gastown

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/gastown/gcclient"
)

// GCDriver is the Gas City implementation of Driver. It speaks the Gas City
// Supervisor HTTP API (https://docs.gascityhall.com/reference/api) via the
// generated gcclient package instead of shelling out to a CLI.
//
// Phase 2 implements the read path (Status → live roster). Dispatch, mail,
// convoy, and molecule operations return ErrUnsupported until Phase 3; the
// gt-only health features (vitals/costs/patrol) have no Gas City equivalent
// and stay ErrUnsupported permanently — callers leave those sections empty.
type GCDriver struct {
	baseURL string
	city    string // optional pin; "" = resolve the first running city
	client  *gcclient.ClientWithResponses
}

// Compile-time assurance that GCDriver satisfies the Driver interface.
var _ Driver = (*GCDriver)(nil)

// NewGCDriver builds a Gas City driver against the supervisor at baseURL
// (scheme://host:port, no /v0 suffix). city may be empty to auto-resolve.
func NewGCDriver(baseURL, city string) (*GCDriver, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	c, err := gcclient.NewClientWithResponses(baseURL, gcclient.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gc client: %w", err)
	}
	return &GCDriver{baseURL: baseURL, city: strings.TrimSpace(city), client: c}, nil
}

func (*GCDriver) Backend() string { return "gascity" }

// Supports reports false for every optional feature today: vitals/costs/patrol
// have no Gas City equivalent, and the SSE stream lands in Phase 4.
func (*GCDriver) Supports(Feature) bool { return false }

// Status fetches the live agent roster over HTTP and adapts it to TownStatus.
func (d *GCDriver) Status(ctx context.Context) (*TownStatus, error) {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.GetV0CityByCityNameAgentsWithResponse(ctx, city, nil)
	if err != nil {
		return nil, fmt.Errorf("gc agents: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("gc agents: %s", gcRespErr(resp.StatusCode(), resp.ApplicationproblemJSONDefault))
	}
	var agents []AgentRuntime
	if resp.JSON200.Items != nil {
		agents = make([]AgentRuntime, 0, len(*resp.JSON200.Items))
		for _, a := range *resp.JSON200.Items {
			agents = append(agents, gcAgentToRuntime(a))
		}
	}
	return &TownStatus{Agents: agents, Rigs: gcDeriveRigs(agents)}, nil
}

// resolveCity returns the pinned city, or the first running city reported by
// the supervisor (falling back to the first city of any state).
func (d *GCDriver) resolveCity(ctx context.Context) (string, error) {
	if d.city != "" {
		return d.city, nil
	}
	resp, err := d.client.GetV0CitiesWithResponse(ctx)
	if err != nil {
		return "", fmt.Errorf("gc cities: %w", err)
	}
	if resp.JSON200 == nil || resp.JSON200.Items == nil {
		return "", fmt.Errorf("gc cities: %s", gcRespErr(resp.StatusCode(), resp.ApplicationproblemJSONDefault))
	}
	items := *resp.JSON200.Items
	for _, c := range items {
		if c.Running {
			return c.Name, nil
		}
	}
	if len(items) > 0 {
		return items[0].Name, nil
	}
	return "", fmt.Errorf("gc: no cities found at %s", d.baseURL)
}

// gcRespErr renders an RFC 9457 problem+json body (or a bare status) for errors.
func gcRespErr(code int, e *gcclient.ErrorModel) string {
	if e != nil && e.Detail != nil && *e.Detail != "" {
		return fmt.Sprintf("%d: %s", code, *e.Detail)
	}
	return fmt.Sprintf("status %d", code)
}

// --- adapter: gcclient.AgentResponse -> AgentRuntime -----------------------

func gcAgentToRuntime(a gcclient.AgentResponse) AgentRuntime {
	rt := AgentRuntime{
		Name:       a.Name,
		Rig:        derefString(a.Rig),
		Running:    a.Running,
		State:      a.State,
		HookBead:   derefString(a.ActiveBead),
		WorkTitle:  derefString(a.Activity),
		Address:    a.Name, // qualified name doubles as the dispatch handle
		AgentAlias: derefString(a.DisplayName),
		AgentInfo:  gcAgentInfo(a.Provider, a.Model),
	}
	if rt.State == "" {
		rt.State = "idle"
	}
	rt.Role = gcInferRole(derefString(a.Pool), a.Name)
	rt.HasWork = rt.HookBead != "" || rt.WorkTitle != ""
	if a.Session != nil {
		rt.Session = a.Session.Name
	}
	return rt
}

// gcInferRole maps a Gas City pool (or the agent name) onto the Gas Town role
// vocabulary mg renders. Unknown pools pass through verbatim so RoleColor
// degrades gracefully.
func gcInferRole(pool, name string) string {
	knownRoles := []string{"mayor", "deacon", "witness", "refinery", "polecat", "crew", "dog"}
	p := strings.ToLower(strings.TrimSpace(pool))
	if p != "" {
		for _, r := range knownRoles {
			if strings.Contains(p, r) {
				return r
			}
		}
		return p
	}
	n := strings.ToLower(name)
	for _, r := range knownRoles {
		if strings.Contains(n, r) {
			return r
		}
	}
	return ""
}

// gcAgentInfo renders the gt-style "provider/model" display string.
func gcAgentInfo(provider, model *string) string {
	p, m := derefString(provider), derefString(model)
	switch {
	case p != "" && m != "":
		return p + "/" + m
	case m != "":
		return m
	default:
		return p
	}
}

// gcDeriveRigs builds a minimal RigStatus list from the roster (Gas City's
// agents endpoint has no separate rig summary). Counts are best-effort by
// inferred role; richer rig detail can come from /status in a later phase.
func gcDeriveRigs(agents []AgentRuntime) []RigStatus {
	order := make([]string, 0)
	idx := make(map[string]*RigStatus)
	for _, a := range agents {
		if a.Rig == "" {
			continue
		}
		rs, ok := idx[a.Rig]
		if !ok {
			rs = &RigStatus{Name: a.Rig}
			idx[a.Rig] = rs
			order = append(order, a.Rig)
		}
		switch a.Role {
		case "polecat":
			rs.PolecatCount++
		case "crew":
			rs.CrewCount++
		case "witness":
			rs.HasWitness = true
		case "refinery":
			rs.HasRefinery = true
		}
	}
	out := make([]RigStatus, 0, len(order))
	for _, name := range order {
		out = append(out, *idx[name])
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- mail + formulas --------------------------------------------------------

// gcRequestToken is the anti-CSRF value sent in the X-GC-Request header on
// every mutation. The supervisor only checks that the header is present and
// non-empty (RFC: "csrf: ..." on a missing header).
const gcRequestToken = "mardi-gras"

// Formulas lists available formula names via GET /v0/city/{city}/formulas.
func (d *GCDriver) Formulas(ctx context.Context) ([]string, error) {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.GetV0CityByCityNameFormulasWithResponse(ctx, city, nil)
	if err != nil {
		return nil, fmt.Errorf("gc formulas: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("gc formulas: %s", gcRespErr(resp.StatusCode(), resp.ApplicationproblemJSONDefault))
	}
	var names []string
	if resp.JSON200.Items != nil {
		names = make([]string, 0, len(*resp.JSON200.Items))
		for _, f := range *resp.JSON200.Items {
			names = append(names, f.Name)
		}
	}
	return names, nil
}

// MailInbox fetches the mailbox via GET /v0/city/{city}/mail. When unreadOnly
// is set it passes status=unread. It deliberately omits the index/wait params
// so the call does not long-poll.
func (d *GCDriver) MailInbox(ctx context.Context, unreadOnly bool) ([]MailMessage, error) {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return nil, err
	}
	params := &gcclient.GetV0CityByCityNameMailParams{}
	if unreadOnly {
		status := "unread"
		params.Status = &status
	}
	resp, err := d.client.GetV0CityByCityNameMailWithResponse(ctx, city, params)
	if err != nil {
		return nil, fmt.Errorf("gc mail: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("gc mail: %s", gcRespErr(resp.StatusCode(), resp.ApplicationproblemJSONDefault))
	}
	var msgs []MailMessage
	if resp.JSON200.Items != nil {
		msgs = make([]MailMessage, 0, len(*resp.JSON200.Items))
		for _, m := range *resp.JSON200.Items {
			msgs = append(msgs, gcMessageToMail(m))
		}
	}
	return msgs, nil
}

// MailRead fetches a single message via GET /v0/city/{city}/mail/{id}.
func (d *GCDriver) MailRead(ctx context.Context, messageID string) (*MailMessage, error) {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.GetV0CityByCityNameMailByIdWithResponse(ctx, city, messageID, nil)
	if err != nil {
		return nil, fmt.Errorf("gc mail read: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("gc mail read: %s", gcRespErr(resp.StatusCode(), resp.ApplicationproblemJSONDefault))
	}
	m := gcMessageToMail(*resp.JSON200)
	return &m, nil
}

// MailReply replies to a message via POST /v0/city/{city}/mail/{id}/reply.
func (d *GCDriver) MailReply(ctx context.Context, messageID, body string) error {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return err
	}
	resp, err := d.client.ReplyMailWithResponse(ctx, city, messageID,
		&gcclient.ReplyMailParams{XGCRequest: gcRequestToken},
		gcclient.ReplyMailJSONRequestBody{Body: &body})
	if err != nil {
		return fmt.Errorf("gc mail reply: %w", err)
	}
	return gcMutationErr("gc mail reply", resp.StatusCode(), resp.ApplicationproblemJSONDefault)
}

// MailSend sends a new message via POST /v0/city/{city}/mail.
func (d *GCDriver) MailSend(ctx context.Context, address, subject, body string) error {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return err
	}
	resp, err := d.client.SendMailWithResponse(ctx, city,
		&gcclient.SendMailParams{XGCRequest: gcRequestToken},
		gcclient.SendMailJSONRequestBody{To: address, Subject: subject, Body: &body})
	if err != nil {
		return fmt.Errorf("gc mail send: %w", err)
	}
	return gcMutationErr("gc mail send", resp.StatusCode(), resp.ApplicationproblemJSONDefault)
}

// MailArchive archives a message via POST /v0/city/{city}/mail/{id}/archive.
func (d *GCDriver) MailArchive(ctx context.Context, messageID string) error {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return err
	}
	resp, err := d.client.PostV0CityByCityNameMailByIdArchiveWithResponse(ctx, city, messageID,
		&gcclient.PostV0CityByCityNameMailByIdArchiveParams{XGCRequest: gcRequestToken})
	if err != nil {
		return fmt.Errorf("gc mail archive: %w", err)
	}
	return gcMutationErr("gc mail archive", resp.StatusCode(), resp.ApplicationproblemJSONDefault)
}

// MailMarkRead marks a message read via POST /v0/city/{city}/mail/{id}/read.
func (d *GCDriver) MailMarkRead(ctx context.Context, messageID string) error {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return err
	}
	resp, err := d.client.PostV0CityByCityNameMailByIdReadWithResponse(ctx, city, messageID,
		&gcclient.PostV0CityByCityNameMailByIdReadParams{XGCRequest: gcRequestToken})
	if err != nil {
		return fmt.Errorf("gc mail mark-read: %w", err)
	}
	return gcMutationErr("gc mail mark-read", resp.StatusCode(), resp.ApplicationproblemJSONDefault)
}

// MailMarkAllRead marks every unread message read. Gas City has no bulk
// endpoint, so it fetches the unread set and marks each one individually.
func (d *GCDriver) MailMarkAllRead(ctx context.Context) error {
	msgs, err := d.MailInbox(ctx, true)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		if m.Read {
			continue
		}
		if err := d.MailMarkRead(ctx, m.ID); err != nil {
			return err
		}
	}
	return nil
}

// gcMessageToMail adapts a Gas City Message to mg's MailMessage. Priority is
// left empty: Gas City reports it as an integer whose scale mg can't map onto
// gt's textual priorities without guessing.
func gcMessageToMail(m gcclient.Message) MailMessage {
	return MailMessage{
		ID:       m.Id,
		From:     m.From,
		To:       m.To,
		Subject:  m.Subject,
		Body:     m.Body,
		Time:     m.CreatedAt.Format(time.RFC3339),
		Read:     m.Read,
		ThreadID: derefString(m.ThreadId),
	}
}

// gcMutationErr converts a mutation response into an error: nil on 2xx,
// otherwise an error carrying the problem+json detail.
func gcMutationErr(label string, code int, e *gcclient.ErrorModel) error {
	if code >= 200 && code < 300 {
		return nil
	}
	return fmt.Errorf("%s: %s", label, gcRespErr(code, e))
}

// --- unsupported operations -------------------------------------------------
//
// These either have no Gas City endpoint (Unsling, ConvoyLand/Watch/Unwatch),
// require semantic mappings that need validation against a live `gc`
// (Sling needs a required `target`; Nudge/Decommission need session/agent
// resolution; convoys are modeled as beads), or have no equivalent at all
// (Vitals/Costs/PatrolScan). See the operation support matrix in
// docs/internal/gascity-integration-design.md §6.3.

func (*GCDriver) Comments(context.Context, string) ([]Comment, error) { return nil, ErrUnsupported }

func (*GCDriver) Sling(context.Context, SlingRequest) error { return ErrUnsupported }

func (*GCDriver) Unsling(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) Nudge(context.Context, string, string) error { return ErrUnsupported }

func (*GCDriver) Decommission(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) CascadeClose(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) Assign(context.Context, string, string, string, string, string, bool) (string, error) {
	return "", ErrUnsupported
}

func (*GCDriver) ConvoyList(context.Context) ([]ConvoyDetail, error) { return nil, ErrUnsupported }

func (*GCDriver) ConvoyStatus(context.Context, string) (*ConvoyDetail, error) {
	return nil, ErrUnsupported
}

func (*GCDriver) ConvoyCreate(context.Context, string, []string) (string, error) {
	return "", ErrUnsupported
}

func (*GCDriver) ConvoyCreateFromEpic(context.Context, string, string) (string, error) {
	return "", ErrUnsupported
}

func (*GCDriver) ConvoyClose(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) ConvoyLand(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) ConvoyWatch(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) ConvoyUnwatch(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) MoleculeDAG(context.Context, string) (*DAGInfo, error) { return nil, ErrUnsupported }

func (*GCDriver) MoleculeProgress(context.Context, string) (*MoleculeProgress, error) {
	return nil, ErrUnsupported
}

func (*GCDriver) MoleculeStepDone(context.Context, string) (*StepDoneResult, error) {
	return nil, ErrUnsupported
}

func (*GCDriver) Vitals(context.Context) (*Vitals, error) { return nil, ErrUnsupported }

func (*GCDriver) Costs(context.Context) (*CostsOutput, error) { return nil, ErrUnsupported }

func (*GCDriver) PatrolScan(context.Context) (*PatrolScanResult, error) { return nil, ErrUnsupported }
