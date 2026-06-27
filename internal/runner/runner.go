// Package runner implements the ralph agent-loop logic.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fschaefer/ralph/internal/config"
	"github.com/fschaefer/ralph/internal/prompt"
)

const sep = "============================================================"

// ansiRE matches ANSI/VT100 escape sequences (colors, cursor movement, etc.).
var ansiRE = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-9;?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

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
				fmt.Printf("▶️  Resume: starting at iteration %d\n", saved)
			}
		}
	}

	if !cfg.Quiet {
		printConfigHeader(cfg)
	}

	startTS := time.Now()
	var statuses []iterStatus

	sigCh, stopSig := trapSIGINT()
	defer stopSig()

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
			logger.warn("could not write iteration file: " + err.Error())
		}

		if !cfg.Quiet {
			printIterBanner(i, cfg.Iterations)
		}

		// Regenerate prompt with fresh workspace snapshot before each iteration.
		if err := prompt.Refresh(cfg); err != nil {
			logger.warn("could not refresh prompt: " + err.Error())
		}

		exitCode, output := runIteration(cfg, i, logger)

		// --await-input: If the agent requests user input, pause and wait.
		if cfg.AwaitInput {
			if handled := handleAwaitInput(cfg, output, logger); handled {
				// The user responded; continue to next iteration
				continue
			}
		}

		// Git diff stat
		if diffStat := gitDiffStat(); diffStat != "" {
			fmt.Println()
			fmt.Println("📊 Changes since last commit (git diff --stat HEAD):")
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
			fmt.Printf("✅ Stop condition matched in iteration %d\n", i)
			printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("✅ Stop condition matched (iteration %d)", i))
			return 0
		}

		if i < cfg.Iterations {
			time.Sleep(time.Duration(cfg.Delay * float64(time.Second)))
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

	fmt.Println("⚠️  Max iterations reached.")
	printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("⚠️  Max iterations (%d) reached", cfg.Iterations))
	return 2
}

// runIteration executes the agent command once and returns (exitCode, captured output).
func runIteration(cfg *config.Config, iteration int, logger *fileLogger) (int, string) {
	var cmd *exec.Cmd
	if cfg.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, cfg.AgentCmd[0], cfg.AgentCmd[1:]...) //nolint:gosec
	} else {
		cmd = exec.Command(cfg.AgentCmd[0], cfg.AgentCmd[1:]...) //nolint:gosec
	}

	// Set working directory explicitly for robustness (e.g. after worktree chdir).
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	// Open last-output.txt for writing
	lof, err := os.Create(cfg.LastOutputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot open last-output.txt: %v\n", err)
	}
	if lof != nil {
		defer lof.Close()
	}

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
		pw.Close()
		<-done
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

	// Append to ralph.log (plain text — strip ANSI codes and CR overwrite sequences)
	logger.info(fmt.Sprintf("Iteration %d exit=%d", iteration, exitCode))
	logger.write(stripTerminalCodes(output))

	return exitCode, output
}

// ── Simple file logger ────────────────────────────────────────────────────────

type fileLogger struct {
	f *os.File
}

func newLogger(logFile string) *fileLogger {
	dir := filepath.Dir(logFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot create log directory %s: %v\n", dir, err)
		return &fileLogger{}
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot open log file %s: %v\n", logFile, err)
		return &fileLogger{}
	}
	return &fileLogger{f: f}
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
	fmt.Println("--- Ralph Configuration ---")
	row := func(k, v string) {
		fmt.Printf("  %-18s %s\n", k, v)
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
	row("Log file:", cfg.LogFile)
	if src := prompt.PromptSource(cfg); src != "" {
		row("Prompt file:", src)
	}
	fmt.Printf("  Command:           ")
	fmt.Println(strings.Join(cfg.AgentCmd, " "))
	fmt.Println()
}

func printIterBanner(i, total int) {
	fmt.Println(sep)
	fmt.Printf("Iteration %d/%d\n", i, total)
	fmt.Println(sep)
}

func printRunSummary(statuses []iterStatus, elapsed time.Duration, outcome string) {
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	fmt.Println()
	fmt.Println(sep)
	fmt.Println("📋 Run Summary")
	fmt.Println(sep)
	fmt.Printf("  %-20s %dm %02ds\n", "Total time:", mins, secs)
	if len(statuses) > 0 {
		fmt.Println()
		fmt.Printf("  %-6s  %-6s  %s\n", "Iter.", "Exit", "Status")
		fmt.Printf("  %-6s  %-6s  %s\n", "------", "------", "------")
		for _, s := range statuses {
			fmt.Printf("  %-6d  %-6d  %s\n", s.iter, s.code, s.note)
		}
	}
	fmt.Println()
	fmt.Printf("  %-20s %s\n", "Outcome:", outcome)
	fmt.Println(sep)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// handleAwaitInput checks if the agent output contains ACTION_REQUIRED.
// If so, it prints the question, polls for user-response.txt, reads it,
// and returns true. Returns false if no ACTION_REQUIRED was found.
func handleAwaitInput(cfg *config.Config, output string, logger *fileLogger) bool {
	const prefix = "ACTION_REQUIRED:"
	idx := strings.Index(output, prefix)
	if idx < 0 {
		return false
	}

	// Extract the question (everything after ACTION_REQUIRED: on that line)
	rest := output[idx+len(prefix):]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[:nl]
	}
	question := strings.TrimSpace(rest)

	fmt.Println()
	fmt.Println(sep)
	fmt.Println("💬 Agent requests your input")
	if question != "" {
		fmt.Println("   Question:", question)
	}
	fmt.Printf("   Write your response into %s and save, then I will continue.\n", cfg.UserResponseFile)
	fmt.Println(sep)

	// Poll for the response file
	pollInterval := 2 * time.Second
	for {
		data, err := os.ReadFile(cfg.UserResponseFile)
		if err == nil {
			// File exists — read the response, remove it, and return
			response := strings.TrimSpace(string(data))
			if response != "" {
				logger.info(fmt.Sprintf("User responded: %s", response))
			}
			// Remove file so it doesn't get reused
			os.Remove(cfg.UserResponseFile)
			fmt.Println("✅ Response received, continuing...")
			return true
		}
		time.Sleep(pollInterval)
	}
}

// stripTerminalCodes removes ANSI/VT100 escape sequences and collapses
// carriage-return overwrite sequences (e.g. progress bars) so that log files
// contain human-readable plain text.
func stripTerminalCodes(s string) string {
	// Remove all ANSI escape sequences
	s = ansiRE.ReplaceAllString(s, "")
	// Collapse "text\r<spaces/text>" overwrite sequences: keep only the last
	// segment after the final \r on each line.
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
		}
		// Skip lines that are entirely whitespace (leftover from erased progress bars)
		if strings.TrimSpace(line) == "" {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
