package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matt-wright86/mardi-gras/internal/app"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/tmux"
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
	statusMode := flag.Bool("status", false, "Output tmux status line and exit")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("mg", version)
		return
	}

	// Parse blocking types from flag, env var, or default
	blockingTypes := parseBlockingTypes(*blockTypesFlag)

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
		fmt.Fprintf(os.Stderr, "  mg --path /path/to/.beads/issues.jsonl\n")
		os.Exit(1)
	}

	// Load issues
	var issues []data.Issue
	switch source.Mode {
	case SourceCLI:
		issues, err = data.FetchIssuesCLI()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues via bd list: %v\n\n", err)
			fmt.Fprintf(os.Stderr, "Ensure the Dolt server is running (dolt sql-server) and bd is working.\n")
			os.Exit(1)
		}
	default:
		var skipped int
		issues, skipped, err = data.LoadIssues(source.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues from %s: %v\n", source.Path, err)
			os.Exit(1)
		}
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "Warning: skipped %d malformed line(s) in %s\n", skipped, source.Path)
		}
	}

	if *statusMode {
		groups := data.GroupByParade(issues, blockingTypes)
		fmt.Print(tmux.StatusLine(groups))
		return
	}

	// Run TUI
	model := app.New(issues, source, blockingTypes)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseBlockingTypes builds the blocking types set from flag, env var, or default.
func parseBlockingTypes(flagVal string) map[string]bool {
	raw := flagVal
	if raw == "" {
		raw = os.Getenv("MG_BLOCK_TYPES")
	}
	if raw == "" {
		return data.DefaultBlockingTypes
	}
	types := make(map[string]bool)
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			types[t] = true
		}
	}
	if len(types) == 0 {
		return data.DefaultBlockingTypes
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
//	.beads/issues.jsonl exists → SourceJSONL (current behavior)
//	.beads/ dir exists, no JSONL, bd on PATH → SourceCLI
//	neither → empty Source (caller should exit with error)
func resolveSource(cwd, pathFlag string) data.Source {
	if pathFlag != "" {
		return data.Source{
			Mode:       data.SourceJSONL,
			Path:       pathFlag,
			ProjectDir: filepath.Dir(filepath.Dir(pathFlag)),
			Explicit:   true,
		}
	}

	// Try JSONL first
	if jsonlPath := findBeadsFile(cwd); jsonlPath != "" {
		return data.Source{
			Mode:       data.SourceJSONL,
			Path:       jsonlPath,
			ProjectDir: filepath.Dir(filepath.Dir(jsonlPath)),
		}
	}

	// Fallback: .beads/ dir exists but no JSONL → try CLI
	if projectDir := findBeadsDir(cwd); projectDir != "" && bdOnPath() {
		return data.Source{
			Mode:       data.SourceCLI,
			ProjectDir: projectDir,
		}
	}

	// Nothing found
	return data.Source{}
}
