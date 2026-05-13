# Agent Integration

Press `a` on any selected issue to launch an AI agent session pre-loaded with the full issue context: title, description, notes, acceptance criteria, and dependency status.

Mardi Gras supports multiple agent runtimes:

- **[Claude Code](https://claude.com/claude-code)** (default) — detected via `claude` on PATH
- **[Cursor](https://cursor.com)** (fallback) — detected via `cursor-agent` on PATH, launched with `-f -p` flags
- **[OpenAI Codex](https://github.com/openai/codex)** (fallback) — detected via `codex` on PATH, launched with `--sandbox workspace-write -a on-request -C <projectDir>` (plus `--no-alt-screen` inside tmux)

## Choosing a runtime

By default, mg prefers `claude`, falls back to `cursor-agent`, then `codex`. Override with the `--agent` flag or the `MG_AGENT_RUNTIME` env var:

```bash
mg --agent codex                  # use codex for this session
MG_AGENT_RUNTIME=cursor mg        # same shape via env var
MG_AGENT_RUNTIME=claude mg        # force claude even if you have other tools installed
```

Accepted values are `claude`, `cursor` (or `cursor-agent`), and `codex`. The override is honored only if the matching binary is on PATH — if you request a runtime that isn't installed, mg falls back to the default detection order rather than failing silently. Unknown values are ignored.

The override applies only to mg's local launch path. When Gas Town is available, the `a` key dispatches through `gt sling` and the runtime is chosen by the Gas Town formula (see [Gas Town docs](https://github.com/steveyegge/gastown)). Gas Town v1.1.0+ has first-class codex support via `gt sling --agent codex`; configure the default in your formula or pass it manually until mg's gt-sling integration follows.

## Codex specifics

Codex is built on stricter defaults than Claude or Cursor — it requires a sandbox policy and an approval policy or it blocks on permission prompts. mg launches codex with `--sandbox workspace-write -a on-request` so unattended tmux agents can edit files and run tests without interactive blocks. Power users who want a different posture (e.g. `-a never` for fully autonomous runs) can wire it through a codex **profile** (`~/.codex/config.toml` under `[profiles.<name>]`) and shell-alias `codex` to `codex -p <name>`.

A few practical gotchas:

- **First-run auth**: run `codex login` once before mg dispatches a codex agent — codex authenticates with your ChatGPT account and stores credentials in `~/.codex/auth.json`. If you launch mg without logging in first, the agent will exit asking for setup.
- **Project trust**: codex prompts to trust a directory the first time it sees one. For unattended tmux dispatch, either run codex interactively in the project once (it'll remember in `[projects."<path>"]`), or pre-trust by editing `~/.codex/config.toml`.
- **nvm-installed codex**: if you installed codex via `npm install -g @openai/codex` under nvm, mg inherits PATH from the shell that launches it — make sure the right Node version is active (e.g. `nvm use 22`) before starting mg, or codex won't resolve. Separately, openai/codex#20906 documents a sandbox-PATH bug specific to nvm installs that can be worked around with `--add-dir $NVM_PATH`; consider a standalone or Homebrew install if you hit it.
- **AGENTS.md**: codex automatically reads `AGENTS.md` from the project root (and merges with `~/.codex/AGENTS.md`). Keep build/test/convention guidance in `AGENTS.md` — mg's prompt focuses on the specific Beads issue and lets codex pick up project context from the file.
- **Beads integration on Homebrew bd v1.0.4**: bd's `codex-hook` subcommand (which injects Beads context into codex sessions) shipped on bd main but is missing from the v1.0.4 release (steveyegge/beads#3924). Until v1.0.5 cuts, codex with bd-installed hooks will log "unknown command 'codex-hook'" once per prompt. This is cosmetic for the agent's work but worth knowing.

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

- Requires `claude`, `cursor-agent`, or `codex` on your `PATH`
- The command palette dynamically shows the detected runtime name (e.g., "Start Claude Code agent", "Start Cursor agent", or "Start Codex agent")
- If no agent runtime is found, the `a` key silently does nothing
- Tmux dispatch requires both the `TMUX` env var and `tmux` binary on PATH
- The prompt includes `bd update` and `bd close` hints so the agent knows how to manage the issue lifecycle
