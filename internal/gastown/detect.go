// Package gastown integrates with the Gas Town multi-agent orchestrator.
// It provides environment detection, status parsing, agent dispatch
// (sling/nudge), convoy management, mail, molecule DAG layout, analytics,
// vitals monitoring, and problem detection.
package gastown

import (
	"os"
	"os/exec"
)

// Env holds Gas Town environment context read once at startup.
type Env struct {
	Available bool   // gt binary on PATH
	Active    bool   // running inside a Gas Town-managed session
	Role      string // GT_ROLE: mayor, polecat, crew, witness, refinery, deacon, dog
	Rig       string // GT_RIG: rig name (project)
	Scope     string // GT_SCOPE: town or rig
	Worker    string // GT_POLECAT or GT_CREW: worker name
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

	return env
}
