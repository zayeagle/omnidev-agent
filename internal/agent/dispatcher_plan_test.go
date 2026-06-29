package agent

import (
	"context"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func TestPlanMode2SkipsLLM(t *testing.T) {
	mock := llm.NewMockProvider(nil)
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	a := New(mock, permissions.NewChecker(false), toolbox, session.New())
	a.SetPipelineOptions(PipelineOptions{PlanMode: 2})
	d := NewTaskDispatcher(a, DefaultDispatcherOptions())

	tasks, err := d.Plan(context.Background(), "build frontend and backend in parallel")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || !IsVerificationTask(tasks[1]) {
		t.Fatalf("tasks=%+v", tasks)
	}
}
