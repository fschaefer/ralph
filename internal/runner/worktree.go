package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fschaefer/ralph/internal/config"
)

// SetupWorktree creates an isolated git worktree and updates cfg paths to point inside it.
// Returns an error if not in a git repo or git commands fail.
func SetupWorktree(cfg *config.Config) error {
	if _, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output(); err != nil {
		return fmt.Errorf("--worktree requires a Git repository")
	}

	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("getting git root: %w", err)
	}
	gitRoot := strings.TrimSpace(string(rootBytes))

	ts := time.Now().Format("20060102-150405")
	branch := "ralph/run-" + ts
	wtPath := filepath.Join(gitRoot, ".ralph", "worktrees", ts)

	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return fmt.Errorf("creating worktrees dir: %w", err)
	}

	out, err := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "HEAD").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, out)
	}
	fmt.Printf("🌿 Worktree created: %s (branch: %s)\n", wtPath, branch)

	cfg.WorktreePath = wtPath
	wtRalphDir := filepath.Join(wtPath, ".ralph")
	if err := os.MkdirAll(wtRalphDir, 0o755); err != nil {
		return fmt.Errorf("creating worktree .ralph dir: %w", err)
	}

	// Copy prompt file into worktree and update agent cmd references
	if cfg.EffectivePromptFile != "" {
		if _, err := os.Stat(cfg.EffectivePromptFile); err == nil {
			dest := filepath.Join(wtRalphDir, "PROMPT.md")
			data, err := os.ReadFile(cfg.EffectivePromptFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot read prompt file for worktree: %v\n", err)
			} else if err := os.WriteFile(dest, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot write prompt file to worktree: %v\n", err)
			} else {
				for i, arg := range cfg.AgentCmd {
					if arg == cfg.EffectivePromptFile {
						cfg.AgentCmd[i] = dest
					} else if arg == "@"+cfg.EffectivePromptFile {
						cfg.AgentCmd[i] = "@" + dest
					}
				}
				cfg.EffectivePromptFile = dest
			}
		}
	}

	// Redirect state files into worktree
	cfg.LogFile = filepath.Join(wtRalphDir, "ralph.log")
	cfg.LastOutputFile = filepath.Join(wtRalphDir, "last-output.txt")
	cfg.IterationFile = filepath.Join(wtRalphDir, "iteration.txt")
	cfg.RalphDir = wtRalphDir

	// Change working directory into worktree
	if err := os.Chdir(wtPath); err != nil {
		return fmt.Errorf("cd into worktree: %w", err)
	}

	return nil
}

// CleanupWorktrees removes all worktrees from previous runs in .ralph/worktrees/.
func CleanupWorktrees(cfg *config.Config) error {
	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("getting git root: %w", err)
	}
	gitRoot := strings.TrimSpace(string(rootBytes))

	wtDir := filepath.Join(gitRoot, ".ralph", "worktrees")
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No worktrees to clean up.")
			return nil
		}
		return fmt.Errorf("reading worktrees dir: %w", err)
	}

	cleaned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(wtDir, entry.Name())
		// Prune the worktree from git, then remove the directory
		out, err := exec.Command("git", "worktree", "remove", "--force", wtPath).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove worktree %s: %s\n", wtPath, strings.TrimSpace(string(out)))
			continue
		}
		cleaned++
		fmt.Printf("🧹 Removed worktree: %s\n", wtPath)
	}

	if cleaned == 0 {
		fmt.Println("No worktrees to clean up.")
	} else {
		fmt.Printf("Cleaned up %d worktree(s).\n", cleaned)
	}
	return nil
}
