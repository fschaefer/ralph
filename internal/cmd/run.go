package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/fschaefer/ralph/internal/config"
)

// run is the cobra RunE handler – wires CLI flags → Config and invokes the runner.
func run(cmd *cobra.Command, args []string) error {
	cfg := config.New()

	// Override delay from RALPH_DELAY env if flag not explicitly set
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
	if cfg.Delay, err = cmd.Flags().GetFloat64("delay"); err != nil {
		return err
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
		cfg.StopRegex = `^COMPLETE:\s*true$`
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

	// Parse positional iterations argument (first non-flag arg before --)
	// cobra puts args after -- into args; but we handle the positional iterations arg ourselves.
	// The first positional arg (if numeric) is treated as iterations.
	remaining := args
	if len(remaining) > 0 {
		if n, e := strconv.Atoi(remaining[0]); e == nil {
			cfg.Iterations = n
			remaining = remaining[1:]
		}
	}

	// Set up derived paths
	cfg.LogFile = cfg.RalphDir + "/ralph.log"
	cfg.LastOutputFile = cfg.RalphDir + "/last-output.txt"
	cfg.IterationFile = cfg.RalphDir + "/iteration.txt"
	cfg.InboxResponseFile = cfg.RalphDir + "/inbox-response.txt"

	// --monitor needs no agent command
	if cfg.Monitor {
		fmt.Fprintln(os.Stderr, "monitor mode not yet implemented")
		return nil
	}

	// Require -- separator / agent command
	if len(remaining) == 0 {
		return fmt.Errorf("agent command is missing – use '--' to separate ralph flags from the agent command")
	}
	cfg.AgentCmd = remaining

	fmt.Printf("ralph v%s – configuration loaded (full implementation coming in next tasks)\n", version)
	fmt.Printf("  iterations: %d  delay: %gs  stop: %q\n", cfg.Iterations, cfg.Delay, cfg.StopRegex)
	fmt.Printf("  agent: %v\n", cfg.AgentCmd)
	return nil
}
