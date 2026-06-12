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

// --- unsupported operations (Phase 3+) -------------------------------------

func (*GCDriver) Formulas(context.Context) ([]string, error) { return nil, ErrUnsupported }

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

func (*GCDriver) MailInbox(context.Context, bool) ([]MailMessage, error) { return nil, ErrUnsupported }

func (*GCDriver) MailRead(context.Context, string) (*MailMessage, error) { return nil, ErrUnsupported }

func (*GCDriver) MailReply(context.Context, string, string) error { return ErrUnsupported }

func (*GCDriver) MailSend(context.Context, string, string, string) error { return ErrUnsupported }

func (*GCDriver) MailArchive(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) MailMarkRead(context.Context, string) error { return ErrUnsupported }

func (*GCDriver) MailMarkAllRead(context.Context) error { return ErrUnsupported }

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
