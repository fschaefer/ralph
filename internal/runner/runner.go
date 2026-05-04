// Package runner implements the ralph agent-loop logic.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/fschaefer/ralph/internal/config"
)

// iterStatus records the outcome of a single iteration.
type iterStatus struct {
	iter int
	code int
	note string
}

// Run executes the main ralph loop. It returns the exit code the process should use.
func Run(cfg *config.Config) int {
	logger := newLogger(cfg.LogFile)

	stopRE, err := regexp.Compile("(?im)" + cfg.StopRegex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid stop-regex %q: %v\n", cfg.StopRegex, err)
		return 1
	}

	startIteration := 1
	if cfg.Resume {
		if saved, err := readIterationFile(cfg.IterationFile); err == nil {
			if saved >= 1 && saved <= cfg.Iterations {
				startIteration = saved
				printInfo(fmt.Sprintf("▶️  Resume: starting at iteration %d", saved))
			}
		}
	}

	if !cfg.Quiet {
		printConfigHeader(cfg)
	}

	startTS := time.Now()
	var statuses []iterStatus

	sigCh := trapSIGINT()

	for i := startIteration; i <= cfg.Iterations; i++ {
		select {
		case <-sigCh:
			fmt.Println()
			printRunSummary(statuses, time.Since(startTS), "⚠️  Interrupted (SIGINT)")
			fmt.Fprintf(os.Stderr, "Last output in %s\n", cfg.LastOutputFile)
			return 130
		default:
		}

		if err := writeIterationFile(cfg.IterationFile, i); err != nil {
			logger.Warn("could not write iteration file", "err", err)
		}

		if !cfg.Quiet {
			printIterBanner(i, cfg.Iterations)
		}

		exitCode, output := runIteration(cfg, i, logger, stopRE)

		// Git diff stat
		if diffStat := gitDiffStat(); diffStat != "" {
			fmt.Println()
			fmt.Println(diffStatStyle.Render("📊 Changes since last commit (git diff --stat HEAD):"))
			fmt.Println(diffStat)
		}

		var note string
		switch {
		case stopRE.MatchString(output):
			note = "✓ stop"
		case cfg.Timeout > 0 && exitCode == 124:
			note = "⏱ timeout"
		case exitCode != 0:
			note = "✗ error"
		default:
			note = "→ continue"
		}
		statuses = append(statuses, iterStatus{iter: i, code: exitCode, note: note})

		if stopRE.MatchString(output) {
			fmt.Println(successStyle.Render(fmt.Sprintf("✅ Stop condition matched in iteration %d", i)))
			printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("✅ Stop condition matched (iteration %d)", i))
			return 0
		}

		// Action inbox
		if cfg.ActionInbox {
			if err := handleActionInbox(cfg, output); err != nil {
				logger.Warn("action inbox error", "err", err)
			}
		}

		if i < cfg.Iterations {
			time.Sleep(time.Duration(cfg.Delay*1000) * time.Millisecond)
		}
	}

	select {
	case <-sigCh:
		fmt.Println()
		printRunSummary(statuses, time.Since(startTS), "⚠️  Interrupted (SIGINT)")
		fmt.Fprintf(os.Stderr, "Last output in %s\n", cfg.LastOutputFile)
		return 130
	default:
	}

	fmt.Println(warnStyle.Render("⚠️  Max iterations reached."))
	printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("⚠️  Max iterations (%d) reached", cfg.Iterations))
	return 2
}

// runIteration executes the agent command once and returns (exitCode, captured output).
func runIteration(cfg *config.Config, iteration int, logger *log.Logger, _ *regexp.Regexp) (int, string) {
	var cmd *exec.Cmd
	if cfg.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, cfg.AgentCmd[0], cfg.AgentCmd[1:]...) //nolint:gosec
	} else {
		cmd = exec.Command(cfg.AgentCmd[0], cfg.AgentCmd[1:]...) //nolint:gosec
	}

	// Open last-output.txt for writing
	lof, err := os.Create(cfg.LastOutputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot open last-output.txt: %v\n", err)
	}
	defer lof.Close()

	// Tee: stdout+stderr → terminal and last-output.txt simultaneously
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			buf.WriteString(line + "\n")
			if lof != nil {
				fmt.Fprintln(lof, line)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot start agent: %v\n", err)
		return 1, ""
	}

	runErr := cmd.Wait()
	pw.Close()
	<-done

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if exitCode == -1 {
				exitCode = 124 // treat killed (timeout) as 124 to match bash behaviour
			}
		} else {
			exitCode = 1
		}
	}

	output := buf.String()

	// Append to ralph.log
	logger.Info(fmt.Sprintf("Iteration %d exit=%d", iteration, exitCode))
	if _, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		// logger already writes to log file; append raw output separately
		if f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(f, output)
			f.Close()
		}
	}

	return exitCode, output
}

