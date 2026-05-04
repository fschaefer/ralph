// Package cmd wires together the cobra root command.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "2.0.0"

var rootCmd = &cobra.Command{
	Use:   "ralph [options] [iterations] -- <agent-command...>",
	Short: "Minimal autonomous AI agent loop runner",
	Long: `ralph runs an agent command in a loop until it signals completion
or the iteration limit is reached.

The '--' separator is REQUIRED to distinguish ralph flags from the agent command.

Examples:
  ralph 5 -- claude -p @{PROMPT_FILE}
  ralph --goal "Build a REST API" --stack "Go, chi" 10 -- claude -p @{PROMPT_FILE}
  ralph --monitor`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          run,
}

// Execute is the entrypoint called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	// All flags are defined in flags.go; run logic lives in run.go.
	defineFlags(rootCmd)
	// Match ralph.sh --version output format: "ralph <version>"
	rootCmd.SetVersionTemplate("ralph {{.Version}}\n")
}
