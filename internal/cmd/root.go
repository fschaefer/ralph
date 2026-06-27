// Package cmd wires together the CLI entry point using stdlib flag.
package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/prompt"
	"github.com/fschaefer/ralph/internal/runner"
)

const version = "2.0.0"

const usageText = `Usage:
  ralph [iterations] [options] -- <agent-command...>

Description:
  ralph runs an AI agent command in a loop until it signals completion
  or the iteration limit is reached.

Prompt input:
  Use exactly one prompt mode:
    1. --prompt-file <path>
       Use an existing prompt file as-is.
    2. --goal <text> [--stack <text>]
       Generate .ralph/PROMPT.md from the built-in template.
       Use @{PROMPT_FILE} in the agent command to pass it to the agent.

Loop options:
  --max-iterations <n>    Maximum number of iterations
  --delay <s>             Pause between iterations in seconds (default: 2, or $RALPH_DELAY)
  --timeout <s>           Kill one agent run after <s> seconds (default: disabled)
  --stop-regex <expr>     Stop when agent output matches this regex
  --resume                Resume from .ralph/iteration.txt
  --worktree              Run inside an isolated git worktree
  --cleanup               Remove worktrees from previous runs in .ralph/worktrees/
  --await-input           Pause and wait for user response in .ralph/user-response.txt when agent requests input
  --dry-run               Print resolved configuration and exit
  --quiet, -q             Suppress config header and iteration banners
  --version, -v           Print version and exit
  --help, -h              Show help and exit

Prompt options:
  --prompt-file <path>    Use an existing prompt file
  --goal <text>           Project goal for auto-generated prompt
  --stack <text>          Tech stack for auto-generated prompt

Rules:
  --prompt-file cannot be combined with --goal or --stack.
  --stack requires --goal.
  {PROMPT_FILE} requires one of the prompt modes above.
  The -- separator is required before the agent command.

Examples:
  ralph 5 -- claude -p "Fix the failing tests and print COMPLETE: true when done"
  ralph 8 --goal "Build a REST API" --stack "Go, chi, SQLite" -- claude -p @{PROMPT_FILE}
  ralph 4 --prompt-file prompts/review.md --timeout 180 -- claude -p @{PROMPT_FILE}
  ralph 10 --resume --worktree -- claude -p "Continue from tasks.md"

Default stop signal:
  COMPLETE: true
`

