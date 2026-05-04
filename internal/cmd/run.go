package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/monitor"
	"github.com/fschaefer/ralph/internal/prompt"
	"github.com/fschaefer/ralph/internal/runner"
)

// run is the cobra RunE handler – wires CLI flags → Config and invokes the runner.
func run(cmd *cobra.Command, args []string) error {
	cfg := config.New()

	// RALPH_DELAY env sets default; explicit --delay flag overrides it below
	if d, ok := os.LookupEnv("RALPH_DELAY"); ok {
		if v, err := strconv.ParseFloat(d, 64); err == nil {
			cfg.Delay = v
		}
	}

	// Read flags
	var err error
	if cfg.Iterations, err = cmd.Flags().GetInt("max-iterations"); err != nil {
		return err
	}
	if cmd.Flags().Changed("delay") {
		if cfg.Delay, err = cmd.Flags().GetFloat64("delay"); err != nil {
			return err
		}
	}
	if cfg.Timeout, err = cmd.Flags().GetInt("timeout"); err != nil {
		return err
	}
	if cfg.StopRegex, err = cmd.Flags().GetString("stop-regex"); err != nil {
		return err
	}
	if cfg.StopRegex == "" {
		if v, ok := os.LookupEnv("STOP_REGEX"); ok && v != "" {
			cfg.StopRegex = v
		}
	}
	if cfg.StopRegex == "" {
		cfg.StopRegex = `^COMPLETE:[[:space:]]*true$`
	}
	if cfg.Quiet, err = cmd.Flags().GetBool("quiet"); err != nil {
		return err
	}
	if cfg.DryRun, err = cmd.Flags().GetBool("dry-run"); err != nil {
		return err
	}
	if cfg.Resume, err = cmd.Flags().GetBool("resume"); err != nil {
		return err
	}
	if cfg.Worktree, err = cmd.Flags().GetBool("worktree"); err != nil {
		return err
	}
	if cfg.Monitor, err = cmd.Flags().GetBool("monitor"); err != nil {
		return err
	}
	if cfg.Goal, err = cmd.Flags().GetString("goal"); err != nil {
		return err
	}
	if cfg.Stack, err = cmd.Flags().GetString("stack"); err != nil {
		return err
	}
	if cfg.PromptFileOverride, err = cmd.Flags().GetString("prompt-file"); err != nil {
		return err
	}
	if cfg.SpecName, err = cmd.Flags().GetString("spec"); err != nil {
		return err
	}
	if cfg.ExtendSpecName, err = cmd.Flags().GetString("extend-spec"); err != nil {
		return err
	}
	if cfg.ActionInbox, err = cmd.Flags().GetBool("action-inbox"); err != nil {
		return err
	}
	if cfg.InboxTimeout, err = cmd.Flags().GetInt("inbox-timeout"); err != nil {
		return err
	}

	// --monitor needs no agent command; start TUI and return
	if cfg.Monitor {
		return monitor.Run(cfg.RalphDir)
	}

	// Parse positional iterations argument (first non-flag arg before --)
	remaining := args
	if len(remaining) > 0 {
		if n, e := strconv.Atoi(remaining[0]); e == nil {
			cfg.Iterations = n
			remaining = remaining[1:]
		}
	}

	// Require agent command for non-monitor, non-dry-run modes
	if len(remaining) == 0 && !cfg.DryRun {
		return fmt.Errorf("agent command is missing – use '--' to separate ralph flags from the agent command")
	}
	cfg.AgentCmd = remaining

	// Set up derived paths
	cfg.LogFile = cfg.RalphDir + "/ralph.log"
	cfg.LastOutputFile = cfg.RalphDir + "/last-output.txt"
	cfg.IterationFile = cfg.RalphDir + "/iteration.txt"
	cfg.InboxResponseFile = cfg.RalphDir + "/inbox-response.txt"

	// Resolve prompt file and substitute {PROMPT_FILE}/{SPEC_FILE} in agent args
	if err := prompt.Resolve(cfg); err != nil {
		return err
	}

	// --dry-run: print config and exit
	if cfg.DryRun {
		runner.DryRun(cfg)
		return nil
	}

	// --extend-spec: inject new task before starting the loop
	if cfg.ExtendSpecName != "" {
		if err := runner.ExtendSpec(cfg); err != nil {
			return err
		}
	}

	// --worktree: set up isolated git worktree
	if cfg.Worktree {
		if err := runner.SetupWorktree(cfg); err != nil {
			return err
		}
	}

	// Run the main loop; exit with the returned code
	exitCode := runner.Run(cfg)
	os.Exit(exitCode)
	return nil
}
