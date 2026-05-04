package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fschaefer/ralph/internal/config"
)

// ExtendSpec appends a new task entry to tasks.md and a note to progress.txt.
func ExtendSpec(cfg *config.Config) error {
	specFile := filepath.Join(cfg.RalphDir, "specs", cfg.ExtendSpecName+".md")
	if _, err := os.Stat(specFile); err != nil {
		return fmt.Errorf("extend-spec file not found: %s", specFile)
	}

	ts := time.Now().Format("2006-01-02 15:04:05")

	taskLine := fmt.Sprintf("\n## Extension: %s (%s)\n\n- [ ] Implement new requirements from .ralph/specs/%s.md (read the file for details)\n",
		cfg.ExtendSpecName, ts, cfg.ExtendSpecName)
	if err := appendToFile("tasks.md", taskLine); err != nil {
		return fmt.Errorf("updating tasks.md: %w", err)
	}

	progressLine := fmt.Sprintf("[%s] Extension added via --extend-spec %s\n", ts, cfg.ExtendSpecName)
	if err := appendToFile("progress.txt", progressLine); err != nil {
		return fmt.Errorf("updating progress.txt: %w", err)
	}

	fmt.Printf("📎 Extension injected into tasks.md from %s\n", specFile)
	return nil
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
