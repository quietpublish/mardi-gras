# Gas City Integration

[Gas City](https://github.com/gastownhall/gascity) (`gc`) is a pack-based rewrite of Gas Town that exposes a typed **Supervisor HTTP API** instead of a CLI. Mardi Gras can drive Gas City through that API as an alternative to Gas Town.

> **Status: opt-in.** The Gas City backend powers the agent roster, mail, formulas, nudge, decommission, agent dispatch (sling), and convoys. A meaningful set of operations — comments, unsling, cascade close, crew assign, molecule DAG, convoy land/watch/unwatch, vitals/costs/patrol, rig recovery, handoff, and the activity feed — has no Gas City equivalent yet. See [What works today](#what-works-today) for the exact matrix.

## How it works

Mardi Gras talks to an orchestrator through a single `Driver` seam (`internal/gastown`). The default driver shells out to the Gas Town `gt` CLI exactly as before. When you opt in, a second driver speaks the Gas City Supervisor HTTP API over `net/http` instead. mg picks one driver at startup; nothing changes for existing Gas Town users.

## Enabling it

Gas City is **opt-in via the `MG_GC_API` environment variable** — without it, mg uses the Gas Town driver. You need a running Gas City supervisor (`gc start` against an initialized city).

The control surface activates whenever an orchestrator is reachable — Gas Town (`gt` on PATH) **or** a configured Gas City supervisor — so `MG_GC_API` lights up the panel, roster, and actions even on a box with **no `gt` installed at all**.

```bash
# Discover the supervisor automatically (recommended).
# Reads the live API address from ~/.gc/supervisor.log.
MG_GC_API=auto mg

# Or point at a specific supervisor explicitly.
MG_GC_API=http://127.0.0.1:8372 mg

# Optionally pin a city; otherwise mg uses the first running city.
MG_GC_API=auto MG_GC_CITY=mycity mg
```

The supervisor binds a **dynamically assigned** TCP port (not a fixed one), logged as `Supervisor API listening on http://host:port`. `MG_GC_API=auto` reads that line so you don't have to track the port yourself. The `~/.gc/supervisor.sock` control socket is a separate protocol and does not serve the HTTP API.

## What works today

| Capability | Gas City | Notes |
|---|---|---|
| Live agent roster (`ctrl+g`) | ✅ | `GET /v0/city/{city}/agents`; role inferred from the agent pool |
| Mail — inbox, read, reply, send, archive, mark-read | ✅ | mutations send the required `X-GC-Request` header |
| Formula listing | ✅ | scoped to the city |
| Nudge (`n`) / decommission (`K`) | ✅ | resolves the roster agent to a live session, then submits a message / kills the session |
| Agent dispatch (sling, `a`) | ✅ | Gas City requires an explicit target agent (unlike gt's auto-pick), so `a` prompts for a target before slinging |
| Convoys — list / create (`C`) / close | ✅ | Gas City models a convoy as a bead |
| Issue comments in the detail panel | ⛔ | `bd comments` is driven through the Gas Town driver today |
| Unsling (`shift+A`) | ⛔ | no Gas City endpoint; the action reports "not supported" |
| Cascade close | ⛔ | no Gas City endpoint |
| Create & assign to crew (`Y`) | ⛔ | no Gas City endpoint; the form submits and fails |
| Convoy `land` / `watch` / `unwatch` / create-from-epic | ⛔ | no Gas City endpoint |
| Molecule DAG, progress, step-done | ⛔ | the Detail panel's DAG section stays empty |
| Vitals / costs / patrol | ⛔ | no Gas City equivalent; these panels stay empty |
| Rig recovery (`Recover dead rigs`) | ⛔ | shells out to `gt release`/`gt sling`; hidden from the palette on Gas City |
| Handoff (`h`) | ⛔ | shells out to `gt handoff`; reports "Handoff is a Gas Town feature" |
| Recent activity feed | ⛔ | reads `~/gt/.events.jsonl` off local disk, which Gas City does not write |

Unsupported operations either hide themselves (recovery is dropped from the
command palette) or return a clear "not supported" message — none of them fail
with a raw `exec: "gt": executable not found`. For anything in the ⛔ rows, run
mg against Gas Town (`gt`) instead.

The three gt-shaped rows at the bottom — recovery, handoff, and the activity
feed — are not Gas City limitations so much as mg ones: they bypass the `Driver`
seam and call `gt` (or read its files) directly. They are declared as
`FeatureRecovery`, `FeatureHandoff`, and `FeatureActivityFeed` so the UI can
gate them, and porting them to the supervisor API would close the gap. Gas
City's events API is the natural replacement for the activity feed.

## Trying it without a real `gc` (demos / screenshots)

`make dev-gc` runs mg against a fake Gas City supervisor (`testdata/fakegc`) — the
HTTP analogue of `make dev-gt`. It serves a rich canned roster (agents across
roles/states/models), mail, formulas, and convoys, so you can explore the panel
(`ctrl+g`) and capture screenshots without installing `gc` or standing up a
city. No dolt, no supervisor, fully deterministic.

```bash
make dev-gc          # builds the fake supervisor + mg, wires MG_GC_API, launches the TUI
```

## Regenerating the API client

The Gas City client (`internal/gastown/gcclient`) is generated by [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) from a pinned copy of the Supervisor OpenAPI spec. To bump it to a newer Gas City release, drop the new `openapi.json` in place and run:

```bash
make gc-client
```

The spec is OpenAPI 3.1; `downgrade.jq` rewrites it to 3.0 first (oapi-codegen does not yet fully support 3.1). Generation is scoped to the endpoints mg uses.

The committed spec is currently **Gas City v1.4.1** (127 paths). Fetch a newer one with:

```bash
gh api "repos/gastownhall/gascity/contents/docs/reference/schema/openapi.json?ref=<tag>" \
  -H "Accept: application/vnd.github.raw" > internal/gastown/gcclient/openapi.json
```

The raw media type matters — the spec is over 1 MB, and the contents API omits inline `content` above that size.

**A path-level diff is not enough to call a bump safe.** Going from the June spec to v1.4.1 added 16 paths and removed none, yet still broke the build: several operations moved from a single `default` error response to explicit status codes, so oapi-codegen stopped emitting `ApplicationproblemJSONDefault` for them. `gcRespErr`/`gcMutationErr` now decode the raw response `Body` instead of a generated typed field, which is stable across regenerations — but after any bump, run `go build ./...` and the `internal/gastown` tests before assuming the contract held.
