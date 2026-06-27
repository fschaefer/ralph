package cmd

import (
	"fmt"
	"os/exec"

	"github.com/fschaefer/ralph/internal/config"
)

// checkDependencies verifies that all external programs required by ralph
// are available in PATH. Returns the first missing tool as an error.
func checkDependencies(cfg *config.Config) error {
	// git is always required — runner.go calls git diff --stat HEAD
	// and prompt generation uses git status/log.
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("required external program 'git' not found in PATH")
	}

	// find is required when generating a prompt from --goal / --stack
	// (the built-in template uses find to capture directory structure).
	if cfg.Goal != "" || cfg.Stack != "" {
		if _, err := exec.LookPath("find"); err != nil {
			return fmt.Errorf("required external program 'find' not found in PATH (needed for prompt generation)")
		}
	}

	// The agent command itself must be resolvable.
	if len(cfg.AgentCmd) > 0 {
		if _, err := exec.LookPath(cfg.AgentCmd[0]); err != nil {
			return fmt.Errorf("agent executable %q not found in PATH", cfg.AgentCmd[0])
		}
	}

	return nil
}
