# Gas City Integration

[Gas City](https://github.com/gastownhall/gascity) (`gc`) is a separate, actively developed orchestrator. Its own README calls it "an orchestration-builder SDK" that "extracts the reusable infrastructure from Gas Town into a configurable toolkit" — it is pack-based, and it exposes a typed **Supervisor HTTP API** rather than a CLI. Mardi Gras can drive Gas City through that API as an alternative to Gas Town.

> **Status: a supported backend — Gas Town is still the default.** The Gas City backend powers the agent roster, issue comments, mail, formulas, nudge, decommission, agent dispatch (sling), convoys (including create-from-epic), and crew assign. Still missing: unsling, cascade close, the molecule DAG, convoy land/watch/unwatch, vitals/costs/patrol, rig recovery, handoff, and the activity feed. See [What works today](#what-works-today) for the exact matrix.

## How it works

Mardi Gras talks to an orchestrator through a single `Driver` seam (`internal/gastown`). One driver shells out to the Gas Town `gt` CLI exactly as before; the other speaks the Gas City Supervisor HTTP API over `net/http`. mg picks exactly one at startup — see [Which driver mg picks](#which-driver-mg-picks) — and nothing changes for existing Gas Town users.

## Which driver mg picks

`SelectDriver()` (`internal/gastown/gc.go`) chooses the backend from evidence about the machine, in this order:

1. **`MG_GC_API` is set** → Gas City. Naming a supervisor explicitly always wins.
2. **Any Gas Town evidence** — a `GT_*` env var, or `gt` on PATH → Gas Town.
3. **No Gas Town evidence at all, but Gas City evidence** — `gc` on PATH, or a `city.toml` in an ancestor of the working directory → Gas City.
4. **Nothing conclusive** → Gas Town, the historical default.

Rule 3 is deliberately narrow: it fires only when there is *no* Gas Town evidence whatsoever, so nobody who has `gt` installed, or who is sitting inside a Gas Town session, gets a different backend than they did before. It exists because mg used to hand a Gas City user a Gas Town driver that shelled out to a `gt` which was not installed. That conservatism is the point — this is a correctness fix for machines mg was already getting wrong, **not** a change of the global default, which remains Gas Town.

If both orchestrators are on the box, rule 2 wins and you get Gas Town; `MG_GC_API` is how you override that.

## Enabling it

Set `MG_GC_API` to reach a running Gas City supervisor (`gc start` against an initialized city). That is rule 1 above — the explicit opt-in, and the only way to select Gas City on a machine that also has `gt`.

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
| Mail — inbox, read, reply, send, archive, mark-read, mark-all-read (`R`) | ✅ | mutations send the required `X-GC-Request` header |
| Formula listing | ✅ | scoped to the city (`scope_kind=city`, which the endpoint requires) |
| Nudge (`n`) / decommission (`K`) | ✅ | resolves the roster agent to a live session, then submits a message / kills the session |
| Agent dispatch (sling, `a`) | ✅ | Gas City requires an explicit target agent (unlike gt's auto-pick), so `a` prompts for a target before slinging |
| Convoys — list / expand (`enter`) / create (`C`) / close (`x`) | ✅ | Gas City models a convoy as a bead; the list endpoint carries no progress, so each convoy's detail is fetched to fill it in |
| Create & assign to crew | ✅ | `POST /v0/city/{city}/beads` takes the assignee inline, so the bead is never briefly unowned; `--nudge` wakes the crew member's session afterwards |
| Convoy create-from-epic | ✅ | no `--from-epic` flag upstream, so mg walks `GET …/beads/graph/{rootID}` and enrols the members, excluding the epic itself |
| Issue comments in the detail panel | ✅ | comments come from `bd comments` — a Beads call, backend-independent — so `GCDriver.Comments` delegates to the same `FetchComments` the gt driver uses |
| Unsling (`shift+A`) | ⛔ | `/sling` is POST-only; the action reports "not supported" |
| Cascade close | ⛔ | `…/bead/{id}/close` takes no cascade parameter |
| Convoy `land` (`l`) / `watch` (`w`) / `unwatch` (`W`) | ⛔ | no Gas City endpoint — `land` is a CLI-only composite, and watch/unwatch are gt-side subscriptions |
| Molecule DAG, progress, step-done | ⛔ | the Detail panel's DAG section stays empty |
| Vitals / costs / patrol | ⛔ | no Gas City equivalent; these panels stay empty |
| Rig recovery (`Recover dead rigs`) | ⛔ | shells out to `gt release`/`gt sling`; hidden from the palette on Gas City |
| Handoff (`h`) | ⛔ | shells out to `gt handoff`; reports "Handoff is a Gas Town feature" |
| Recent activity feed | ⛔ | reads `~/gt/.events.jsonl` off local disk, which Gas City does not write |
| Live status stream (SSE) | ⛔ | `FeatureSSE` exists on the `Feature` enum but **no driver implements it** — `Supports(FeatureSSE)` is `false` on Gas Town too. Both backends poll; this is not a Gas City gap |

Unsupported operations either hide themselves (recovery is dropped from the
command palette) or return a clear "not supported" message — none of them fail
with a raw `exec: "gt": executable not found`. For anything in the ⛔ rows —
except SSE, which no backend has — run mg against Gas Town (`gt`) instead.

The three gt-shaped rows near the bottom — recovery, handoff, and the activity
feed — are not Gas City limitations so much as mg ones: they bypass the `Driver`
seam and call `gt` (or read its files) directly. They are declared as
`FeatureRecovery`, `FeatureHandoff`, and `FeatureActivityFeed` so the UI can
gate them, and porting them to the supervisor API would close the gap. Gas
City's events API is the natural replacement for the activity feed.

Comments used to sit in the ⛔ block, and the reason given for it ("the
supervisor API has no comments endpoint") was itself the bug: comments never
came from the supervisor API. `Driver.Comments` shells out to `bd comments`,
which is a Beads call and answers identically on either backend, so returning
`ErrUnsupported` cost Gas City users a whole panel of data they already had.
It sits on `Driver` only because that is where the caller reaches for it.

## When the supervisor is unreachable

If the status fetch fails and there is no earlier status to fall back on, the
panel (`ctrl+g`) says so and names the remedy instead of spinning its loading
line forever:

```
⛽ Gas Town status unavailable

<the underlying error>

The Gas City supervisor did not answer. Check it is running, or point
mg at it with MG_GC_API=<url> (MG_GC_API=auto reads ~/.gc/supervisor.log).
Press ctrl+g again to retry.
```

The remedy is chosen from `Driver.Backend()`, because the two failure modes look
nothing alike — a missing `gt` binary versus a supervisor that is not listening.
(The heading keeps the "Gas Town" wording on both backends.)

Expect up to **~30 seconds** before the message appears on Gas City: that is the
driver's HTTP client timeout (`NewGCDriver`), and until it expires an unreachable
supervisor is genuinely indistinguishable from a slow one. Starting a fresh poll
clears the error and returns the panel to its spinner, so a transient failure
heals on the next retry, and a poll that already succeeded keeps its data — the
error only surfaces when there is nothing else to show.

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

The committed spec is currently **Gas City v1.4.1** (127 paths), and that is
the current pin: as of 2026-08-29 v1.4.1 is the latest Gas City release, and a
full contract diff against gascity `main` came back additive-only. **There is
nothing to bump today.** When a newer release does land, fetch its spec with:

```bash
gh api "repos/gastownhall/gascity/contents/docs/reference/schema/openapi.json?ref=<tag>" \
  -H "Accept: application/vnd.github.raw" > internal/gastown/gcclient/openapi.json
```

The raw media type matters — the spec is over 1 MB, and the contents API omits inline `content` above that size.

**A path-level diff is not enough to call a bump safe.** Going from the June spec to v1.4.1 added 16 paths and removed none, yet still broke the build: several operations moved from a single `default` error response to explicit status codes, so oapi-codegen stopped emitting `ApplicationproblemJSONDefault` for them. `gcRespErr`/`gcMutationErr` now decode the raw response `Body` instead of a generated typed field, which is stable across regenerations — but after any bump, run `go build ./...` and the `internal/gastown` tests before assuming the contract held.
