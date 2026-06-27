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

func TestHeadlessDeniesDangerousOps(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: "delete it",
			ToolCalls: []llm.ToolCall{{
				ID:   "d1",
				Name: "delete_file",
				Arguments: map[string]interface{}{
					"path": "deliverables/tmp.txt",
				},
			}},
		},
		{Content: "done"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()
	perm := permissions.NewForRun(true, false) // headless, no yolo
	a := agent.New(mock, perm, toolbox, sess)

	msgCh := make(chan tea.Msg, 32)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = a.RunLoop(ctx, "cleanup", msgCh)
	}()

	deadline := time.After(3 * time.Second)
	var denied bool
	for !denied {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				goto done
			}
			if tr, ok := msg.(agent.ToolResultMsg); ok && !tr.Success && tr.Error != "" {
				denied = true
			}
		case <-deadline:
			goto done
		}
	}
done:
	if !denied {
		t.Fatal("expected headless to deny dangerous delete_file")
	}
}

func TestLegacyDeleteBlockedAtRoot(t *testing.T) {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()
	a := agent.New(llm.NewMockProvider(nil), permissions.NewChecker(false), toolbox, sess)
	repoRoot, _ := filepathAbsParent()
	a.SetGuard(agent.NewProjectAwarenessGuard(toolbox, sess, repoRoot))

	msg, ok := a.ValidateLegacyWriteForTest("delete_file", map[string]interface{}{"path": "hello_server.go"})
	if ok {
		t.Fatal("expected legacy delete at root to be blocked")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}

func filepathAbsParent() (string, error) {
	return "..", nil // tests run from tests/; repo root is parent
}
