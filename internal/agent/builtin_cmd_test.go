package agent

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/commands"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func TestRunLoop_BuiltinHelpSkipsPipeline(t *testing.T) {
	a := New(nil, permissions.NewChecker(true), tools.NewRegistry(), session.New())
	msgCh := make(chan tea.Msg, 32)

	err := a.RunLoop(context.Background(), "/help", msgCh)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	close(msgCh)

	var helpText string
	var sawPlan bool
	for msg := range msgCh {
		switch m := msg.(type) {
		case StreamChunkMsg:
			helpText += m.Content
		case TaskPlanMsg:
			sawPlan = true
		}
	}
	if sawPlan {
		t.Fatal("builtin /help must not trigger task decomposition")
	}
	if !strings.Contains(helpText, "/help") {
		t.Fatalf("expected help text, got %q", helpText)
	}
	if a.Session().Count() != 0 {
		t.Fatalf("builtin /help should not add session entries, got %d", a.Session().Count())
	}
	if !strings.Contains(commands.HelpText(), "/help") {
		t.Fatal("sanity check HelpText")
	}
}
