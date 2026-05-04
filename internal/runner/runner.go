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

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/prompt"
	"github.com/fschaefer/ralph/internal/ui"
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
	defer logger.close()

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
				fmt.Printf("▶  Resume: starting at iteration %d\n", saved)
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
			printRunSummary(statuses, time.Since(startTS), "⚠  Interrupted (SIGINT)")
			fmt.Fprintf(os.Stderr, "Last output in %s\n", cfg.LastOutputFile)
			return 130
		default:
		}

		if err := writeIterationFile(cfg.IterationFile, i); err != nil {
			logger.warn("could not write iteration file: " + err.Error())
		}

		if !cfg.Quiet {
			printIterBanner(i, cfg.Iterations)
		}

		exitCode, output := runIteration(cfg, i, logger, stopRE)

		// Git diff stat
		if diffStat := gitDiffStat(); diffStat != "" {
			fmt.Println()
			fmt.Println(ui.Gray("📊 Changes since last commit (git diff --stat HEAD):"))
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
			fmt.Println(ui.Green(fmt.Sprintf("✔ Stop condition matched in iteration %d", i)))
			printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("✔ Stop condition matched (iteration %d)", i))
			return 0
		}

		// Action inbox
		if cfg.ActionInbox {
			if err := handleActionInbox(cfg, output); err != nil {
				logger.warn("action inbox error: " + err.Error())
			}
		}

		if i < cfg.Iterations {
			time.Sleep(time.Duration(cfg.Delay*1000) * time.Millisecond)
		}
	}

	select {
	case <-sigCh:
		fmt.Println()
		printRunSummary(statuses, time.Since(startTS), "⚠  Interrupted (SIGINT)")
		fmt.Fprintf(os.Stderr, "Last output in %s\n", cfg.LastOutputFile)
		return 130
	default:
	}

	fmt.Println(ui.Yellow("⚠  Max iterations reached."))
	printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("⚠  Max iterations (%d) reached", cfg.Iterations))
	return 2
}

// runIteration executes the agent command once and returns (exitCode, captured output).
func runIteration(cfg *config.Config, iteration int, logger *fileLogger, _ *regexp.Regexp) (int, string) {
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
	logger.info(fmt.Sprintf("Iteration %d exit=%d", iteration, exitCode))
	logger.write(output)

	return exitCode, output
}

// ── Simple file logger ────────────────────────────────────────────────────────

type fileLogger struct {
	f *os.File
}

func newLogger(logFile string) *fileLogger {
	dir := strings.TrimSuffix(logFile, "/ralph.log")
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			return &fileLogger{f: f}
		}
	}
	return &fileLogger{}
}

func (l *fileLogger) info(msg string) {
	if l.f != nil {
		fmt.Fprintf(l.f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	}
}

func (l *fileLogger) warn(msg string) {
	if l.f != nil {
		fmt.Fprintf(l.f, "[%s] WARN %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	}
}

func (l *fileLogger) write(s string) {
	if l.f != nil {
		fmt.Fprint(l.f, s)
	}
}

func (l *fileLogger) close() {
	if l.f != nil {
		l.f.Close()
	}
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

// ── Output helpers ────────────────────────────────────────────────────────────

func printConfigHeader(cfg *config.Config) {
	fmt.Println(ui.Sep())
	fmt.Printf("  %s\n\n", ui.Header("Ralph"))
	row := func(k, v string) {
		fmt.Printf("  %s %s\n", ui.Gray(fmt.Sprintf("%-18s", k)), v)
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
	if src := prompt.PromptSource(cfg); src != "" {
		row("Prompt file:", src)
	}
	fmt.Printf("  %s %s\n", ui.Gray(fmt.Sprintf("%-18s", "Command:")), strings.Join(cfg.AgentCmd, " "))
	fmt.Println()
}

func printIterBanner(i, total int) {
	fmt.Println(ui.Sep())
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Iteration %d / %d", i, total)))
	fmt.Println(ui.Sep())
}

func printRunSummary(statuses []iterStatus, elapsed time.Duration, outcome string) {
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	fmt.Println()
	fmt.Println(ui.Sep())
	fmt.Printf("  %s\n\n", ui.Header("Run Summary"))
	fmt.Printf("  %s  %dm %02ds\n", ui.Gray(fmt.Sprintf("%-20s", "Total time:")), mins, secs)
	if len(statuses) > 0 {
		fmt.Println()
		fmt.Printf("  %s  %s  %s\n",
			ui.Gray(fmt.Sprintf("%-6s", "Iter.")),
			ui.Gray(fmt.Sprintf("%-6s", "Exit")),
			ui.Gray("Status"))
		fmt.Printf("  %-6s  %-6s  %s\n", "------", "------", "------")
		for _, s := range statuses {
			fmt.Printf("  %-6d  %-6d  %s\n", s.iter, s.code, s.note)
		}
	}
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Gray(fmt.Sprintf("%-20s", "Outcome:")), outcome)
	fmt.Println(ui.Sep())
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

