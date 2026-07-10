package runner

import (
	"context"
	"fmt"

	"github.com/fschaefer/ralph/internal/config"
)

// Backend runs a single agent iteration.
type Backend interface {
	// RunIteration executes one agent turn and returns (exitCode, captured output).
	RunIteration(ctx context.Context, cfg *config.Config, iteration int) (int, string)

	// Setup initialises the backend once before the loop.
	Setup(cfg *config.Config) error

	// Cleanup releases resources after the loop.
	Cleanup()
}

// NewBackend returns the appropriate backend for the configuration.
func NewBackend(cfg *config.Config) (Backend, error) {
	if cfg.CopilotSDK {
		return newSdkBackend(cfg), nil
	}
	if len(cfg.AgentCmd) == 0 {
		return nil, fmt.Errorf("agent command required (use --copilot-sdk or -- <agent-cmd>)")
	}
	return newExecBackend(cfg), nil
}
