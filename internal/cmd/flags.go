package cmd

import (
	"github.com/spf13/cobra"
)

// defineFlags registers all CLI flags on the root command.
// This mirrors every option from the original ralph.sh.
func defineFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	// Loop settings
	f.IntP("max-iterations", "n", 5, "Maximum number of loop iterations")
	f.Float64("delay", 2, "Pause between iterations in seconds (env: RALPH_DELAY)")
	f.Int("timeout", 0, "Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)")
	f.String("stop-regex", "", "Regex that triggers a successful stop (env: STOP_REGEX, default: ^COMPLETE:\\s*true$)")
	f.BoolP("quiet", "q", false, "Suppress config header and iteration banners")
	f.Bool("dry-run", false, "Print configuration and exit without running the agent")
	f.Bool("resume", false, "Resume from last saved iteration (.ralph/iteration.txt)")
	f.Bool("worktree", false, "Create an isolated Git worktree for this run (branch: ralph/run-<ts>)")
	f.Bool("monitor", false, "Tail .ralph/ralph.log in real-time (open in a second terminal)")

	// Prompt / spec
	f.String("goal", "", "Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)")
	f.String("stack", "", "Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)")
	f.String("prompt-file", "", "Use a ready-made prompt file directly (overrides --goal/--stack)")
	f.String("spec", "", "Load .ralph/specs/<name>.md as prompt; use {SPEC_FILE} in agent command")
	f.String("extend-spec", "", "Resume a completed project: appends a new task to tasks.md referencing .ralph/specs/<name>.md")

	// Action inbox
	f.Bool("action-inbox", false, `Pause loop when agent outputs "ACTION_REQUIRED: <msg>"; wait for user input`)
	f.Int("inbox-timeout", 0, "Timeout for user input in seconds (0 = unlimited; requires --action-inbox)")
}
