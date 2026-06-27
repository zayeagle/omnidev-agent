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

// TestDangerousToolApprovalFlow verifies the approve path.
func TestDangerousToolApprovalFlow(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "Let me delete that file.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "del-1",
					Name: "delete_file",
					Arguments: map[string]interface{}{
						"path": "/tmp/test-delete-nonexistent",
					},
				},
			},
		},
		{
			Content:   "File deleted.",
			ToolCalls: nil,
		},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(true) // interactive mode ON
	sess := session.New()

	a := agent.New(mock, permChecker, toolbox, sess)

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer close(msgCh)
		a.RunLoop(ctx, "delete a file", msgCh)
	}()

	// Read messages, looking for ConfirmRequestMsg
	var confirmReq *agent.ConfirmRequestMsg
	for msg := range msgCh {
		if cr, ok := msg.(agent.ConfirmRequestMsg); ok {
			confirmReq = &cr
			break
		}
	}

	if confirmReq == nil {
		t.Fatal("expected ConfirmRequestMsg for dangerous operation")
	}
	if confirmReq.Level != permissions.LevelDangerous {
		t.Errorf("expected LevelDangerous, got %v", confirmReq.Level)
	}

	// Approve
	confirmReq.Reply <- permissions.ConfirmResponse{Granted: true, Reason: "test approval"}

	// Drain remaining messages
	for range msgCh {
	}

	// Verify session recorded the tool call as allowed
	foundAllowed := false
	for _, e := range sess.Entries {
		for _, tc := range e.ToolCalls {
			if tc.Name == "delete_file" && tc.Allowed {
				foundAllowed = true
			}
		}
	}
	if !foundAllowed {
		t.Error("expected delete_file entry with Allowed=true in session")
	}
}

// TestDangerousToolDenyFlow verifies the deny path.
func TestDangerousToolDenyFlow(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "Let me run a shell command.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "sh-1",
					Name: "shell_exec",
					Arguments: map[string]interface{}{
						"cmd": "rm -rf /tmp/test",
					},
				},
			},
		},
		{
			Content:   "Command was denied by user.",
			ToolCalls: nil,
		},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(true) // interactive mode ON
	sess := session.New()

	a := agent.New(mock, permChecker, toolbox, sess)

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer close(msgCh)
		a.RunLoop(ctx, "run dangerous command", msgCh)
	}()

	// Read messages, looking for ConfirmRequestMsg
	var confirmReq *agent.ConfirmRequestMsg
	for msg := range msgCh {
		if cr, ok := msg.(agent.ConfirmRequestMsg); ok {
			confirmReq = &cr
			break
		}
		_ = msg
	}

	time.Sleep(50 * time.Millisecond)
	if confirmReq == nil {
		t.Fatal("expected ConfirmRequestMsg for dangerous operation")
	}

	// Deny
	confirmReq.Reply <- permissions.ConfirmResponse{Granted: false, Reason: "test denial"}

	// Drain remaining messages
	for range msgCh {
	}

	// Verify session recorded the rejection
	foundDenied := false
	for _, e := range sess.Entries {
		for _, tc := range e.ToolCalls {
			if tc.Name == "shell_exec" && !tc.Allowed {
				foundDenied = true
			}
		}
	}
	if !foundDenied {
		t.Error("expected shell_exec entry with Allowed=false in session")
	}
}

// TestSafeToolNoApproval verifies that LevelSafe tools don't trigger confirmation.
func TestSafeToolNoApproval(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "Let me read that file.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "read-1",
					Name: "read_file",
					Arguments: map[string]interface{}{
						"path": "/tmp/test-read-nonexistent",
					},
				},
			},
		},
		{
			Content:   "File content.",
			ToolCalls: nil,
		},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(true) // interactive ON but safe tools skip
	sess := session.New()

	a := agent.New(mock, permChecker, toolbox, sess)

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		defer close(msgCh)
		a.RunLoop(ctx, "read a file", msgCh)
	}()

	// Safe tools should NOT produce a ConfirmRequestMsg
	for msg := range msgCh {
		if _, ok := msg.(agent.ConfirmRequestMsg); ok {
			t.Error("unexpected ConfirmRequestMsg for LevelSafe tool")
		}
	}
}
