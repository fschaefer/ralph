package config

// Config holds all runtime configuration for a ralph run.
type Config struct {
	// Loop settings
	Iterations int
	Delay      float64
	Timeout    int
	StopRegex  string
	Quiet      bool
	DryRun     bool
	Worktree   bool

	// Prompt
	Goal               string
	Stack              string
	PromptFileOverride string

	// Clean removes worktrees from previous runs
	Clean bool

	// CleanAll removes the entire .ralph directory
	CleanAll bool

	// Ponytail enables lazy-minimalist coding rules in the agent prompt.
	Ponytail bool

	// CopilotSDK uses the Copilot Go SDK instead of shelling out to a CLI agent.
	CopilotSDK bool

	// Model selects the model for the Copilot SDK (default: "auto").
	Model string

	// Agent command (after --)
	AgentCmd []string

	// Derived at runtime
	RalphDir            string
	LogFile             string
	LastOutputFile      string
	IterationFile       string
	EffectivePromptFile string
	WorktreePath        string
	PromptSourceNote    string
}

// New returns a Config with sensible defaults.
func New() *Config {
	return &Config{
		Iterations: 5,
		Delay:      2,
		RalphDir:   ".ralph",
	}
}
