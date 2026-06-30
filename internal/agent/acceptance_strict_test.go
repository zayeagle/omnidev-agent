package agent

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// TestStrictExitGateAlwaysContinues verifies exit gate never hard-stops the loop.
func TestStrictExitGateAlwaysContinues(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "done without enough exploration"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	sess := session.New()
	sess.AddWithState("user", "implement feature", StateThinking.String(), 0)
	sess.Add(session.Entry{
		Role:               "assistant",
		AssistantToolCalls: []session.ToolCallData{{Name: "write_file"}},
	})

	a := newStrictTestAgent(mock, toolbox, sess)
	nudges := 5
	msgCh := make(chan tea.Msg, 256)
	go func() {
		for range msgCh {
		}
	}()

	passed, cont := a.runExitGate(context.Background(), msgCh, "implement feature", nil, &nudges)
	if passed {
		t.Fatal("expected exit gate failure with 1 write and 0 exploration")
	}
	if !cont {
		t.Fatal("expected loop to continue after acceptance failure (no hard stop)")
	}
}

// TestStrictMergeResultsFailsWithoutCriteria uses dispatcher merge when criteria cannot be met after recovery.
func TestStrictMergeResultsFailsWithoutCriteria(t *testing.T) {
	mock := llm.NewMockProvider(nil)
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()
	a := newStrictTestAgent(mock, toolbox, sess)
	d := NewTaskDispatcher(a, DefaultDispatcherOptions())

	cp := &Checkpoint{
		Instruction: "implement game",
		Results:     []TaskResult{{TaskID: "1", Success: true, Content: "ok"}},
	}
	msgCh := make(chan tea.Msg, 256)
	go func() {
		for range msgCh {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	outcome := d.mergeResults(ctx, "implement game", cp, msgCh)
	if outcome == OutcomeSuccess {
		t.Fatal("expected failure when acceptance criteria not met in session")
	}
}

func newStrictTestAgent(provider llm.Provider, toolbox *tools.Registry, sess *session.Session) *Agent {
	a := New(provider, permissions.NewChecker(false), toolbox, sess)
	a.SetAcceptanceStrict(true)
	a.SetPipelineOptions(PipelineOptions{UseLLMAcceptance: false, PlanMode: 2})
	return a
}
