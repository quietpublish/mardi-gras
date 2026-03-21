# Agent Integration

Press `a` on any selected issue to launch an AI agent session pre-loaded with the full issue context: title, description, notes, acceptance criteria, and dependency status.

Mardi Gras supports multiple agent runtimes:

- **[Claude Code](https://claude.com/claude-code)** (preferred) — detected via `claude` on PATH
- **[Cursor](https://cursor.com)** (fallback) — detected via `cursor-agent` on PATH, launched with `-f -p` flags

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
