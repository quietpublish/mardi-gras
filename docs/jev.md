# Jev (optional judge)

Mardi Gras can ask [Jev](https://docs.typesafe.ai), TypeSafe AI's System One model, typed questions about your issues. Jev does not generate text: given a JSON state and a question, it returns a yes/no probability, a pick from named options, or a point on a scale, each with a calibrated confidence, in well under a second. mg uses it as a fast judge in places that are otherwise hand-written heuristics.

**Nothing is sent unless you opt in.** Jev is off until `MG_JEV_API_KEY` is set, and every feature built on it renders exactly as it does today when Jev is off, slow, or wrong.

## What it does today

**Duplicate check on create.** When you submit the new-issue form (`N`), mg picks up to eight existing issues whose titles overlap the new one (open, in progress, or closed in the last two weeks), asks Jev whether each is the same piece of work, and only then runs `bd create`. While it asks, the toast reads `Checking for duplicates…`; the check has a 1.5 s budget and the issue is created as usual if Jev is slow or down.

- A strong match (80% or more, with confidence) opens a dialog listing the matches with their probabilities: `enter` abandons the new issue and selects the existing one, `c` creates it anyway, `l` creates it and marks it a duplicate of the selected match (`bd dep add --type=duplicates`), `esc` abandons it.
- A weaker match (50% to 80%) does not interrupt: the issue is created and the toast reads `… → created · similar to mg-002 (64%)`.
- Below that, or when no existing title shares a word with the new one, nothing changes and Jev is not asked.

What is sent: the new title, type and priority, and the redacted snapshots of the candidates (see below). Nothing is sent when no title overlaps.

More features will follow on the same plumbing; the design notes list formula choice and a Jev-ranked focus mode next.

## Enabling it

```bash
export MG_JEV_API_KEY=...      # your TypeSafe key — this is the on switch
mg
```

The footer shows a `jev` chip when Jev is on. At startup mg sends one trivial probe so a rejected key or a wrong URL is reported at once: a `401`/`403`/`404` disables Jev for the session with a toast and the chip reads `jev off`.

| Variable | Meaning | Default |
|---|---|---|
| `MG_JEV_API_KEY` | Bearer token. Set it to enable Jev. | unset: Jev off |
| `MG_JEV_URL` | An API-compatible server (OpenJev, Von, tensai). A host alone gets `/v1/systemone` appended. | `https://api.typesafe.ai/v1/systemone` |
| `MG_JEV_MODEL` | Model route. | `jev-latest` |
| `MG_JEV=off` | Disable for this run even with a key. The `--no-jev` flag sets it. | unset |
| `MG_JEV_SCOPE` | `minimal` sends title and structure only; `standard` adds the start of the description. | `standard` |

`--cmd-timeout` scales Jev's request timeout along with every other external call.

## What leaves the machine

Only a redacted snapshot of each open issue, built by one function (`data.SnapshotForJudge`) so the field list is reviewed in one place:

- **Sent**: id, title, type, status, priority, labels, ages in whole days (created, last updated, due, deferred), counts of blockers and comments, and at `standard` scope the first 600 characters of the description with code blocks replaced by `[code]`.
- **Never sent**: owner, assignee, creator (usernames and emails), notes, design, acceptance criteria, close reason, metadata, dependency IDs, raw timestamps, the project path, or anything about your git remote or machine.
- Token-shaped strings in titles and descriptions (`sk-…`, `ghp_…`, `AKIA…`, bearer tokens, long hex or base64 runs, `password=…`) are replaced with `[redacted]` before sending.

Agent pane output, orchestrator mail bodies, costs and vitals are not in any payload. For a private repo or a Gas Town rig, point `MG_JEV_URL` at a self-hosted server: the wire format is the same.

## Cost and caching

Jev charges about $0.042 per million input tokens; output is free. A snapshot is roughly 250 tokens, so judging a 200-issue backlog once costs about a fifth of a cent. mg caches every verdict by a hash of the snapshot it answered and asks again only when that hash changes: a label added, a priority moved, a description edited, or a day passing. Re-asking on every reload would cost dollars a day, so the loop never bypasses the cache.

## When Jev misbehaves

Failures feed a circuit breaker. After three consecutive failures the chip reads `jev degraded`; after five it reads `jev paused` and mg stops asking for 30 seconds, doubling up to five minutes, then sends one probe request before resuming. A rejected key or a missing endpoint disables Jev for the session. Only the first failure and the pause raise a toast; successes are silent.

## Developing against a fake

```bash
make dev-jev                                   # mg against testdata/fakejev, log in /tmp/mg-fakejev.log
FAKEJEV_FLAGS="-status 401" make dev-jev        # watch Jev disable itself
FAKEJEV_FLAGS="-fail-every 2" make dev-jev      # trip the circuit breaker
make contract-jev KEY=$TYPESAFE_API_KEY         # the real endpoint, once, from a dev machine
```

The fake answers deterministically and logs the shape of every request, which is the quickest way to check what a new question set sends.