// Execute is the entrypoint called from main.
func Execute() {
	cfg := config.New()
	fs := flag.NewFlagSet("ralph", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usageText)
	}

	// --version / -v
	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.BoolVar(&showVersion, "v", false, "Print version and exit (shorthand)")

	// Loop settings
	fs.IntVar(&cfg.Iterations, "max-iterations", 5, "Maximum number of loop iterations")
	fs.Float64Var(&cfg.Delay, "delay", 2, "Pause between iterations in seconds (env: RALPH_DELAY)")
	fs.IntVar(&cfg.Timeout, "timeout", 0, "Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)")
	fs.StringVar(&cfg.StopRegex, "stop-regex", "", "Regex that triggers a successful stop (env: STOP_REGEX)")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "Suppress config header and iteration banners")
	fs.BoolVar(&cfg.Quiet, "q", false, "Suppress config header and iteration banners (shorthand)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Print configuration and exit without running the agent")
	fs.BoolVar(&cfg.Resume, "resume", false, "Resume from last saved iteration (.ralph/iteration.txt)")
	fs.BoolVar(&cfg.Worktree, "worktree", false, "Create an isolated Git worktree for this run (branch: ralph/run-<ts>)")
	fs.BoolVar(&cfg.AwaitInput, "await-input", false, "Pause and wait for user response in .ralph/user-response.txt when agent requests input")
	fs.BoolVar(&cfg.Cleanup, "cleanup", false, "Remove all worktrees from previous runs in .ralph/worktrees/")

	// Prompt
	fs.StringVar(&cfg.Goal, "goal", "", "Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.Stack, "stack", "", "Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.PromptFileOverride, "prompt-file", "", "Use a ready-made prompt file directly (overrides --goal/--stack)")

	// No arguments: print help to stdout and exit 0.
	if len(os.Args) == 1 {
		fmt.Print(usageText)
		os.Exit(0)
	}

	// Split os.Args at '--' to separate ralph flags from agent command.
	ralphArgs, agentArgs := splitAtDashDash(os.Args[1:])
	flagArgs, iterationArg, err := extractIterationArg(ralphArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	if err := fs.Parse(flagArgs); err != nil {
		// flag.ContinueOnError already printed the error; -h/-help exits 0 via ErrHelp.
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if showVersion {
		fmt.Printf("ralph %s\n", version)
		os.Exit(0)
	}

	// RALPH_DELAY env sets default; explicit --delay flag overrides it.
	if !isFlagChanged(fs, "delay") {
		if d, ok := os.LookupEnv("RALPH_DELAY"); ok {
			if v, err := strconv.ParseFloat(d, 64); err == nil {
				cfg.Delay = v
			}
		}
	}

	// Resolve STOP_REGEX from env if not set via flag.
	if cfg.StopRegex == "" {
		if v, ok := os.LookupEnv("STOP_REGEX"); ok && v != "" {
			cfg.StopRegex = v
		} else {
			cfg.StopRegex = `^COMPLETE:[[:space:]]*true$`
		}
	}

	if iterationArg != "" {
		if isFlagChanged(fs, "max-iterations") {
			fmt.Fprintln(os.Stderr, "Error: use either positional iterations or --max-iterations, not both")
			os.Exit(2)
		}
		n, err := strconv.Atoi(iterationArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid iteration count %q\n", iterationArg)
			os.Exit(2)
		}
		cfg.Iterations = n
	}

	// Agent command follows '--'.
	cfg.AgentCmd = agentArgs

	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	// Require agent command for non-dry-run modes.
	if len(cfg.AgentCmd) == 0 && !cfg.DryRun {
		fmt.Fprintln(os.Stderr, "Error: agent command is missing – use '--' to separate ralph flags from the agent command")
		os.Exit(2)
	}

	// --cleanup: remove worktrees from previous runs.
	if cfg.Cleanup {
		if err := runner.CleanupWorktrees(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}

	// Set up derived paths for runtime state files.
	cfg.LogFile = cfg.RalphDir + "/ralph.log"
	cfg.LastOutputFile = cfg.RalphDir + "/last-output.txt"
	cfg.IterationFile = cfg.RalphDir + "/iteration.txt"
	cfg.UserResponseFile = cfg.RalphDir + "/user-response.txt"

	// Resolve prompt file and substitute {PROMPT_FILE} in agent args.
	if err := prompt.Resolve(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// --dry-run: print config and exit.
	if cfg.DryRun {
		runner.DryRun(cfg)
		return
	}

	// --worktree: set up isolated git worktree.
	if cfg.Worktree {
		if err := runner.SetupWorktree(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	// Verify that required external programs are available.
	if err := checkDependencies(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	// Run the main loop; exit with the returned code.
	exitCode := runner.Run(cfg)
	os.Exit(exitCode)
}

func validateConfig(cfg *config.Config) error {
	switch {
	case cfg.PromptFileOverride != "" && cfg.Goal != "":
		return fmt.Errorf("--prompt-file cannot be combined with --goal")
	case cfg.PromptFileOverride != "" && cfg.Stack != "":
		return fmt.Errorf("--prompt-file cannot be combined with --stack")
	case cfg.Stack != "" && cfg.Goal == "":
		return fmt.Errorf("--stack requires --goal")
	}

	for _, arg := range cfg.AgentCmd {
		switch {
		case strings.Contains(arg, "{SPEC_FILE}"):
			return fmt.Errorf("{SPEC_FILE} is no longer supported; use --prompt-file or --goal")
		case strings.Contains(arg, "{PROMPT_FILE}") && cfg.PromptFileOverride == "" && cfg.Goal == "":
			return fmt.Errorf("{PROMPT_FILE} requires --prompt-file or --goal")
		}
	}

	return nil
}

func extractIterationArg(args []string) (flagArgs []string, iterationArg string, err error) {
	valueFlags := map[string]bool{
		"max-iterations": true,
		"delay":          true,
		"timeout":        true,
		"stop-regex":     true,
		"prompt-file":    true,
		"goal":           true,
		"stack":          true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)

			name := strings.TrimLeft(arg, "-")
			name, _, hasValue := strings.Cut(name, "=")
			if valueFlags[name] && !hasValue && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}

		if iterationArg != "" {
			return nil, "", fmt.Errorf("unexpected positional argument %q (did you forget '--'?)", arg)
		}
		if _, err := strconv.Atoi(arg); err != nil {
			return nil, "", fmt.Errorf("unexpected positional argument %q (did you forget '--'?)", arg)
		}
		iterationArg = arg
	}

	return flagArgs, iterationArg, nil
}

// splitAtDashDash splits args into the part before '--' and the part after.
// If '--' is absent, all args are considered ralph flags and agentArgs is nil.
func splitAtDashDash(args []string) (ralphArgs, agentArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// isFlagChanged reports whether flag name was explicitly set on the command line.
func isFlagChanged(fs *flag.FlagSet, name string) bool {
	changed := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			changed = true
		}
	})
	return changed
}
