package cmd

import (
	"testing"

	"github.com/fschaefer/ralph/internal/config"
)

func TestValidateConfigAcceptsPromptFileMode(t *testing.T) {
	cfg := config.New()
	cfg.PromptFileOverride = "prompt.md"
	cfg.AgentCmd = []string{"agent", "-p", "@{PROMPT_FILE}"}

	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig returned error: %v", err)
	}
}

func TestValidateConfigRejectsPromptFileWithGoal(t *testing.T) {
	cfg := config.New()
	cfg.PromptFileOverride = "prompt.md"
	cfg.Goal = "Build a REST API"

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestValidateConfigRejectsStackWithoutGoal(t *testing.T) {
	cfg := config.New()
	cfg.Stack = "Go, chi, SQLite"

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected stack validation error")
	}
}

func TestValidateConfigRejectsPromptPlaceholderWithoutPromptMode(t *testing.T) {
	cfg := config.New()
	cfg.AgentCmd = []string{"agent", "-p", "@{PROMPT_FILE}"}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected prompt placeholder validation error")
	}
}

func TestValidateConfigRejectsLegacySpecPlaceholder(t *testing.T) {
	cfg := config.New()
	cfg.AgentCmd = []string{"agent", "-p", "@{SPEC_FILE}"}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected legacy spec placeholder validation error")
	}
}

func TestExtractIterationArgAllowsIterationsBeforeOptions(t *testing.T) {
	flags, iteration, err := extractIterationArg([]string{
		"8",
		"--goal", "Build a REST API",
		"--stack=Go, chi, SQLite",
		"--quiet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if iteration != "8" {
		t.Fatalf("iteration = %q, want 8", iteration)
	}
	want := []string{"--goal", "Build a REST API", "--stack=Go, chi, SQLite", "--quiet"}
	if len(flags) != len(want) {
		t.Fatalf("flags = %v, want %v", flags, want)
	}
	for i := range want {
		if flags[i] != want[i] {
			t.Fatalf("flags = %v, want %v", flags, want)
		}
	}
}

func TestExtractIterationArgRejectsMultiplePositionals(t *testing.T) {
	if _, _, err := extractIterationArg([]string{"8", "9"}); err == nil {
		t.Fatal("expected multiple positional arguments to fail")
	}
}
