package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
)

// runHeadless executes a single prompt and streams output to stdout/stderr.
func runHeadless(ctx context.Context, a *agent.Agent, sess *session.Session, store *session.Store, prompt string) error {
	msgCh := make(chan tea.Msg, 100)

	go func() {
		defer close(msgCh)
		_ = a.RunLoop(ctx, prompt, msgCh)
	}()

	for msg := range msgCh {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		switch m := msg.(type) {
		case agent.StreamChunkMsg:
			fmt.Print(m.Content)
			if m.Done {
				fmt.Println()
			}

		case agent.ToolCallMsg:
			fmt.Printf("\n⚙ %s", m.Name)
			if m.Status == "awaiting_approval" {
				fmt.Print(" (awaiting approval)")
			}
			fmt.Println()

		case agent.ToolResultMsg:
			if m.Success {
				fmt.Printf("  ✓ %s\n", truncate(m.Data, 80))
			} else {
				fmt.Printf("  ✗ %s\n", truncate(m.Error, 80))
			}

		case agent.ErrorMsg:
			fmt.Fprintf(os.Stderr, "\n✗ %s\n", truncate(m.Error, 200))

		case agent.CheckpointPromptMsg:
			fmt.Printf("\n⏸ Checkpoint: phase=%s %d/%d tasks done\n", m.Phase, m.Completed, m.Total)
			fmt.Println("  (auto-resume — headless mode; use TUI for Y/N)")
			select {
			case m.Reply <- agent.CheckpointResponse{Resume: true}:
			case <-ctx.Done():
				return ctx.Err()
			}

		case agent.ConfirmRequestMsg:
			fmt.Printf("\n⛔ Permission required: %s\n", m.Description)
			fmt.Println("  (denied — headless mode blocks dangerous ops; use TUI or --yolo)")
			select {
			case m.Reply <- permissions.ConfirmResponse{Granted: false}:
			case <-ctx.Done():
				return ctx.Err()
			}

		case agent.ResolveConflictMsg:
			fmt.Printf("\n⚠ Checkpoint conflict: phase=%s hasInProgress=%v\n", m.LastPhase, m.HasInProgress)

		case agent.DoneMsg:
			fmt.Println("\n── Session complete ──")
		}
	}

	if err := store.Save(sess); err != nil {
		fmt.Fprintf(os.Stderr, "session save: %v\n", err)
	}
	if err := store.Export(sess); err != nil {
		fmt.Fprintf(os.Stderr, "session export: %v\n", err)
	}

	return nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
