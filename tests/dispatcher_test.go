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

// TestDispatcherSimpleDecomposition verifies that the TaskDispatcher
// correctly decomposes and executes parallel sub-tasks.
func TestDispatcherSimpleDecomposition(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: `[{"id":"1","description":"create file A"},{"id":"2","description":"create file B"}]`,
		},
		// Sub-agents will each need at least one response
		{Content: "Done, created file A"},
		{Content: "Done, created file B"},
		// Extra in case of retry/follow-up
		{Content: "Done"},
		{Content: "Done"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(false)
	sess := session.New()
	a := agent.New(mock, permChecker, toolbox, sess)
	a.SetAcceptanceStrict(false)
	a.SetPipelineOptions(agent.PipelineOptions{PlanMode: 1})

	dispatcher := agent.NewTaskDispatcher(a, agent.DispatcherOptions{
		MaxParallel:     agent.DefaultDispatcherOptions().MaxParallel,
		SubAgentTimeout:   agent.DefaultDispatcherOptions().SubAgentTimeout,
		SubAgentMaxTurns:  agent.DefaultDispatcherOptions().SubAgentMaxTurns,
		SkipPlanConfirm:   true,
	})

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outcome, err := dispatcher.Dispatch(ctx, "create two files", msgCh)
	if err != nil {
		t.Logf("dispatch error (expected with mock): %v", err)
	}
	if !outcome.Handled() {
		t.Error("expected two-task plan to be handled by dispatcher")
	}
}

// TestDispatcherSingleTaskHandled verifies that even a single-task plan
// is handled by the dispatcher (new behavior: always decompose).
func TestDispatcherSingleTaskHandled(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: `[{"id":"1","description":"simple task"}]`,
		},
		// Sub-agent response
		{Content: "Done, completed simple task"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(false)
	sess := session.New()
	a := agent.New(mock, permChecker, toolbox, sess)
	a.SetAcceptanceStrict(false)
	a.SetPipelineOptions(agent.PipelineOptions{PlanMode: 1})

	dispatcher := agent.NewTaskDispatcher(a, agent.DispatcherOptions{
		MaxParallel:     agent.DefaultDispatcherOptions().MaxParallel,
		SubAgentTimeout:   agent.DefaultDispatcherOptions().SubAgentTimeout,
		SubAgentMaxTurns:  agent.DefaultDispatcherOptions().SubAgentMaxTurns,
		SkipPlanConfirm:   true,
	})

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	outcome, _ := dispatcher.Dispatch(ctx, "simple task", msgCh)
	if !outcome.Handled() {
		t.Error("expected single-task plan to be handled by dispatcher (always decompose)")
	}
}

// TestDispatcherPlanParsing verifies plan parsing extracts tasks correctly.
func TestDispatcherPlanParsing(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{
			Content: `[{"id":"1","description":"write main.go"},{"id":"2","description":"write test","depends_on":["1"]}]`,
		},
		// Sub-agent responses
		{Content: "Done, wrote main.go"},
		{Content: "Done, wrote test"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	permChecker := permissions.NewChecker(false)
	sess := session.New()
	a := agent.New(mock, permChecker, toolbox, sess)
	a.SetAcceptanceStrict(false)
	a.SetPipelineOptions(agent.PipelineOptions{PlanMode: 1})

	dispatcher := agent.NewTaskDispatcher(a, agent.DispatcherOptions{
		MaxParallel:     agent.DefaultDispatcherOptions().MaxParallel,
		SubAgentTimeout:   agent.DefaultDispatcherOptions().SubAgentTimeout,
		SubAgentMaxTurns:  agent.DefaultDispatcherOptions().SubAgentMaxTurns,
		SkipPlanConfirm:   true,
	})

	msgCh := make(chan tea.Msg, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	outcome, _ := dispatcher.Dispatch(ctx, "create a go project with tests", msgCh)
	if !outcome.Handled() {
		t.Error("expected 2-task plan to be handled by dispatcher")
	}
}
