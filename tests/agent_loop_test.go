package tests

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// TestAgentLoopFullCycle runs a complete multi-turn reasoning cycle with Mock LLM.
func TestAgentLoopFullCycle(t *testing.T) {
	// Setup: Mock LLM with two responses
	// Turn 1: tool call → write_file
	// Turn 2: done
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "Let me create that file for you.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call-1",
					Name: "write_file",
					Arguments: map[string]interface{}{
						"path":    "/tmp/test-agent.txt",
						"content": "hello from agent test",
					},
				},
			},
		},
		{
			Content:   "File created successfully!",
			ToolCalls: nil,
		},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(false) // non-interactive (auto-approve safe)
	sess := session.New()
	sessionStore := session.NewStore("/tmp/.ai_history/sessions/")

	a := agent.New(mock, permChecker, toolbox, sess)
	a.SetStore(sessionStore)

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run agent loop in goroutine
	go func() {
		defer close(msgCh)
		if err := a.RunLoop(ctx, "create a test file", msgCh); err != nil {
			t.Logf("RunLoop error: %v", err)
		}
	}()

	// Collect all messages
	var msgs []tea.Msg
	for msg := range msgCh {
		msgs = append(msgs, msg)
	}

	// Verify: state transitions, tool execution, done
	foundThinking := false
	foundExecuting := false
	foundDone := false

	for _, m := range msgs {
		switch m.(type) {
		case agent.AgentStateMsg:
			as := m.(agent.AgentStateMsg)
			if as.State == agent.StateThinking {
				foundThinking = true
			}
			if as.State == agent.StateExecuting {
				foundExecuting = true
			}
		case agent.DoneMsg:
			foundDone = true
		}
	}

	if !foundThinking {
		t.Error("expected StateThinking transition")
	}
	if !foundExecuting {
		t.Error("expected StateExecuting transition")
	}
	if !foundDone {
		t.Error("expected DoneMsg at end of loop")
	}

	// Verify session has entries
	if sess.Count() == 0 {
		t.Error("expected session entries after loop")
	}
	t.Logf("Session entries: %d", sess.Count())

	// Verify tool was executed (session contains tool result)
	foundToolResult := false
	for _, e := range sess.Entries {
		if e.Role == "tool" {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Error("expected tool result entry in session")
	}
}

// TestAgentLoopToolCallsWithoutContent verifies assistant+tool message pairing
// when the LLM returns tool_calls with empty content (OpenAI requirement).
func TestAgentLoopToolCallsWithoutContent(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call-1",
					Name: "list_dir",
					Arguments: map[string]interface{}{
						"path": ".",
					},
				},
			},
		},
		{Content: "Done.", ToolCalls: nil},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	a := agent.New(mock, permissions.NewChecker(false), toolbox, session.New())
	a.SetSubAgent(true) // skip pipeline, test standard loop only

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer close(msgCh)
		if err := a.RunLoop(ctx, "nihao", msgCh); err != nil {
			t.Errorf("RunLoop error: %v", err)
		}
	}()

	for range msgCh {
	}

	var sawAssistantToolCalls, sawToolResult bool
	for _, e := range a.Session().Entries {
		if e.Role == "assistant" && len(e.AssistantToolCalls) > 0 {
			sawAssistantToolCalls = true
		}
		if e.Role == "tool" {
			sawToolResult = true
		}
	}
	if !sawAssistantToolCalls {
		t.Error("expected assistant entry with tool_calls before tool result")
	}
	if !sawToolResult {
		t.Error("expected tool result entry")
	}
}

// TestAgentLoopMaxTurns verifies that the loop stops after maxTurns.
func TestAgentLoopMaxTurns(t *testing.T) {
	// LLM that always returns tool calls — should hit maxTurns limit
	mock := llm.NewMockProvider(nil)
	for i := 0; i < 5; i++ {
		mock.AddResponse(&llm.Response{
			Content: "processing...",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call-x",
					Name: "list_dir",
					Arguments: map[string]interface{}{
						"path": ".",
					},
				},
			},
		})
	}

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(false)
	sess := session.New()

	a := agent.New(mock, permChecker, toolbox, sess)
	a.SetMaxTurns(3) // Should stop after turn 3

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer close(msgCh)
		a.RunLoop(ctx, "list dir", msgCh)
	}()

	var msgs []tea.Msg
	for msg := range msgCh {
		msgs = append(msgs, msg)
	}

	foundDone := false
	for _, m := range msgs {
		if _, ok := m.(agent.DoneMsg); ok {
			foundDone = true
		}
	}
	if !foundDone {
		t.Error("expected DoneMsg after maxTurns")
	}
}
