package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fschaefer/ralph/internal/config"
)

// execBackend runs an external agent command via exec.
type execBackend struct {
	agentCmd []string
}

func newExecBackend(cfg *config.Config) *execBackend {
	return &execBackend{agentCmd: cfg.AgentCmd}
}

func (b *execBackend) Setup(cfg *config.Config) error { return nil }

func (b *execBackend) Cleanup() {}

func (b *execBackend) RunIteration(ctx context.Context, cfg *config.Config, iteration int) (int, string) {
	var cmd *exec.Cmd
	if cfg.Timeout > 0 {
		iterCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(iterCtx, b.agentCmd[0], b.agentCmd[1:]...) //nolint:gosec
	} else {
		cmd = exec.CommandContext(ctx, b.agentCmd[0], b.agentCmd[1:]...) //nolint:gosec
	}

	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	lof, err := os.Create(cfg.LastOutputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot open last-output.txt: %v\n", err)
	}
	if lof != nil {
		defer lof.Close()
	}

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
				exitCode = 124
			}
		} else {
			exitCode = 1
		}
	}
	return exitCode, buf.String()
}
