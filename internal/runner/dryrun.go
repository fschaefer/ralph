package runner

import (
	"fmt"
	"strings"

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/prompt"
)

// DryRun prints the effective configuration and exits without running the agent.
func DryRun(cfg *config.Config) {
	fmt.Println("🔍 Dry-run – configuration (no command will be executed):")
	row := func(k, v string) {
		fmt.Printf("  %-14s %s\n", k, v)
	}
	row("Iterations:", fmt.Sprintf("%d", cfg.Iterations))
	row("Delay:", fmt.Sprintf("%gs", cfg.Delay))
	if cfg.Timeout > 0 {
		row("Timeout:", fmt.Sprintf("%ds", cfg.Timeout))
	} else {
		row("Timeout:", "disabled")
	}
	row("Stop regex:", cfg.StopRegex)
	row("Resume:", yesNo(cfg.Resume))
	row("Worktree:", yesNo(cfg.Worktree))
	if cfg.ActionInbox {
		if cfg.InboxTimeout > 0 {
			row("Action inbox:", fmt.Sprintf("yes (timeout: %ds)", cfg.InboxTimeout))
		} else {
			row("Action inbox:", "yes (timeout: unlimited)")
		}
	} else {
		row("Action inbox:", "no")
	}
	if cfg.ExtendSpecName != "" {
		row("Extend spec:", ".ralph/specs/"+cfg.ExtendSpecName+".md")
	}
	if src := prompt.PromptSource(cfg); src != "" {
		row("Prompt file:", src)
	}
	fmt.Printf("  %-14s %s\n", "Command:", strings.Join(cfg.AgentCmd, " "))
}
