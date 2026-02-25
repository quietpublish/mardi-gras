---
name: upstream-triage
description: Scan open issues on the beads GitHub repo, identify ones we can help answer based on our experience, and draft responses.
disable-model-invocation: true
argument-hint: "[limit, default 30]"
---

# Upstream Triage

Scan open issues on `steveyegge/beads` and identify ones where our experience with mardi-gras can provide helpful answers.

## Step 1: Fetch open issues

```bash
gh issue list --repo steveyegge/beads --state open --limit <limit> --json number,title,comments,createdAt --jq '.[] | "\(.number)\t\(.comments | length)\t\(.title)"'
```

## Step 2: Categorize issues

Read the body and comments of each issue. Classify into:

### Can help (we have direct experience)
Issues where our work on mardi-gras gives us firsthand knowledge:
- Dolt setup and migration (we've done this)
- `bd` CLI behavior and workarounds
- Gas Town integration patterns
- Multi-agent workflows
- JSONL format and parsing

### Might help (we have related context)
Issues where we have partial knowledge from upstream research:
- Features on `main` not yet released (we track these in memory)
- Known bugs with workarounds we've found
- Configuration patterns we've tested

### Can't help (outside our experience)
- Platform-specific issues we haven't hit (Windows, specific Linux distros)
- Features we don't use (Linear sync, specific backends)
- Internal beads architecture questions

## Step 3: For each "can help" issue

1. Read the full issue body and all comments
2. Check if someone already answered adequately
3. If not, draft a response that:
   - Starts with what we know from direct experience
   - Cites specific versions tested (e.g., "on beads v0.56.1")
   - Mentions relevant fixes on `main` if applicable (with commit hashes from our memory)
   - Provides concrete workarounds, not just "wait for next release"
   - Stays factual — don't speculate about upstream intentions

## Step 4: Present findings

Show the user a summary table:

```
| # | Title | Comments | Category | Our knowledge |
|---|-------|----------|----------|---------------|
```

Then for each "can help" issue, show the drafted response and ask whether to post it.

## Important

- Do NOT post comments without explicit user approval
- Use `gh issue comment --repo steveyegge/beads <number> --body "..."` only after the user confirms
- Our GitHub identity is visible — keep responses professional and helpful
- Reference our project (mardi-gras) only when directly relevant to the answer

## Reference: our tested configurations

Check MEMORY.md for current versions and known issues. Key facts:
- Beads v0.56.1 installed locally (GitHub release binary)
- Gas Town v0.8.0
- Dolt server mode on port 3307
- `bd sync` is a no-op on v0.56+
- `bd init --from-jsonl` fixed on main (fd21fa4) but not in v0.56.1
- Self-managing Dolt server on main (893f6fc) but not in v0.56.1
- `bd edit` opens $EDITOR — never use from agents
