package runner

import (
	"fmt"
	"strings"

	"github.com/fschaefer/ralph/internal/config"
)

// DryRun prints the effective configuration and exits without running the agent.
func DryRun(cfg *config.Config) {
	fmt.Println("Dry-run – configuration (no command will be executed):")
	configRow(14, "Iterations:", fmt.Sprintf("%d", cfg.Iterations))
	configRow(14, "Delay:", fmt.Sprintf("%gs", cfg.Delay))
	if cfg.Timeout > 0 {
		configRow(14, "Timeout:", fmt.Sprintf("%ds", cfg.Timeout))
	} else {
		configRow(14, "Timeout:", "disabled")
	}
	configRow(14, "Stop regex:", cfg.StopRegex)
	configRow(14, "Worktree:", yesNo(cfg.Worktree))
	if cfg.PromptSourceNote != "" {
		configRow(14, "Prompt file:", cfg.PromptSourceNote)
	}
	if cfg.CopilotSDK {
		configRow(14, "Backend:", fmt.Sprintf("Copilot SDK (model: %s)", cfg.Model))
	} else {
		fmt.Printf("  %-14s %s\n", "Command:", strings.Join(cfg.AgentCmd, " "))
	}
}
