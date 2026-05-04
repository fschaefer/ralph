package runner

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fschaefer/ralph/internal/config"
)

var actionRE = regexp.MustCompile(`(?i)ACTION_REQUIRED:\s*(.+)`)

// handleActionInbox checks agent output for ACTION_REQUIRED: and, if found,
// prompts the user via plain stdin and writes the response to inbox-response.txt.
func handleActionInbox(cfg *config.Config, output string) error {
	matches := actionRE.FindStringSubmatch(output)
	if len(matches) < 2 {
		return nil
	}
	msg := strings.TrimSpace(matches[1])

	fmt.Println()
	fmt.Println("📬 Action Inbox – agent is waiting for input:")
	fmt.Printf("   %s\n\n", msg)
	fmt.Print("Your reply: ")

	var reply string
	if cfg.InboxTimeout > 0 {
		done := make(chan string, 1)
		go func() {
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			done <- strings.TrimRight(line, "\r\n")
		}()
		select {
		case r := <-done:
			reply = r
		case <-time.After(time.Duration(cfg.InboxTimeout) * time.Second):
			fmt.Println()
			fmt.Println("⏱  Timeout – no input received. Continuing without a reply.")
		}
	} else {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading inbox reply: %w", err)
		}
		reply = strings.TrimRight(line, "\r\n")
	}

	if err := os.WriteFile(cfg.InboxResponseFile, []byte(reply+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing inbox response: %w", err)
	}
	fmt.Printf("✔ Reply saved to %s\n", cfg.InboxResponseFile)
	return nil
}
