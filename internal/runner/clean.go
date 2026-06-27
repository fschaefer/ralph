package runner

import (
	"fmt"
	"os"

	"github.com/fschaefer/ralph/internal/config"
)

// CleanAll removes the entire .ralph directory and all its contents.
func CleanAll(cfg *config.Config) error {
	if err := os.RemoveAll(cfg.RalphDir); err != nil {
		return fmt.Errorf("removing %s: %w", cfg.RalphDir, err)
	}
	fmt.Printf("🧹 Removed %s/\n", cfg.RalphDir)
	return nil
}
