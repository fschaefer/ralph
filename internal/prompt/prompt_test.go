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
