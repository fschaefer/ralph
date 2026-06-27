package config

// Config holds all runtime configuration for a ralph run.
type Config struct {
	// Loop settings
	Iterations int
	Delay      float64
	Timeout    int
	StopRegex  string
	Resume     bool
	Quiet      bool
	DryRun     bool
	Worktree   bool

	// Prompt
	Goal               string
	Stack              string
	PromptFileOverride string

	// Action inbox (future: pause and wait for user response)
	ActionInbox bool

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
		StopRegex:  `^COMPLETE:[[:space:]]*true$`,
		RalphDir:   ".ralph",
	}
}
