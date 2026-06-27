package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fschaefer/ralph/internal/config"
)

func TestResolvePromptFileOverrideRequiresExistingFile(t *testing.T) {
	chdir(t, t.TempDir())

	cfg := config.New()
	cfg.PromptFileOverride = "missing.md"
	cfg.AgentCmd = []string{"agent", "-p", "@{PROMPT_FILE}"}

	err := Resolve(cfg)
	if err == nil {
		t.Fatal("expected missing prompt file to return an error")
	}
	if !strings.Contains(err.Error(), "prompt file not found: missing.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePromptFileOverrideSubstitutesExistingFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	promptPath := filepath.Join(dir, "custom.md")
	if err := os.WriteFile(promptPath, []byte("review this project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.PromptFileOverride = promptPath
	cfg.AgentCmd = []string{"agent", "-p", "@{PROMPT_FILE}"}

	if err := Resolve(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.EffectivePromptFile != promptPath {
		t.Fatalf("EffectivePromptFile = %q, want %q", cfg.EffectivePromptFile, promptPath)
	}
	if got, want := cfg.AgentCmd[2], "@"+promptPath; got != want {
		t.Fatalf("AgentCmd[2] = %q, want %q", got, want)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func TestGeneratePromptFileCreatesFileWithSubstitutions(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := config.New()
	cfg.Goal = "Build a REST API"
	cfg.Stack = "Go, chi, SQLite"

	path, err := generatePromptFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Build a REST API") {
		t.Error("expected GOAL substitution")
	}
	if !strings.Contains(content, "Go, chi, SQLite") {
		t.Error("expected STACK substitution")
	}
	if strings.Contains(content, "DIRECTORY_STRUCTURE") {
		t.Error("expected DIRECTORY_STRUCTURE placeholder to be substituted")
	}
}

func TestGeneratePromptFileUsesExternalTemplate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Write an external PROMPT_TEMPLATE.md
	tmpl := "Goal: {{GOAL}}\nStack: {{STACK}}\n"
	if err := os.WriteFile("PROMPT_TEMPLATE.md", []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.Goal = "Refactor"
	cfg.Stack = "Python"

	path, err := generatePromptFile(cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Goal: Refactor") {
		t.Errorf("expected external template substitution, got: %s", content)
	}
}

func TestRefreshNoopWithoutGoal(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := config.New()
	// No Goal, no Stack — Refresh should be a no-op
	if err := Refresh(cfg); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	// PROMPT.md should not have been created
	if _, err := os.Stat(filepath.Join(cfg.RalphDir, "PROMPT.md")); err == nil {
		t.Error("PROMPT.md should not exist when no goal/stack is set")
	}
}

func TestRefreshRegeneratesWithGoal(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cfg := config.New()
	cfg.Goal = "Test refresh"
	cfg.Stack = "Go"

	// First call creates the file
	if err := Refresh(cfg); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(cfg.RalphDir, "PROMPT.md")
	data1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Change goal and refresh
	cfg.Goal = "Updated goal"
	if err := Refresh(cfg); err != nil {
		t.Fatal(err)
	}

	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(data1) == string(data2) {
		t.Error("expected prompt file to be regenerated with new goal")
	}
	if !strings.Contains(string(data2), "Updated goal") {
		t.Error("expected updated goal in regenerated prompt")
	}
}
