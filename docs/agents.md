# Agent Integration

Press `a` on any selected issue to launch an AI agent session pre-loaded with the full issue context: title, description, notes, acceptance criteria, and dependency status.

Mardi Gras supports multiple agent runtimes:

- **[Claude Code](https://claude.com/claude-code)** (default) — detected via `claude` on PATH
- **[Cursor](https://cursor.com)** (fallback) — detected via `cursor-agent` on PATH, launched with `-f -p` flags

## Choosing a runtime

By default, mg prefers `claude` and falls back to `cursor-agent` when claude isn't installed. Override the default with the `--agent` flag or the `MG_AGENT_RUNTIME` env var:

```bash
mg --agent cursor                 # use cursor-agent for this session
MG_AGENT_RUNTIME=cursor mg        # same, via env var
MG_AGENT_RUNTIME=claude mg        # force claude even if you have other tools installed
```

Accepted values are `claude` and `cursor` (or `cursor-agent`). The override is honored only if the matching binary is on PATH — if you request a runtime that isn't installed, mg falls back to the default detection order rather than failing silently. Unknown values are ignored.

The override applies only to mg's local launch path. When Gas Town is available, the `a` key dispatches through `gt sling` and the runtime is chosen by the Gas Town formula (see [Gas Town docs](https://github.com/steveyegge/gastown)).

## Tmux-native dispatch (multi-agent)

When running inside tmux, agents launch in **new tmux windows** instead of suspending the TUI. This means:

- The parade stays visible while agents work
- Multiple agents can run simultaneously on different issues
- Active agents show a `⚡` badge next to their issue in the parade
- The header displays the total active agent count
- Press `a` on an issue with an active agent to **switch** to its tmux window
- Press `A` to **kill** the active agent on the selected issue
- Agent status is polled automatically alongside the file watcher

## Fallback (non-tmux)

Outside tmux, the TUI suspends while the agent runs (using BubbleTea's `tea.ExecProcess`), giving the agent the full terminal. When you exit the session, Mardi Gras resumes and reloads data to pick up any changes.

## Requirements

- Requires `claude` or `cursor-agent` on your `PATH`
- The command palette dynamically shows the detected runtime name (e.g., "Start Claude Code agent" or "Start Cursor agent")
- If no agent runtime is found, the `a` key silently does nothing
- Tmux dispatch requires both the `TMUX` env var and `tmux` binary on PATH
- The prompt includes `bd update` and `bd close` hints so the agent knows how to manage the issue lifecycle
