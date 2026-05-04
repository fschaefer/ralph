// Package monitor implements the --monitor live-log tail using a simple poll loop.
package monitor

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fschaefer/ralph/internal/ui"
)

const (
	maxLines     = 50
	pollInterval = 500 * time.Millisecond
)

// Run tails ralphDir/ralph.log in real-time until Ctrl+C is pressed.
func Run(ralphDir string) error {
	logPath := filepath.Join(ralphDir, "ralph.log")
	iterPath := filepath.Join(ralphDir, "iteration.txt")

	if _, err := os.Stat(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: No log file found: %s\nStart a ralph run first before using --monitor.\n", logPath)
		return nil
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Read and display the last maxLines of the log file
	printHeader(logPath, iterPath)
	offset := printTail(logPath, maxLines)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println()
			fmt.Println(ui.Dim("Monitor stopped."))
			return nil
		case <-ticker.C:
			offset = appendNew(logPath, iterPath, offset)
		}
	}
}

func printHeader(logPath, iterPath string) {
	iter := readIter(iterPath)
	fmt.Println(ui.Sep())
	fmt.Printf("  %s\n\n", ui.Header("Ralph Monitor – Live Log"))
	fmt.Printf("  %s  %s\n", ui.Gray(fmt.Sprintf("%-18s", "Log file:")), logPath)
	fmt.Printf("  %s  %s\n", ui.Gray(fmt.Sprintf("%-18s", "Current iteration:")), iter)
	fmt.Println(ui.Sep())
	fmt.Println(ui.Dim("  (Press Ctrl+C to stop)"))
	fmt.Println()
}

// printTail prints the last n lines of path and returns the byte offset after the last printed byte.
func printTail(path string, n int) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, l := range lines {
		fmt.Println(ui.Dim(l))
	}
	return int64(len(data))
}

// appendNew reads any bytes after offset, prints new lines, and returns the new offset.
func appendNew(logPath, iterPath string, offset int64) int64 {
	data, err := os.ReadFile(logPath)
	if err != nil || int64(len(data)) <= offset {
		return offset
	}
	newData := data[offset:]
	lines := strings.Split(strings.TrimRight(string(newData), "\n"), "\n")
	for _, l := range lines {
		if l != "" {
			fmt.Println(l)
		}
	}
	// Refresh the iteration counter in the header area is not feasible without a TUI;
	// just print a brief note when the iteration changes.
	_ = iterPath
	return int64(len(data))
}

func readIter(iterPath string) string {
	b, err := os.ReadFile(iterPath)
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}
