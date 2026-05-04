package runner

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/fschaefer/ralph/internal/config"
)

var actionRE = regexp.MustCompile(`(?i)ACTION_REQUIRED:\s*(.+)`)

// handleActionInbox checks agent output for ACTION_REQUIRED: and, if found,
// prompts the user (via huh) and writes the response to inbox-response.txt.
func handleActionInbox(cfg *config.Config, output string) error {
	matches := actionRE.FindStringSubmatch(output)
	if len(matches) < 2 {
		return nil
	}
	msg := strings.TrimSpace(matches[1])

	fmt.Println()
	fmt.Println("📬 Action Inbox – agent is waiting for input:")
	fmt.Printf("   %s\n\n", msg)

	var reply string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Your reply").
				Value(&reply),
		),
	)

	if cfg.InboxTimeout > 0 {
		done := make(chan error, 1)
		go func() { done <- form.Run() }()
		select {
		case <-done:
		case <-time.After(time.Duration(cfg.InboxTimeout) * time.Second):
			fmt.Println()
			fmt.Println("⏱️  Timeout – no input received. Continuing without a reply.")
		}
	} else {
		if err := form.Run(); err != nil {
			return fmt.Errorf("reading inbox reply: %w", err)
		}
	}

	if err := os.WriteFile(cfg.InboxResponseFile, []byte(reply+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing inbox response: %w", err)
	}
	fmt.Printf("✅ Reply saved to %s\n", cfg.InboxResponseFile)
	return nil
}

