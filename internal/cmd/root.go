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
  ralph [iterations] [options] --copilot-sdk

Description:
  ralph runs an AI agent in a loop until it signals completion
  or the iteration limit is reached.
  Two backends: exec (default, any CLI agent) or Copilot SDK (--copilot-sdk).

Prompt input:
  Use exactly one prompt mode:
    1. --prompt-file <path>
       Use an existing prompt file as-is.
    2. --goal <text> [--stack <text>]
       Generate .ralph/PROMPT.md from the built-in template.
       Use @{PROMPT_FILE} in the agent command to pass it to the agent.

Loop options:
  --max-iterations <n>    Maximum number of iterations
  --delay <s>             Pause between iterations in seconds (default: 2)
  --timeout <s>           Kill one agent run after <s> seconds (default: disabled)
  --stop-regex <expr>     Stop when agent output matches this regex
  --worktree              Run inside an isolated git worktree
  --clean-all             Remove the entire .ralph/ directory
  --clean                 Remove worktrees from previous runs in .ralph/worktrees/
  --dry-run               Print resolved configuration and exit
  --quiet, -q             Suppress config header and iteration banners
  --version, -v           Print version and exit
  --help, -h              Show help and exit

Prompt options:
  --prompt-file <path>    Use an existing prompt file
  --goal <text>           Project goal for auto-generated prompt
  --stack <text>          Tech stack for auto-generated prompt

Agent behaviour:
  --ponytail              Lazy-minimalist coding rules: stdlib first, YAGNI, no abstractions

SDK options:
  --copilot-sdk           Use Copilot Go SDK (no -- or agent command needed)
  --model <name>          Model for Copilot SDK (default: auto)

Rules:
  --prompt-file cannot be combined with --goal or --stack.
  --stack requires --goal.
  {PROMPT_FILE} requires one of the prompt modes above.
  The -- separator is required before the agent command (unless --copilot-sdk).

Examples:
  ralph 5 -- claude -p "Fix the failing tests and print COMPLETE: true when done"
  ralph 8 --goal "Build a REST API" --stack "Go, chi, SQLite" -- claude -p @{PROMPT_FILE}
  ralph 50 --goal "Fix tests" --copilot-sdk
  ralph 10 --worktree -- claude -p "Continue from tasks.md"

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
	fs.Float64Var(&cfg.Delay, "delay", 2, "Pause between iterations in seconds")
	fs.IntVar(&cfg.Timeout, "timeout", 0, "Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)")
	fs.StringVar(&cfg.StopRegex, "stop-regex", "", "Regex that triggers a successful stop")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "Suppress config header and iteration banners")
	fs.BoolVar(&cfg.Quiet, "q", false, "Suppress config header and iteration banners (shorthand)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Print configuration and exit without running the agent")
	fs.BoolVar(&cfg.Worktree, "worktree", false, "Create an isolated Git worktree for this run (branch: ralph/run-<ts>)")
	fs.BoolVar(&cfg.CleanAll, "clean-all", false, "Remove the entire .ralph/ directory")
	fs.BoolVar(&cfg.Clean, "clean", false, "Remove all worktrees from previous runs in .ralph/worktrees/")

	// Prompt
	fs.StringVar(&cfg.Goal, "goal", "", "Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.Stack, "stack", "", "Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.PromptFileOverride, "prompt-file", "", "Use a ready-made prompt file directly (overrides --goal/--stack)")

	// Agent behaviour
	fs.BoolVar(&cfg.Ponytail, "ponytail", false, "Enable lazy-minimalist coding rules in the agent prompt")

	// SDK backend
	fs.BoolVar(&cfg.CopilotSDK, "copilot-sdk", false, "Use the Copilot Go SDK instead of a shell agent command")
	fs.StringVar(&cfg.Model, "model", "auto", "Model for Copilot SDK (only used with --copilot-sdk)")

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

	// Default StopRegex (env var support was removed — use --stop-regex instead).
	if cfg.StopRegex == "" {
		cfg.StopRegex = `^COMPLETE:[[:space:]]*true$`
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

	// --clean-all: remove entire .ralph directory and continue.
	if cfg.CleanAll {
		if err := runner.CleanAll(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	// --clean: remove worktrees from previous runs and continue.
	if cfg.Clean {
		if err := runner.CleanWorktrees(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	// Require agent command for non-dry-run, non-sdk modes.
	if len(cfg.AgentCmd) == 0 && !cfg.DryRun && !cfg.CopilotSDK {
		fmt.Fprintln(os.Stderr, "Error: agent command is missing – use '--' to separate ralph flags from the agent command, or --copilot-sdk")
		os.Exit(2)
	}

	// Set up derived paths for runtime state files.
	cfg.LogFile = cfg.RalphDir + "/ralph.log"
	cfg.LastOutputFile = cfg.RalphDir + "/last-output.txt"
	cfg.IterationFile = cfg.RalphDir + "/iteration.txt"

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
	// Build a temporary FlagSet to determine which flags expect values.
	// We use Lookup to check if a flag name is registered and whether it's a bool flag.
	fs := flag.NewFlagSet("ralph", flag.ContinueOnError)
	fs.Bool("version", false, "")
	fs.Bool("v", false, "")
	fs.Int("max-iterations", 0, "")
	fs.Float64("delay", 0, "")
	fs.Int("timeout", 0, "")
	fs.String("stop-regex", "", "")
	fs.Bool("quiet", false, "")
	fs.Bool("q", false, "")
	fs.Bool("dry-run", false, "")
	fs.Bool("worktree", false, "")
	fs.Bool("clean-all", false, "")
	fs.Bool("clean", false, "")
	fs.String("goal", "", "")
	fs.String("stack", "", "")
	fs.String("prompt-file", "", "")
	fs.Bool("ponytail", false, "")
	fs.Bool("copilot-sdk", false, "")
	fs.String("model", "", "")

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)

			name := strings.TrimLeft(arg, "-")
			name, _, hasValue := strings.Cut(name, "=")
			if !hasValue && i+1 < len(args) {
				if f := fs.Lookup(name); f != nil {
					if _, isBool := f.Value.(interface{ IsBoolFlag() bool }); !isBool {
						i++
						flagArgs = append(flagArgs, args[i])
					}
				}
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
