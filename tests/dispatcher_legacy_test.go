package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// TestDispatcherLegacyParallelDAG verifies a 3-task plan with two parallel tasks
// and one dependent task on a legacy-style project layout.
func TestDispatcherLegacyParallelDAG(t *testing.T) {
	_ = t.TempDir() // legacy-style fixture reserved for future full-loop test

	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: `[{"id":"1","description":"add helper A"},{"id":"2","description":"add helper B"},{"id":"3","description":"wire helpers","depends_on":["1","2"]}]`,
		},
		{Content: "Done A"},
		{Content: "Done B"},
		{Content: "Done C"},
		{Content: "Done"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	sess := session.New()
	sess.Add(session.Entry{
		Role:    "system",
		Content: "[PROJECT ANALYSIS] Legacy Go project with main.go entry point.",
	})

	a := agent.New(mock, permissions.NewChecker(false), toolbox, sess)
	a.SetPipelineOptions(agent.PipelineOptions{PlanMode: 1})

	dispatcher := agent.NewTaskDispatcher(a, agent.DispatcherOptions{
		MaxParallel:     4,
		SubAgentTimeout: 30 * time.Second,
		SubAgentMaxTurns: 5,
		SkipPlanConfirm: true,
	})

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	handled, err := dispatcher.Dispatch(ctx, "extend legacy app with two helpers", msgCh)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !handled {
		t.Fatal("expected legacy 3-task plan to be handled by dispatcher")
	}

	var running, done int
drain:
	for {
		select {
		case msg := <-msgCh:
			if st, ok := msg.(agent.SubtaskMsg); ok {
				switch st.Status {
				case "running":
					running++
				case "done":
					done++
				}
			}
		default:
			break drain
		}
	}
	if running < 3 {
		t.Fatalf("expected 3 subtasks started, got running=%d done=%d", running, done)
	}
}

// TestWriteEditLineCountIntegration verifies tool results include +/- line stats.
func TestWriteEditLineCountIntegration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	writeTool, ok := toolbox.Get("write_file")
	if !ok {
		t.Fatal("write_file not registered")
	}
	editTool, ok := toolbox.Get("edit_file")
	if !ok {
		t.Fatal("edit_file not registered")
	}

	ctx := context.Background()

	res := writeTool.Execute(ctx, map[string]interface{}{
		"path":    path,
		"content": "line1\nline2\nline3\n",
	})
	if !res.Success {
		t.Fatalf("write: %s", res.Error)
	}
	if !strings.Contains(res.Data, "(+") || !strings.Contains(res.Data, "sample.go") {
		t.Fatalf("write result missing line stats: %q", res.Data)
	}

	res = editTool.Execute(ctx, map[string]interface{}{
		"path":        path,
		"old_snippet": "line2",
		"new_snippet": "line2\nline2b",
	})
	if !res.Success {
		t.Fatalf("edit: %s", res.Error)
	}
	if !strings.Contains(res.Data, "(+") {
		t.Fatalf("edit result missing line stats: %q", res.Data)
	}
}
