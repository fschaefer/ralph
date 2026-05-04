package config

// Config holds all runtime configuration for a ralph run.
type Config struct {
	// Loop settings
	Iterations   int
	Delay        float64
	Timeout      int
	StopRegex    string
	Resume       bool
	Quiet        bool
	DryRun       bool
	Monitor      bool
	Worktree     bool

	// Prompt / spec
	Goal             string
	Stack            string
	PromptFileOverride string
	SpecName         string
	ExtendSpecName   string

	// Action inbox
	ActionInbox  bool
	InboxTimeout int

	// Agent command (after --)
	AgentCmd []string

	// Derived at runtime
	RalphDir            string
	LogFile             string
	LastOutputFile      string
	IterationFile       string
	InboxResponseFile   string
	EffectivePromptFile string
	SpecFilePath        string
	WorktreePath        string
}

// New returns a Config with sensible defaults.
func New() *Config {
	return &Config{
		Iterations: 5,
		Delay:      2,
		StopRegex:  `^COMPLETE:\s*true$`,
		RalphDir:   ".ralph",
	}
}
