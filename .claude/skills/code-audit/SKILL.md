---
name: code-audit
description: Run a systematic code reliability audit on the mardi-gras codebase. Identifies issues ranked by severity, produces a findings document, and optionally creates a fix plan.
disable-model-invocation: true
argument-hint: "[focus area, e.g. 'gastown', 'data', 'app', or blank for full]"
---

# Code Audit

Run a systematic reliability and quality audit. Produces a findings document with severity-ranked issues and an implementation plan.

## Scope

If `$ARGUMENTS` specifies a focus area (e.g., `gastown`, `data`, `app`), limit the audit to that package. Otherwise, audit the full codebase.

## Step 1: Automated checks

Run all automated quality tools first:

```bash
make test                    # Test suite
go vet ./...                 # Static analysis
go build ./...               # Compilation check
```

If `golangci-lint` is available: `make lint`

Note any failures — these are automatic high-severity findings.

## Step 2: Systematic code review

Review each file in scope for the following categories:

### Resource management (High severity)
- Unbounded goroutines or process spawning
- Missing timeouts on `exec.Command` calls
- File handles not closed
- Channels that can deadlock
- Race conditions (shared state without sync)

### Error handling (Medium severity)
- Errors silently swallowed (especially in goroutines)
- Panics in production paths
- Missing nil checks on pointers from external data
- Silent fallbacks that hide real problems

### Performance (Medium severity)
- O(n) operations that could be O(1) with maps/caches
- Unbounded memory growth (slices that only grow)
- Redundant work (repeated parsing, duplicate fetches)
- Hot-path allocations

### Correctness (varies)
- Value receiver mutations (Go gotcha: mutations lost)
- Stale closures capturing loop variables
- Off-by-one in slice operations
- Missing edge cases in switch statements

### Package boundaries (Low severity)
- Circular or unexpected dependencies
- Docs claiming "no dependencies" when imports exist
- Exported symbols that should be internal

## Step 3: Classify findings

Rate each finding:

| Severity | Criteria |
|----------|----------|
| **Critical** | Data loss, crash, or security vulnerability |
| **High** | Resource leak, process pile-up, or silent data corruption |
| **Medium** | Degraded UX, hidden errors, or O(n) where O(1) is easy |
| **Low** | Style, docs drift, or minor inefficiency |

## Step 4: Create findings document

Write to `docs/internal/code-audit-<date>.md`:

```markdown
# Code Audit — <date>

## Summary
<count> findings: <n> critical, <n> high, <n> medium, <n> low

## Findings

### 1. <Title> (Severity)

**File**: `<path>:<line>`
**Category**: <resource mgmt | error handling | performance | correctness | boundaries>

**Problem**: <what's wrong and why it matters>

**Evidence**: <code snippet or reproduction steps>

**Suggested fix**: <concrete approach, not vague "improve this">

---
```

## Step 5: Create fix plan (if requested)

If the user wants to proceed with fixes, create a prioritized implementation plan:

1. Order by severity (critical first), then by blast radius
2. Group related fixes that can share a commit
3. Note which fixes require tests vs. which are test-only
4. Estimate each fix: trivial (< 5 min), small (< 30 min), medium (< 2 hours)

## Previous audits

Check `docs/internal/` for prior audit docs to avoid rediscovering known issues. Reference resolved findings and their commit hashes.

## Key areas to watch

Based on project history, these areas tend to accumulate issues:
- `internal/app/app.go` — Large file, many message handlers, BubbleTea complexity
- `internal/gastown/*.go` — External process calls (`gt`, `bd`) with timeout sensitivity
- `internal/data/source.go` — Data loading with fallback logic
- `internal/views/gastown.go` — Complex rendering with nil-safety requirements
