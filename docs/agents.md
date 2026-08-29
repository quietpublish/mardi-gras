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

The override applies only to mg's local launch path. When an orchestrator is available, the `a` key dispatches through it (`gt sling`, or the Gas City sling endpoint) and the runtime is chosen by the formula (see [Gas Town docs](https://github.com/gastownhall/gastown)). Gas Town v1.1.0+ has first-class codex support via `gt sling --agent codex`, and mg propagates that automatically — see [Gas Town routing for Codex](#gas-town-routing-for-codex) below.

## Codex specifics

Codex is built on stricter defaults than Claude or Cursor — it requires a sandbox policy and an approval policy or it blocks on permission prompts. mg launches codex with `--sandbox workspace-write -a on-request` so unattended tmux agents can edit files and run tests without interactive blocks. Power users who want a different posture (e.g. `-a never` for fully autonomous runs) can wire it through a codex **profile** (`~/.codex/config.toml` under `[profiles.<name>]`) and shell-alias `codex` to `codex -p <name>`.

A few practical gotchas:

- **First-run auth**: run `codex login` once before mg dispatches a codex agent — codex authenticates with your ChatGPT account and stores credentials in `~/.codex/auth.json`. If you launch mg without logging in first, the agent will exit asking for setup.
- **Project trust**: codex prompts to trust a directory the first time it sees one. For unattended tmux dispatch, either run codex interactively in the project once (it'll remember in `[projects."<path>"]`), or pre-trust by editing `~/.codex/config.toml`.
- **nvm-installed codex**: if you installed codex via `npm install -g @openai/codex` under nvm, mg inherits PATH from the shell that launches it — make sure the right Node version is active (e.g. `nvm use 22`) before starting mg, or codex won't resolve. Separately, openai/codex#20906 documents a sandbox-PATH bug specific to nvm installs that can be worked around with `--add-dir $NVM_PATH`; consider a standalone or Homebrew install if you hit it.
- **AGENTS.md**: codex automatically reads `AGENTS.md` from the project root (and merges with `~/.codex/AGENTS.md`). Keep build/test/convention guidance in `AGENTS.md` — mg's prompt focuses on the specific Beads issue and lets codex pick up project context from the file.
- **Beads integration on bd v1.0.4**: bd's `codex-hook` subcommand (which injects Beads context into codex sessions) shipped on bd main but was missing from the v1.0.4 release ([gastownhall/beads#3924](https://github.com/gastownhall/beads/issues/3924)), so codex with bd-installed hooks logs "unknown command 'codex-hook'" once per prompt. Cosmetic for the agent's work, and only relevant if you are pinned to that old release — bd has since moved on to the v1.2.x line.

### In-app transcript (`M`)

`M` on a selected issue opens a live Codex transcript in place of the detail pane. Unlike `a`, this path does not use tmux at all — mg spawns `codex mcp-server`, performs the MCP handshake, and streams the session's events (agent messages, exec commands, tool calls, patches, errors) straight into the panel. It needs `codex` on `PATH`; without it the launch reports the runtime as unavailable.

Because a human is watching, `M` launches with approval policy `on-request` (the tmux and orchestrator paths use `never`), so exec and apply-patch approvals surface as a modal inside mg — `j`/`k` to choose, `enter` to confirm. Press `r` with the transcript open to send a follow-up prompt into the running session, and `M` again to close it.

### Resuming a prior Codex session

When codex is the active runtime inside tmux, the command palette (`:` or `Ctrl+K`) shows a **"Resume last Codex session"** action. It launches `codex resume --last` in a new tmux split rooted at the project directory. The action is gated on a rollout file actually existing under `~/.codex/sessions/YYYY/MM/DD/*.jsonl` so a never-launched codex doesn't surface as a confusing empty pane.

### Gas Town routing for Codex

When Gas Town is on PATH and `MG_AGENT_RUNTIME=codex` (or `--agent codex`) is active, mg's sling commands pass `--agent codex` to `gt sling`, so the agent preference propagates from mg into Gas Town. Requires gt v1.1.0+ (earlier versions reject the `--agent` flag). For `claude` / `cursor-agent`, mg continues to let gt pick its default agent — the v0.19.0 behavior is unchanged.

The Gas City backend does not carry the override: its sling endpoint is addressed by target agent rather than by runtime, so mg prompts for a target (`sling to>`) and leaves the runtime to the city.

## Tmux-native dispatch (multi-agent)

When running inside tmux, agents launch in **new tmux panes** instead of suspending the TUI. mg runs `tmux split-window -h -l 60% -d`, so the agent gets a 60%-wide pane to the right, focus stays on the parade, and each pane is tagged with the `@mg_agent` pane option (set to `mg-<issueID>`) so mg can find, focus, capture, and kill it later. This means:

- The parade stays visible while agents work
- Multiple agents can run simultaneously on different issues
- Active agents show a `⚡` badge next to their issue in the parade
- The header displays the total active agent count
- Press `a` on an issue with an active agent to **switch** to its tmux pane
- Press `A` to **stop** the active agent on the selected issue — when an orchestrator is present this asks it to unsling; only without one does mg kill the pane directly
- The detail panel tails the last 15 lines of the agent's pane (ANSI stripped) under **AGENT OUTPUT**
- Agent status is polled automatically alongside the file watcher

Claude Code is launched with `--teammate-mode tmux` on this path so it participates in Claude Code's native agent teams. Cursor and Codex get their own flag sets (see above).

## Fallback (non-tmux)

Outside tmux, the TUI suspends while the agent runs (using BubbleTea's `tea.ExecProcess`), giving the agent the full terminal. When you exit the session, Mardi Gras resumes and reloads data to pick up any changes.

## Requirements

- A local runtime — `claude`, `cursor-agent`, or `codex` on your `PATH` — is required only for **direct launch** (the tmux pane, or the suspend-and-run path). Orchestrator dispatch does not need one: `gt sling` and Gas City both start the agent themselves.
- The command palette's **Launch agent** entry names the detected runtime in its description ("Start Claude Code agent on issue", "Start Cursor agent on issue", or "Start Codex agent on issue")
- With no local runtime installed, `a` still dispatches through an orchestrator — single issue, multi-selection, and Gas City alike. It is only **direct launch** that goes quiet, since that path needs a binary for mg to exec.
- Tmux dispatch requires both the `TMUX` env var and `tmux` binary on PATH
- The prompt includes `bd update` and `bd close` hints so the agent knows how to manage the issue lifecycle