// newLogger creates a charmbracelet/log logger writing to the given log file.
func newLogger(logFile string) *log.Logger {
	if err := os.MkdirAll(strings.TrimSuffix(logFile, "/ralph.log"), 0o755); err == nil {
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			return log.New(f)
		}
	}
	return log.New(io.Discard)
}

func readIterationFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func writeIterationFile(path string, i int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(i)), 0o644)
}

func gitDiffStat() string {
	out, err := exec.Command("git", "diff", "--stat", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	bannerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	successStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	diffStatStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tableKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tableValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
)

func printInfo(msg string) {
	fmt.Println(msg)
}

func printConfigHeader(cfg *config.Config) {
	fmt.Println(headerStyle.Render("--- Ralph Configuration ---"))
	row := func(k, v string) {
		fmt.Printf("  %s %s\n", tableKeyStyle.Render(fmt.Sprintf("%-18s", k)), tableValStyle.Render(v))
	}
	row("Iterations:", strconv.Itoa(cfg.Iterations))
	row("Delay:", fmt.Sprintf("%gs", cfg.Delay))
	if cfg.Timeout > 0 {
		row("Timeout:", fmt.Sprintf("%ds", cfg.Timeout))
	} else {
		row("Timeout:", "disabled")
	}
	row("Stop regex:", cfg.StopRegex)
	row("Resume:", yesNo(cfg.Resume))
	if cfg.Worktree && cfg.WorktreePath != "" {
		row("Worktree:", cfg.WorktreePath)
	} else {
		row("Worktree:", yesNo(cfg.Worktree))
	}
	if cfg.ActionInbox {
		if cfg.InboxTimeout > 0 {
			row("Action inbox:", fmt.Sprintf("yes (timeout: %ds)", cfg.InboxTimeout))
		} else {
			row("Action inbox:", "yes (timeout: unlimited)")
		}
	} else {
		row("Action inbox:", "no")
	}
	if cfg.ExtendSpecName != "" {
		row("Extend spec:", ".ralph/specs/"+cfg.ExtendSpecName+".md")
	}
	row("Log file:", cfg.LogFile)
	if cfg.EffectivePromptFile != "" {
		row("Prompt file:", cfg.EffectivePromptFile)
	}
	fmt.Printf("  %s %s\n", tableKeyStyle.Render("Command:          "), tableValStyle.Render(strings.Join(cfg.AgentCmd, " ")))
	fmt.Println()
}

func printIterBanner(i, total int) {
	sep := strings.Repeat("=", 60)
	fmt.Println(bannerStyle.Render(sep))
	fmt.Println(bannerStyle.Render(fmt.Sprintf("Iteration %d/%d", i, total)))
	fmt.Println(bannerStyle.Render(sep))
}

func printRunSummary(statuses []iterStatus, elapsed time.Duration, outcome string) {
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	sep := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(headerStyle.Render(sep))
	fmt.Println(headerStyle.Render("📋 Run Summary"))
	fmt.Println(headerStyle.Render(sep))
	fmt.Printf("  %-20s %dm %02ds\n", tableKeyStyle.Render("Total time:"), mins, secs)
	if len(statuses) > 0 {
		fmt.Println()
		fmt.Printf("  %-6s  %-6s  %s\n",
			tableKeyStyle.Render("Iter."),
			tableKeyStyle.Render("Exit"),
			tableKeyStyle.Render("Status"))
		fmt.Printf("  %-6s  %-6s  %s\n", "------", "------", "------")
		for _, s := range statuses {
			fmt.Printf("  %-6d  %-6d  %s\n", s.iter, s.code, s.note)
		}
	}
	fmt.Println()
	fmt.Printf("  %-20s %s\n", tableKeyStyle.Render("Outcome:"), outcome)
	fmt.Println(headerStyle.Render(sep))
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
