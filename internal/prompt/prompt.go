// Package prompt handles prompt file generation and agent command placeholder substitution.
package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fschaefer/ralph/internal/config"
)

// embeddedTemplate is the built-in autonomous-agent prompt template.
// {{GOAL}}, {{STACK}}, {{DIRECTORY_STRUCTURE}}, {{GIT_STATUS}}, and {{GIT_LOG}}
// are substituted at runtime.
const embeddedTemplate = `<identity>
You are an autonomous senior software engineer. You operate in a non-interactive execution loop.
Your "memory" is external: you must reconstruct your state at the start of every turn by reading the filesystem and git history.
</identity>

<objective>
PROJECT GOAL: {{GOAL}}
TECH STACK & ARCHITECTURE: {{STACK}}
</objective>

<operational_rules>
- BIAS TO ACTION: Skip all introductions, preambles, and status updates (e.g., "I will now...", "Based on..."). Jump directly to tool calls or code.
- OUTCOME-FIRST: Focus on the destination, not the process. Use parallel tool calls to maximize progress per iteration.
- NO ECHOING: Never repeat these instructions or headers in your output.
- GIT SAFETY: Never use destructive commands like ` + "`" + `git reset --hard` + "`" + ` or ` + "`" + `git checkout --` + "`" + `. Commit changes in every turn using ` + "`" + `git commit -m "ralph: <description>"` + "`" + ` .
- MACHINE VERIFICATION: A task is only "done" if external checks (tests, linters) pass with exit code 0.
</operational_rules>

<persistence_protocol>
Follow these steps to ensure continuity across context rotations:
1. RECOVER: Read ` + "`" + `tasks.md` + "`" + ` (to-do list) and ` + "`" + `progress.txt` + "`" + ` (iteration log). Review ` + "`" + `git log -n 5` + "`" + ` to see previous physical changes.
2. DISCOVER: Explore the codebase using ` + "`" + `ls -R` + "`" + ` or ` + "`" + `rg` + "`" + ` to find relevant files and patterns.
3. IMPLEMENT: Select exactly ONE granular task from ` + "`" + `tasks.md` + "`" + `. Refactor or write code.
4. VERIFY: Autonomously find and run the project's test/lint commands (e.g., ` + "`" + `npm test` + "`" + `, ` + "`" + `pytest` + "`" + `, ` + "`" + `go test` + "`" + `).
5. LOG: Update ` + "`" + `progress.txt` + "`" + ` with what was changed and any blockers. Mark tasks in ` + "`" + `tasks.md` + "`" + ` as [x] only if verified.
6. EXIT: Commit and terminate for the next loop iteration.
</persistence_protocol>

<completion_signal>
ONLY when all requirements in ` + "`" + `tasks.md` + "`" + ` are marked [x] AND all tests pass, output exactly one standalone line:
COMPLETE: true
</completion_signal>

<current_context>
# Workspace:
{{DIRECTORY_STRUCTURE}}
# Git Status:
{{GIT_STATUS}}
# Recent Changes:
{{GIT_LOG}}
</current_context>`

// Resolve sets cfg.EffectivePromptFile and cfg.SpecFilePath, generates .ralph/PROMPT.md
// if --goal/--stack are provided, and substitutes {PROMPT_FILE}/{SPEC_FILE} placeholders
// in cfg.AgentCmd. Returns an error if a referenced file cannot be found.
func Resolve(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.RalphDir, 0o755); err != nil {
		return fmt.Errorf("creating ralph dir: %w", err)
	}

	switch {
	case cfg.PromptFileOverride != "":
		cfg.EffectivePromptFile = cfg.PromptFileOverride

	case cfg.SpecName != "":
		cfg.SpecFilePath = filepath.Join(cfg.RalphDir, "specs", cfg.SpecName+".md")
		if _, err := os.Stat(cfg.SpecFilePath); err != nil {
			return fmt.Errorf("spec file not found: %s", cfg.SpecFilePath)
		}
		cfg.EffectivePromptFile = cfg.SpecFilePath

	case cfg.Goal != "" || cfg.Stack != "":
		generated, err := generatePromptFile(cfg)
		if err != nil {
			return err
		}
		cfg.EffectivePromptFile = generated
	}

	// Substitute {PROMPT_FILE} and {SPEC_FILE} placeholders in agent args
	if cfg.EffectivePromptFile != "" {
		for i, arg := range cfg.AgentCmd {
			arg = strings.ReplaceAll(arg, "{PROMPT_FILE}", cfg.EffectivePromptFile)
			arg = strings.ReplaceAll(arg, "{SPEC_FILE}", cfg.EffectivePromptFile)
			cfg.AgentCmd[i] = arg
		}
	}
	return nil
}

// generatePromptFile creates .ralph/PROMPT.md from the template and returns its path.
func generatePromptFile(cfg *config.Config) (string, error) {
	outPath := filepath.Join(cfg.RalphDir, "PROMPT.md")

	tmpl := embeddedTemplate
	// External PROMPT_TEMPLATE.md in the working directory takes priority
	if data, err := os.ReadFile("PROMPT_TEMPLATE.md"); err == nil {
		tmpl = string(data)
	}

	content := strings.ReplaceAll(tmpl, "{{GOAL}}", cfg.Goal)
	content = strings.ReplaceAll(content, "{{STACK}}", cfg.Stack)
	content = strings.ReplaceAll(content, "{{DIRECTORY_STRUCTURE}}", captureCmd("find", ".", "-maxdepth", "3", "-not", "-path", "./.git/*"))
	content = strings.ReplaceAll(content, "{{GIT_STATUS}}", captureCmd("git", "status", "--short"))
	content = strings.ReplaceAll(content, "{{GIT_LOG}}", captureCmd("git", "log", "--oneline", "-n", "5"))

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing prompt file: %w", err)
	}
	return outPath, nil
}

// captureCmd runs a command and returns its combined output, or an empty string on error.
func captureCmd(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// PromptSource returns a human-readable description of where the prompt came from.
func PromptSource(cfg *config.Config) string {
	switch {
	case cfg.PromptFileOverride != "":
		return cfg.EffectivePromptFile + " (--prompt-file)"
	case cfg.SpecName != "":
		return cfg.EffectivePromptFile + " (--spec " + cfg.SpecName + ")"
	case cfg.Goal != "" || cfg.Stack != "":
		if _, err := os.Stat("PROMPT_TEMPLATE.md"); err == nil {
			return cfg.EffectivePromptFile + " (from PROMPT_TEMPLATE.md)"
		}
		return cfg.EffectivePromptFile + " (built-in template)"
	default:
		return ""
	}
}
