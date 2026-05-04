// Package cmd wires together the CLI entry point using stdlib flag.
package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/monitor"
	"github.com/fschaefer/ralph/internal/prompt"
	"github.com/fschaefer/ralph/internal/runner"
)

const version = "2.0.0"

const usageText = `Usage:
  ralph [options] [iterations] -- <agent-command...>

Options:
  --delay <s>              Pause between iterations in seconds (default: 2, or $RALPH_DELAY)
  --max-iterations <n>     Maximum number of iterations (alias for positional argument)
  --timeout <s>            Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)
  --stop-regex <pattern>   Regex that triggers a successful stop (default: $STOP_REGEX or ^COMPLETE:...)
  --action-inbox           Pause loop when agent outputs "ACTION_REQUIRED: <msg>";
                           wait for user input and write it to .ralph/inbox-response.txt
  --inbox-timeout <s>      Timeout for user input in seconds (0 = unlimited, default: 0)
  --monitor                Tail .ralph/ralph.log in real-time (open in a second terminal)
  --quiet, -q              Suppress config header and iteration banners
  --dry-run                Print configuration and exit without running the agent
  --resume                 Resume from last saved iteration (.ralph/iteration.txt)
  --worktree               Create an isolated Git worktree for this run (branch: ralph/run-<ts>)
  --goal <text>            Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)
  --stack <text>           Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)
  --prompt-file <path>     Use a ready-made prompt file directly (overrides --goal/--stack)
  --spec <name>            Load .ralph/specs/<name>.md as prompt;
                           use {SPEC_FILE} in the agent command as a placeholder
  --extend-spec <name>     Resume a completed project: appends a new task to tasks.md
                           referencing .ralph/specs/<name>.md so the agent picks it up
  -v, --version            Print version number and exit

Prompt integration:
  With --goal and --stack the prompt template is filled and saved as .ralph/PROMPT.md.
  The template is embedded in the binary; an external PROMPT_TEMPLATE.md in the project
  directory takes priority over the built-in one.
  Use {PROMPT_FILE} anywhere in your agent command as a placeholder for the path:
    ralph --goal "Build a REST API" --stack "Node.js" 5 -- claude -p @{PROMPT_FILE}

Examples:
  ralph 3 -- claude -p "Just output OK"
  ralph --max-iterations 3 -- claude -p "Just output OK"
  ralph --delay 5 3 -- claude -p "Just output OK"
  ralph --stop-regex '^DONE$' 3 -- claude -p "Just output OK"
  ralph --dry-run 3 -- claude -p "Just output OK"
  ralph --resume 3 -- claude -p @{PROMPT_FILE}
  ralph --worktree 5 -- claude -p @{PROMPT_FILE}
  ralph --goal "Build a REST API" --stack "Node.js, Express" 5 -- claude -p @{PROMPT_FILE}
  ralph --timeout 120 5 -- claude -p @{PROMPT_FILE}
  ralph --action-inbox 5 -- claude -p @{PROMPT_FILE}
  ralph --action-inbox --inbox-timeout 120 5 -- claude -p @{PROMPT_FILE}
  ralph --spec myfeature 5 -- claude -p @{SPEC_FILE}
  ralph --monitor        # in a second terminal while a run is active

Note:
  By default the loop stops when the agent output contains a line matching:
    COMPLETE: true
  Customise via --stop-regex or the STOP_REGEX environment variable.
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
	fs.IntVar(&cfg.Iterations, "n", 5, "Maximum number of loop iterations (shorthand)")
	fs.Float64Var(&cfg.Delay, "delay", 2, "Pause between iterations in seconds (env: RALPH_DELAY)")
	fs.IntVar(&cfg.Timeout, "timeout", 0, "Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)")
	fs.StringVar(&cfg.StopRegex, "stop-regex", "", "Regex that triggers a successful stop (env: STOP_REGEX)")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "Suppress config header and iteration banners")
	fs.BoolVar(&cfg.Quiet, "q", false, "Suppress config header and iteration banners (shorthand)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Print configuration and exit without running the agent")
	fs.BoolVar(&cfg.Resume, "resume", false, "Resume from last saved iteration (.ralph/iteration.txt)")
	fs.BoolVar(&cfg.Worktree, "worktree", false, "Create an isolated Git worktree for this run (branch: ralph/run-<ts>)")
	fs.BoolVar(&cfg.Monitor, "monitor", false, "Tail .ralph/ralph.log in real-time (open in a second terminal)")

	// Prompt / spec
	fs.StringVar(&cfg.Goal, "goal", "", "Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.Stack, "stack", "", "Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)")
	fs.StringVar(&cfg.PromptFileOverride, "prompt-file", "", "Use a ready-made prompt file directly (overrides --goal/--stack)")
	fs.StringVar(&cfg.SpecName, "spec", "", "Load .ralph/specs/<name>.md as prompt; use {SPEC_FILE} in agent command")
	fs.StringVar(&cfg.ExtendSpecName, "extend-spec", "", "Resume a completed project: appends a new task to tasks.md referencing .ralph/specs/<name>.md")

	// Action inbox
	fs.BoolVar(&cfg.ActionInbox, "action-inbox", false, `Pause loop when agent outputs "ACTION_REQUIRED: <msg>"; wait for user input`)
	fs.IntVar(&cfg.InboxTimeout, "inbox-timeout", 0, "Timeout for user input in seconds (0 = unlimited; requires --action-inbox)")

	// Split os.Args at '--' to separate ralph flags from agent command.
	ralphArgs, agentArgs := splitAtDashDash(os.Args[1:])

	if err := fs.Parse(ralphArgs); err != nil {
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

	// --monitor needs no agent command; start tail loop and return.
	if cfg.Monitor {
		if err := monitor.Run(cfg.RalphDir); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}

	// The first remaining positional arg (before --) may be an iteration count.
	remaining := fs.Args()
	if len(remaining) > 0 {
		if n, err := strconv.Atoi(remaining[0]); err == nil {
			cfg.Iterations = n
			remaining = remaining[1:]
		}
		if len(remaining) > 0 {
			fmt.Fprintf(os.Stderr, "Error: unexpected positional argument %q (did you forget '--'?)\n", remaining[0])
			os.Exit(2)
		}
	}

	// Agent command follows '--'.
	cfg.AgentCmd = agentArgs

	// Require agent command for non-dry-run modes.
	if len(cfg.AgentCmd) == 0 && !cfg.DryRun {
		fmt.Fprintln(os.Stderr, "Error: agent command is missing – use '--' to separate ralph flags from the agent command")
		os.Exit(2)
	}

	// Set up derived paths.
	cfg.LogFile = cfg.RalphDir + "/ralph.log"
	cfg.LastOutputFile = cfg.RalphDir + "/last-output.txt"
	cfg.IterationFile = cfg.RalphDir + "/iteration.txt"
	cfg.InboxResponseFile = cfg.RalphDir + "/inbox-response.txt"

	// Resolve prompt file and substitute {PROMPT_FILE}/{SPEC_FILE} in agent args.
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

	// --extend-spec: inject new task after entering the worktree (if any).
	if cfg.ExtendSpecName != "" {
		if err := runner.ExtendSpec(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	// Run the main loop; exit with the returned code.
	exitCode := runner.Run(cfg)
	os.Exit(exitCode)
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

