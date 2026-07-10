package runner

import (
	"context"
	"fmt"
	"os"

	copilot "github.com/github/copilot-sdk/go"

	"github.com/fschaefer/ralph/internal/config"
)

// sdkBackend communicates with Copilot via the Go SDK.
type sdkBackend struct {
	client  *copilot.Client
	session *copilot.Session
	model   string
}

func newSdkBackend(cfg *config.Config) *sdkBackend {
	model := cfg.Model
	if model == "" {
		model = "auto"
	}
	return &sdkBackend{model: model}
}

func (b *sdkBackend) Setup(cfg *config.Config) error {
	wd, _ := os.Getwd()
	b.client = copilot.NewClient(&copilot.ClientOptions{
		WorkingDirectory: wd,
	})
	if err := b.client.Start(context.Background()); err != nil {
		return fmt.Errorf("copilot client start: %w", err)
	}

	session, err := b.client.CreateSession(context.Background(), &copilot.SessionConfig{
		Model:               b.model,
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
		Streaming:           copilot.Bool(true),
	})
	if err != nil {
		b.client.Stop()
		return fmt.Errorf("copilot create session: %w", err)
	}
	b.session = session
	return nil
}

func (b *sdkBackend) Cleanup() {
	if b.session != nil {
		b.session.Disconnect()
	}
	if b.client != nil {
		b.client.Stop()
	}
}

func (b *sdkBackend) RunIteration(ctx context.Context, cfg *config.Config, iteration int) (int, string) {
	// Read the prompt file content
	prompt, err := os.ReadFile(cfg.EffectivePromptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read prompt file: %v\n", err)
		return 1, ""
	}

	response, err := b.session.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: string(prompt),
	})
	if err != nil {
		if ctx.Err() != nil {
			return 124, "" // timeout or cancelled
		}
		fmt.Fprintf(os.Stderr, "Error: copilot send: %v\n", err)
		return 1, ""
	}

	var content string
	if d, ok := response.Data.(*copilot.AssistantMessageData); ok {
		content = d.Content
	}
	fmt.Print(content)

	// Write to last-output.txt
	if err := os.WriteFile(cfg.LastOutputFile, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot write last-output.txt: %v\n", err)
	}

	return 0, content
}
