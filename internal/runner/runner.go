// Package runner implements the ralph agent-loop logic.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
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

	backend, err := NewBackend(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	if err := backend.Setup(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: backend setup: %v\n", err)
		return 1
	}
	defer backend.Cleanup()

	if !cfg.Quiet {
		printConfigHeader(cfg)
	}

	startTS := time.Now()
	var statuses []iterStatus

	ctx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stopSig()

	for i := 1; i <= cfg.Iterations; i++ {
		select {
		case <-ctx.Done():
			fmt.Println()
			printRunSummary(statuses, time.Since(startTS), "Interrupted (SIGINT)")
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

		exitCode, output := backend.RunIteration(ctx, cfg, i)

		// Append to ralph.log
		logger.info(fmt.Sprintf("Iteration %d exit=%d", i, exitCode))
		logger.write(stripTerminalCodes(output))

		// Git diff stat
		if diffStat := gitDiffStat(); diffStat != "" {
			fmt.Println()
			fmt.Println("Changes since last commit (git diff --stat HEAD):")
			fmt.Println(diffStat)
		}

		stopped := stopRE.MatchString(output)
		var note string
		switch {
		case stopped:
			note = "stop"
		case cfg.Timeout > 0 && exitCode == 124:
			note = "timeout"
		case exitCode != 0:
			note = "error"
		default:
			note = "continue"
		}
		statuses = append(statuses, iterStatus{iter: i, code: exitCode, note: note})

		if stopped {
			fmt.Printf("Stop condition matched in iteration %d\n", i)
			printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("Stop condition matched (iteration %d)", i))
			return 0
		}

		if i < cfg.Iterations {
			time.Sleep(time.Duration(cfg.Delay * float64(time.Second)))
		}
	}

	select {
	case <-ctx.Done():
		fmt.Println()
		printRunSummary(statuses, time.Since(startTS), "Interrupted (SIGINT)")
		fmt.Fprintf(os.Stderr, "Last output in %s\n", cfg.LastOutputFile)
		return 130
	default:
	}

	fmt.Println("Max iterations reached.")
	printRunSummary(statuses, time.Since(startTS), fmt.Sprintf("Max iterations (%d) reached", cfg.Iterations))
	return 2
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

func configRow(w int, k, v string) {
	fmt.Printf("  %-*s %s\n", w, k, v)
}

func printConfigHeader(cfg *config.Config) {
	fmt.Println("--- Ralph Configuration ---")
	configRow(18, "Iterations:", strconv.Itoa(cfg.Iterations))
	configRow(18, "Delay:", fmt.Sprintf("%gs", cfg.Delay))
	if cfg.Timeout > 0 {
		configRow(18, "Timeout:", fmt.Sprintf("%ds", cfg.Timeout))
	} else {
		configRow(18, "Timeout:", "disabled")
	}
	configRow(18, "Stop regex:", cfg.StopRegex)
	if cfg.Worktree && cfg.WorktreePath != "" {
		configRow(18, "Worktree:", cfg.WorktreePath)
	} else {
		configRow(18, "Worktree:", yesNo(cfg.Worktree))
	}
	configRow(18, "Log file:", cfg.LogFile)
	if cfg.CopilotSDK {
		configRow(18, "Backend:", "Copilot SDK")
		configRow(18, "Model:", cfg.Model)
	} else {
		configRow(18, "Backend:", "exec")
	}
	if cfg.PromptSourceNote != "" {
		configRow(18, "Prompt file:", cfg.PromptSourceNote)
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
	fmt.Println("Run Summary")
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
