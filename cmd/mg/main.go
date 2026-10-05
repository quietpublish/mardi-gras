// Package main is the entry point for the Mardi Gras (mg) TUI. It parses
// command-line flags, resolves Beads data sources, and launches the
// BubbleTea program.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/app"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/jev"
	"github.com/matt-wright86/mardi-gras/internal/tmux"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Alias SourceMode constants for convenience.
const (
	SourceJSONL = data.SourceJSONL
	SourceCLI   = data.SourceCLI
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	path := flag.String("path", "", "Path to .beads/issues.jsonl file")
	blockTypesFlag := flag.String("block-types", "", "Comma-separated dependency types that count as blockers (default: blocks)")
	excludeTypesFlag := flag.String("exclude-type", "", "Comma-separated issue types to hide from the parade and status output")
	excludeLabelsFlag := flag.String("exclude-label", "", "Comma-separated labels to hide from the parade and status output")
	statusMode := flag.Bool("status", false, "Output tmux status line and exit")
	showVersion := flag.Bool("version", false, "Print version and exit")
	noAnimations := flag.Bool("no-animations", false, "Disable confetti and header shimmer animations")
	cmdTimeout := flag.Int("cmd-timeout", 0, "Command timeout in seconds (scales all external command timeouts; default 30)")
	agentRuntime := flag.String("agent", "", "Preferred agent runtime: claude, cursor, or codex (default: first on PATH — claude, then cursor, then codex)")
	agentCmd := flag.String("agent-cmd", "", "Single executable (no arguments) to launch instead of the agent binary; the runtime's flags are still passed, and mg will not start if it is not executable (default: MG_AGENT_CMD env, or the runtime binary)")
	themeFlag := flag.String("theme", "", "Color theme: auto, dark, or light (default: MG_THEME env or auto)")
	noJev := flag.Bool("no-jev", false, "Disable the Jev judge for this run even when MG_JEV_API_KEY is set")
	flag.Parse()

	// MG_NO_ANIMATIONS=1 env var as alternative to --no-animations flag
	if !*noAnimations && os.Getenv("MG_NO_ANIMATIONS") == "1" {
		*noAnimations = true
	}

	// --agent flag takes precedence over MG_AGENT_RUNTIME env var; both feed
	// the same env-based contract consumed by internal/agent.DetectRuntime.
	if *agentRuntime != "" {
		os.Setenv("MG_AGENT_RUNTIME", *agentRuntime)
	}

	// --agent-cmd sets MG_AGENT_CMD, the same env-based contract, read at
	// launch time by internal/agent to interpose a wrapper (a gateway router,
	// a sandbox, a credential broker) between mg and the agent binary. Pair it
	// with --agent when the wrapper stands in for a specific runtime: the
	// wrapper is exec'd in place of that runtime's binary, with its flags.
	if *agentCmd != "" {
		os.Setenv(agent.AgentCommandEnv, *agentCmd)
	}

	// MG_CMD_TIMEOUT env var as alternative to --cmd-timeout flag
	if *cmdTimeout <= 0 {
		if envTimeout := os.Getenv("MG_CMD_TIMEOUT"); envTimeout != "" {
			if v, err := strconv.Atoi(envTimeout); err == nil && v > 0 {
				*cmdTimeout = v
			}
		}
	}
	if *cmdTimeout > 300 {
		*cmdTimeout = 300
	}
	if *cmdTimeout > 0 {
		gastown.SetCmdTimeout(*cmdTimeout)
		data.SetCmdTimeout(*cmdTimeout)
		jev.SetCmdTimeout(*cmdTimeout)
	}

	// --no-jev feeds the same env contract app reads (MG_JEV=off), like
	// --agent does for MG_AGENT_RUNTIME. Jev is opt-in: without
	// MG_JEV_API_KEY nothing here changes anything.
	if *noJev {
		os.Setenv(jev.EnvToggle, "off")
	}

	if *showVersion {
		fmt.Println("mg", version)
		return
	}

	// Parse blocking types from flag, env var, or default
	blockingTypes := parseBlockingTypes(*blockTypesFlag)
	excludeTypes := parseTypeSet(*excludeTypesFlag)
	excludeLabels := parseTypeSet(*excludeLabelsFlag)

	// Resolve data source: JSONL file or bd CLI fallback
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}
	source := resolveSource(cwd, *path)
	if source.Mode == SourceJSONL && source.Path == "" {
		fmt.Fprintf(os.Stderr, "No .beads/issues.jsonl found and bd not on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Run mg from inside a project with Beads, or specify a path:\n")
		fmt.Fprintf(os.Stderr, "  mg --path /path/to/.beads/issues.jsonl\n\n")
		fmt.Fprintf(os.Stderr, "New to Beads? Install bd (https://github.com/gastownhall/beads),\n")
		fmt.Fprintf(os.Stderr, "then run `bd init` in your project and start mg there.\n")
		os.Exit(1)
	}

	// Load issues
	var issues []data.Issue
	var skipped int // malformed lines the JSONL load skipped
	switch source.Mode {
	case SourceCLI:
		issues, err = data.FetchIssuesCLI(source.ProjectDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues via bd list: %v\n\n", err)
			if hint := data.SchemaSkewHint(err); hint != "" {
				fmt.Fprint(os.Stderr, hint)
			} else {
				fmt.Fprintf(os.Stderr, "Ensure the Dolt server is running (dolt sql-server) and bd is working.\n")
			}
			os.Exit(1)
		}
	default:
		issues, skipped, err = data.LoadIssues(source.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues from %s: %v\n", source.Path, err)
			os.Exit(1)
		}
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "Warning: skipped %d malformed line(s) in %s\n", skipped, source.Path)
		}
	}

	filters := app.Filters{ExcludeTypes: excludeTypes, ExcludeLabels: excludeLabels}

	if *statusMode {
		visible := data.ExcludeByLabel(data.ExcludeByType(issues, excludeTypes), excludeLabels)
		groups := data.GroupByParade(visible, blockingTypes)
		fmt.Print(tmux.StatusLine(groups))
		return
	}

	// Validate the agent wrapper before the TUI owns the terminal: past this
	// point a launch error can only surface as a toast, and only once someone
	// presses a key that launches an agent.
	if err := resolveAgentCmd(); err != nil {
		fmt.Fprintf(os.Stderr, "mg: %v\n", err)
		os.Exit(1)
	}

	// Run TUI. The app reads the Jev config itself; the warning for a
	// misconfigured one belongs on stderr before the TUI takes the terminal.
	if _, err := jev.FromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v; running without Jev\n", err)
	}
	applyTheme(*themeFlag)
	guard := app.NewOSCGuard()
	model := app.NewWithGuard(issues, source, blockingTypes, guard, *noAnimations, filters).WithSkippedLines(skipped)
	p := tea.NewProgram(model, tea.WithFilter(guard.Filter()))
	finalModel, err := p.Run()
	if final, ok := finalModel.(app.Model); ok {
		final.Cleanup()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// resolveAgentCmd checks MG_AGENT_CMD once, at startup, and pins it to the
// absolute path it resolves to. mg refuses to start on a wrapper it cannot
// exec: the wrapper may be what keeps the agent off credentials or the
// network, so launching without it is not a safe fallback. Pinning the
// absolute path also means a later chdir or PATH change cannot make every
// launch resolve to a different file. Unset is fine — agents launch directly.
func resolveAgentCmd() error {
	v := strings.TrimSpace(os.Getenv(agent.AgentCommandEnv))
	if v == "" {
		return nil
	}
	abs, err := agent.ResolveAgentCommand(v)
	if err != nil {
		return err
	}
	return os.Setenv(agent.AgentCommandEnv, abs)
}

// applyTheme resolves the color theme from the --theme flag, the MG_THEME env
// var, or (in auto mode) the terminal's reported background color. It must run
// before tea.NewProgram: the auto probe queries the tty directly, and the ui
// package rebakes every style when the theme switches. Detection failures
// (pipes, terminals that ignore OSC 11) fall back to the dark default.
func applyTheme(flagVal string) {
	mode := flagVal
	if mode == "" {
		mode = os.Getenv("MG_THEME")
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "light":
		ui.SetTheme(ui.ThemeLight)
	case "dark":
		ui.SetTheme(ui.ThemeDark)
	case "", "auto":
		if !lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
			ui.SetTheme(ui.ThemeLight)
		}
	default:
		fmt.Fprintf(os.Stderr, "Warning: unknown theme %q (want auto, dark, or light); using dark\n", mode)
	}
}

