package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/commands"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
)

// runHeadless executes a single prompt and streams output to stdout/stderr.
func runHeadless(ctx context.Context, a *agent.Agent, sess *session.Session, store *session.Store, prompt string) error {
	if cmd, _, ok := commands.Parse(prompt); ok && cmd == "help" {
		fmt.Println(commands.HelpText())
		return nil
	}

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

		case agent.TaskPlanConfirmMsg:
			fmt.Printf("\n📋 Task plan: %d sub-tasks\n", m.TaskCount)
			fmt.Println("  (auto-confirm — headless mode; use TUI for Enter/Esc)")
			select {
			case m.Reply <- agent.TaskPlanConfirmResponse{Confirmed: true}:
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

		case agent.VerificationProgressMsg:
			fmt.Printf("\n%s\n", m.Detail)
			if m.Detail == "" {
				fmt.Printf("Acceptance: %d/%d criteria\n", m.Passed, m.Total)
				for _, c := range m.Criteria {
					mark := "FAIL"
					if c.Met {
						mark = "PASS"
					}
					fmt.Printf("  [%s] %s", mark, c.Text)
					if c.Evidence != "" {
						fmt.Printf(" — %s", c.Evidence)
					}
					fmt.Println()
				}
			}

		case agent.PartialCompleteMsg:
			if len(m.Criteria) > 0 {
				fmt.Println(formatHeadlessAcceptanceDetail(m.Criteria))
			}
			fmt.Fprintf(os.Stderr, "\n⚠ %s\n", m.Summary)

		case agent.DoneMsg:
			fmt.Println("\n── Session complete ──")
		}
	}

	if err := store.SaveActive(sess); err != nil {
		fmt.Fprintf(os.Stderr, "session save: %v\n", err)
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

func formatHeadlessAcceptanceDetail(criteria []agent.CriterionStatus) string {
	var b strings.Builder
	b.WriteString("── Acceptance failure detail ──\n")
	for _, c := range criteria {
		mark := "FAIL"
		if c.Met {
			mark = "PASS"
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", mark, c.Text))
		if ev := strings.TrimSpace(c.Evidence); ev != "" {
			b.WriteString(fmt.Sprintf("      reason: %s\n", ev))
		}
	}
	return strings.TrimSpace(b.String())
}
