# Mardi Gras — Project Instructions

Mardi Gras (`mg`) is a BubbleTea TUI for Beads issues. It reads `.beads/issues.jsonl` directly — no daemon, no config file.

## Build & Test

```bash
make build        # Build binary → ./mg
make test         # go test ./...
make lint         # golangci-lint run ./...
make fmt          # go fmt ./...
make dev          # Build and run with testdata/sample.jsonl
```

Always run `make test` after changes. Run `make lint` before committing.

## Project Layout

| Package | Purpose |
|---------|---------|
| `cmd/mg` | Entry point, flag parsing |
| `internal/app` | Root BubbleTea model, key handlers, message routing |
| `internal/views` | Parade list, Detail panel, Gas Town panel views |
| `internal/components` | Header, footer, help overlay, divider |
| `internal/ui` | Theme colors, styles, symbols — no logic |
| `internal/data` | JSONL loading, issue types, filtering, file watcher |
| `internal/agent` | Claude Code launch, tmux pane dispatch |
| `internal/gastown` | Gas Town integration: env detection, status parsing, sling/nudge commands |
| `internal/tmux` | `mg --status` widget for tmux status bar |

## Conventions

- **Go style**: `gofmt` formatting, no lint warnings. Run `golangci-lint` before committing.
- **Value receivers** on BubbleTea models (`Update`, `View`), **pointer receivers** on mutating helpers (`layout`, `rebuildParade`, `syncSelection`).
- **UI constants** live in `internal/ui/` — colors in `theme.go`, symbols in `symbols.go`, lipgloss styles in `styles.go`. Don't scatter raw colors or symbols in view code.
- **No new packages** without good reason. Prefer expanding existing packages.
- **Test naming**: `TestFunctionName` for the happy path, `TestFunctionNameEdgeCase` for variants.

## Beads Workflow

This project uses [Beads](https://github.com/beads-project/beads) for issue tracking.

```bash
bd ready                              # Find unblocked work
bd update <id> --status=in_progress   # Claim an issue
bd close <id>                         # Mark done
bd sync                               # Sync beads data
```

Do NOT use `bd edit` — it opens `$EDITOR` and blocks agents.

## Gas Town Integration

Mardi Gras integrates with [Gas Town](https://github.com/steveyegge/gastown) (`gt`) for multi-agent orchestration. The `internal/gastown` package handles:

- **Environment detection** (`detect.go`): Reads `GT_ROLE`, `GT_RIG`, `GT_SCOPE`, `GT_POLECAT`, `GT_CREW` env vars and checks if `gt` is on PATH.
- **Status parsing** (`status.go`): Parses `gt status --json` output. The raw JSON nests agents under `rigs[].agents`; `normalizeStatus()` flattens them into a single `Agents` slice for the UI.
- **Sling/Nudge** (`sling.go`): Issue dispatch to polecats, formula selection, multi-sling, nudge messaging.

**Key gotcha**: `gt status --json` takes ~9 seconds to run. Background polling via BubbleTea Cmds may not return before the user interacts. The Gas Town panel (`ctrl+g`) triggers an on-demand fetch if status is nil and shows a loading state while waiting.

**Testing with real gt**: Run mg from a Gas Town workspace (e.g., `cd ~/gt/<rig>/crew/<name> && ~/Work/mardi-gras/mg`). The `gt` source code is at `~/go/pkg/mod/github.com/steveyegge/gastown@v0.7.0/` — check it directly rather than guessing struct shapes.

## Agent Dispatch

When running in tmux, `mg` launches Claude agents in split panes via `tmux split-window`. Agents are tagged with `@mg_agent` pane options for tracking. The `--teammate-mode tmux` flag enables Claude Code's native agent teams.

## Git

- Create feature branches off `main` — the `main` branch is protected.
- Commit messages: imperative mood, describe the "why". Prefix with `feat:`, `fix:`, `docs:`, `chore:`, `test:` as appropriate.
- Do not push unless explicitly asked.