// parseBlockingTypes builds the blocking types set from flag, env var, or default.
func parseBlockingTypes(flagVal string) map[string]bool {
	raw := flagVal
	if raw == "" {
		raw = os.Getenv("MG_BLOCK_TYPES")
	}
	types := parseTypeSet(raw)
	if len(types) == 0 {
		return data.DefaultBlockingTypes
	}
	return types
}

func parseTypeSet(flagVal string) map[string]bool {
	if flagVal == "" {
		return nil
	}
	types := make(map[string]bool)
	for _, t := range strings.Split(flagVal, ",") {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			types[t] = true
		}
	}
	return types
}

// findBeadsFile walks up from dir looking for .beads/issues.jsonl.
func findBeadsFile(dir string) string {
	for {
		candidate := filepath.Join(dir, ".beads", "issues.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// findBeadsDir walks up from dir looking for a .beads/ directory (even without issues.jsonl).
func findBeadsDir(dir string) string {
	for {
		candidate := filepath.Join(dir, ".beads")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// bdOnPath returns true if the bd command is available.
func bdOnPath() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// resolveSource determines how mg should load issues.
//
//	--path flag → SourceJSONL with explicit path
//	.beads/ dir exists, bd on PATH → SourceCLI (preferred)
//	.beads/issues.jsonl exists → SourceJSONL (legacy fallback)
//	neither → empty Source (caller should exit with error)
func resolveSource(cwd, pathFlag string) data.Source {
	if pathFlag != "" {
		absPath, err := filepath.Abs(pathFlag)
		if err != nil {
			absPath = filepath.Clean(pathFlag)
		}
		return data.Source{
			Mode:       data.SourceJSONL,
			Path:       absPath,
			ProjectDir: filepath.Dir(filepath.Dir(absPath)),
			Explicit:   true,
		}
	}

	// Prefer CLI when bd is available (JSONL removed upstream in beads v0.56+)
	if projectDir := findBeadsDir(cwd); projectDir != "" && bdOnPath() {
		return data.Source{
			Mode:       data.SourceCLI,
			ProjectDir: projectDir,
		}
	}

	// Legacy fallback: JSONL file exists but bd not on PATH
	if jsonlPath := findBeadsFile(cwd); jsonlPath != "" {
		return data.Source{
			Mode:       data.SourceJSONL,
			Path:       jsonlPath,
			ProjectDir: filepath.Dir(filepath.Dir(jsonlPath)),
		}
	}

	// Nothing found
	return data.Source{}
}
