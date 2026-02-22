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
| `internal/views` | Parade list + Detail panel views |
| `internal/components` | Header, footer, help overlay, divider |
| `internal/ui` | Theme colors, styles, symbols — no logic |
| `internal/data` | JSONL loading, issue types, filtering, file watcher |
| `internal/agent` | Claude Code launch, tmux pane dispatch |
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
bd sync --from-main                   # Pull beads data from main
```

Do NOT use `bd edit` — it opens `$EDITOR` and blocks agents.

## Agent Dispatch

When running in tmux, `mg` launches Claude agents in split panes via `tmux split-window`. Agents are tagged with `@mg_agent` pane options for tracking. The `--teammate-mode tmux` flag enables Claude Code's native agent teams.

## Git

- Create feature branches off `main` — the `main` branch is protected.
- Commit messages: imperative mood, describe the "why". Prefix with `feat:`, `fix:`, `docs:`, `chore:`, `test:` as appropriate.
- Do not push unless explicitly asked.
