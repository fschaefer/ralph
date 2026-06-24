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
- GIT SAFETY: Never use destructive commands like ` + "`" + `git reset --hard` + "`" + ` or ` + "`" + `git checkout --` + "`" + `. Commit changes in every turn using ` + "`" + `git commit -m "ralph: <description>"` + "`" + `.
- MACHINE VERIFICATION: A task is only "done" if external checks (tests, linters) pass with exit code 0.
</operational_rules>

<workflow>
You navigate a five-phase workflow. Each turn, determine your current phase by reading ` + "`" + `.ralph/phase` + "`" + ` (or by running ` + "`" + `ls .ralph/work/ 2>/dev/null` + "`" + ` to see what artifacts exist). Progress through phases in order — never skip a phase.

### Phase 1: INTAKE
Understand {{GOAL}} and {{STACK}}. Explore the codebase. Write ` + "`" + `.ralph/work/intake.md` + "`" + ` containing:
- Goal summary (3–5 sentences)
- Affected files/modules
- Dependencies and risks
- Acceptance criteria (checklist)

Signal: write ` + "`" + `ANALYZE` + "`" + ` to ` + "`" + `.ralph/phase` + "`" + `

### Phase 2: ANALYZE
Develop technical understanding. Write ` + "`" + `.ralph/work/analysis.md` + "`" + `:
- Architecture/design changes required
- Interfaces and contracts to maintain
- Edge cases and error handling
- Dependencies (new packages, config changes)

Signal: write ` + "`" + `PLAN` + "`" + ` to ` + "`" + `.ralph/phase` + "`" + `

### Phase 3: PLAN
Break the work into concrete steps. Write ` + "`" + `.ralph/work/tasks.md` + "`" + `:
- Each step is one verifiable unit (one file change, one concern)
- Order respects dependencies
- Format: ` + "`" + `- [ ] description (file:path)` + "`" + ` ` + "`" + `- [x] ...` + "`" + ` when done

Signal: write ` + "`" + `IMPLEMENT` + "`" + ` to ` + "`" + `.ralph/phase` + "`" + `

### Phase 4: IMPLEMENT ⇄ VERIFY
Work through tasks.md one step at a time:
1. Pick the first unchecked task, mark it in-progress (e.g. ` + "`" + `[-] ` + "`" + `)
2. Make the code change (minimal, precise)
3. **RUN** the project's build/test/lint — never skip verification
4. On success: mark ` + "`" + `[x]` + "`" + `, commit (` + "`" + `git commit -m "ralph: <description>"` + "`" + `)
5. On failure: fix and re-verify (⇄ loop). Do NOT move to next task until current one passes.

If you discover new work during implementation, append it to tasks.md and continue.

Signal: when all tasks are marked done and verified, write ` + "`" + `CHECKPOINT` + "`" + ` to ` + "`" + `.ralph/phase` + "`" + `

### Phase 5: CHECKPOINT → DONE
- Run a final build + full test suite
- Validate all acceptance criteria from intake.md
- Document known limitations in ` + "`" + `.ralph/work/notes.md` + "`" + `
- Clean up ` + "`" + `.ralph/work/` + "`" + ` if artifacts are no longer needed

When both conditions are met (all tasks verified + acceptance criteria satisfied):
Output exactly: ` + "`" + `COMPLETE: true` + "`" + `
</workflow>

<recovery>
At the start of EVERY turn:
1. Read ` + "`" + `.ralph/phase` + "`" + ` to know your current phase
2. Read existing work artifacts (` + "`" + `intake.md` + "`" + `, ` + "`" + `analysis.md` + "`" + `, ` + "`" + `tasks.md` + "`" + `)
3. Run ` + "`" + `git log --oneline -n 5` + "`" + ` to see previous changes
4. If the work artifacts contradict the phase (e.g., phase says IMPLEMENT but no tasks.md exists), roll back to the earlier phase and re-do it
5. Resume from where you left off — do NOT restart completed phases
</recovery>

<interruptions>
If you need input from the user (e.g., ambiguous requirement, design decision):
Output: ` + "`" + `ACTION_REQUIRED: <your question>` + "`" + `
Then end your turn. Ralph will pause (if run with --action-inbox) and wait for your response in ` + "`" + `.ralph/inbox-response.txt` + "`" + `.
</interruptions>

<current_context>
# Workspace:
{{DIRECTORY_STRUCTURE}}
# Git Status:
{{GIT_STATUS}}
# Recent Changes:
{{GIT_LOG}}
</current_context>

<workflow_state>
# Phase: (read from .ralph/phase)
# Artifacts: (check .ralph/work/)
</workflow_state>`

// Resolve sets cfg.EffectivePromptFile, generates .ralph/PROMPT.md
// if --goal/--stack are provided, and substitutes {PROMPT_FILE} placeholders
// in cfg.AgentCmd. Returns an error if a referenced file cannot be found.
func Resolve(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.RalphDir, 0o755); err != nil {
		return fmt.Errorf("creating ralph dir: %w", err)
	}

	switch {
	case cfg.PromptFileOverride != "":
		if _, err := os.Stat(cfg.PromptFileOverride); err != nil {
			return fmt.Errorf("prompt file not found: %s", cfg.PromptFileOverride)
		}
		cfg.EffectivePromptFile = cfg.PromptFileOverride

	case cfg.Goal != "" || cfg.Stack != "":
		generated, err := generatePromptFile(cfg)
		if err != nil {
			return err
		}
		cfg.EffectivePromptFile = generated
	}

	// Substitute {PROMPT_FILE} placeholders in agent args.
	if cfg.EffectivePromptFile != "" {
		for i, arg := range cfg.AgentCmd {
			arg = strings.ReplaceAll(arg, "{PROMPT_FILE}", cfg.EffectivePromptFile)
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
	case cfg.Goal != "" || cfg.Stack != "":
		if _, err := os.Stat("PROMPT_TEMPLATE.md"); err == nil {
			return cfg.EffectivePromptFile + " (from PROMPT_TEMPLATE.md)"
		}
		return cfg.EffectivePromptFile + " (built-in template)"
	default:
		return ""
	}
}
