// Package gastown integrates with the Gas Town multi-agent orchestrator.
// It provides environment detection, status parsing, agent dispatch
// (sling/nudge), convoy management, mail, molecule DAG layout, analytics,
// vitals monitoring, and problem detection.
package gastown

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Env holds Gas Town environment context read once at startup.
// The GC* fields are populated when Gas City (`gc`) is detected on PATH or
// a `city.toml` is discovered in the working-tree ancestry. They are
// informational today — mg does not drive `gc`, but exposing the signal
// lets the footer / diagnostics flag when a user has migrated to Gas City.
type Env struct {
	Available bool   // gt binary on PATH
	Active    bool   // running inside a Gas Town-managed session
	Role      string // GT_ROLE: mayor, polecat, crew, witness, refinery, deacon, dog
	Rig       string // GT_RIG: rig name (project)
	Scope     string // GT_SCOPE: town or rig
	Worker    string // GT_POLECAT or GT_CREW: worker name

	GCAvailable bool   // gc binary on PATH (Gas City)
	GCCityPath  string // absolute path to the nearest ancestor containing city.toml, or ""
}

// Detect reads the Gas Town environment. Safe to call even if gt is not installed.
func Detect() Env {
	env := Env{
		Role:  os.Getenv("GT_ROLE"),
		Rig:   os.Getenv("GT_RIG"),
		Scope: os.Getenv("GT_SCOPE"),
	}

	_, err := exec.LookPath("gt")
	env.Available = err == nil

	env.Active = env.Role != "" || env.Rig != ""

	// Worker name comes from the role-specific env var.
	if w := os.Getenv("GT_POLECAT"); w != "" {
		env.Worker = w
	} else if w := os.Getenv("GT_CREW"); w != "" {
		env.Worker = w
	}

	if _, err := exec.LookPath("gc"); err == nil {
		env.GCAvailable = true
	}
	if cwd, err := os.Getwd(); err == nil {
		env.GCCityPath = findCityToml(cwd)
	}

	return env
}

// findCityToml walks up from dir looking for a city.toml marker file.
// Returns the directory containing it, or "" if none found.
func findCityToml(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "city.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
