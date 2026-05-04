// Package prompt handles prompt file generation and agent command placeholder substitution.
package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fschaefer/ralph/internal/config"
)

// embeddedTemplate is the built-in autonomous-agent prompt template.
// {{GOAL}} and {{STACK}} are substituted at runtime.
const embeddedTemplate = `SYSTEM DIRECTIVE: AUTONOMOUS RALPH LOOP AGENT
You are an autonomous software engineering agent driven by an external Bash loop. You have no memory between iterations. Your entire knowledge of the project lives exclusively in the filesystem.

1. PROJECT GOAL & SPECIFICATION

- You are building the following project:
{{GOAL}}

- Tech stack & architecture rules:
{{STACK}}

2. STRICT WORKFLOW (ALWAYS FOLLOW IN ORDER!)
Follow these steps exactly in the order given. Do not skip any step.

STEP 1: Orientation (State Recovery)

Read tasks.md (the to-do list) and progress.txt (the log left by previous iterations).

If these files do not exist: this is iteration 1. Create the basic project structure. Create tasks.md with a very granular checklist based on the project goal. Create an empty progress.txt.

STEP 2: Task selection

Identify the next single, logically isolated task in tasks.md that is not yet done.

Never tackle multiple complex things at once.

STEP 3: Implementation

Implement the selected task. Write or refactor the relevant code.

STEP 4: Backpressure & Verification (EXTREMELY IMPORTANT)
Never assume your code works. You must use external validation.

Analyse the project structure autonomously (e.g. read files like package.json, Makefile, Cargo.toml or explore the directory tree) to find out which linter, type-check, and test commands this specific project uses.

Run the identified check commands (e.g. npm test, tsc --noEmit, pytest) via your terminal tool.
If a test or linter fails, analyse the error and fix the code. If you are stuck, document it in step 5 and terminate for the next iteration.

STEP 5: Update memory (Memory Injection)

Append a short entry to progress.txt: which task was worked on, which files were changed, any unresolved errors. (Be brief!)

Mark the task in tasks.md as done (e.g. [x]) only when the code has been written and successfully verified by the commands in step 4 with no errors.

STEP 6: Git commit

Run via terminal: git add . and then git commit with a concise, descriptive message
that summarises what was implemented in this iteration. Use the format:
  ralph: <short description of the task>
Examples:
  git commit -m "ralph: add user authentication endpoint"
  git commit -m "ralph: fix input validation in registration form"
  git commit -m "ralph: implement pagination for product list"
Note: make sure .ralph/ is listed in .gitignore so the runner's state files are not committed.

STEP 7: Termination

Scenario A (there are still open tasks or errors): End your output with a short summary. The external loop will restart you for the next task.

Scenario B (ALL tasks are done AND verified): Only when absolutely all requirements from tasks.md have been completed and all external checks pass without errors, output exactly the following string as a standalone line:

COMPLETE: true`

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

	template := embeddedTemplate
	// External PROMPT_TEMPLATE.md in the working directory takes priority
	if data, err := os.ReadFile("PROMPT_TEMPLATE.md"); err == nil {
		template = string(data)
	}

	content := strings.ReplaceAll(template, "{{GOAL}}", cfg.Goal)
	content = strings.ReplaceAll(content, "{{STACK}}", cfg.Stack)

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing prompt file: %w", err)
	}
	return outPath, nil
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
