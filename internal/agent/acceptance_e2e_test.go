package agent

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

type failJudgeProvider struct {
	ok *llm.Response
}

func (f *failJudgeProvider) Chat(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return nil, fmt.Errorf("judge unavailable")
}

func (f *failJudgeProvider) Stream(ctx context.Context, req *llm.Request) (<-chan *llm.Chunk, error) {
	return nil, fmt.Errorf("judge unavailable")
}

func TestJudgeAcceptanceLLMFailureStrictNoHeuristic(t *testing.T) {
	a := New(&failJudgeProvider{}, permissions.NewChecker(false), tools.NewRegistry(), session.New())
	a.SetAcceptanceStrict(true)
	a.SetPipelineOptions(PipelineOptions{UseLLMAcceptance: true})
	plan := AcceptancePlan{Criteria: []string{"Implement feature X"}}
	statuses, allMet := a.judgeAcceptance(context.Background(), "implement X", plan, nil, mechanicalVerifyResult{WorkspaceOK: true, CustomOK: true})
	if allMet {
		t.Fatal("expected failure when LLM judge fails in strict mode")
	}
	if len(statuses) != 1 || statuses[0].Met {
		t.Fatalf("expected unmet criteria, got %+v", statuses)
	}
}

func TestDefaultTaskContractAlignsWithParent(t *testing.T) {
	c := defaultTaskContract(Task{ID: "1", Description: "implement snake game"})
	if c.MinWriteOps < 1 || c.MinReadOps < 2 {
		t.Fatalf("expected write>=1 read>=2, got %+v", c)
	}
}

func TestStrictAgentLoopPassesExitGateAfterToolRounds(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "feature implemented"},
	})

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	sess := session.New()
	sess.AddWithState("user", "implement feature", StateThinking.String(), 0)
	for i := 0; i < 2; i++ {
		sess.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "read_file"}}})
	}
	sess.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "write_file"}}})

	a := newStrictTestAgent(mock, toolbox, sess)
	a.SetSubAgent(true)
	a.SetOutputDir(t.TempDir())
	msgCh := make(chan tea.Msg, 32)

	if err := a.agentLoop(context.Background(), msgCh, true); err != nil {
		t.Fatalf("agentLoop: %v", err)
	}
	if a.state == StateError {
		t.Fatal("expected exit gate pass after read/read/write rounds")
	}
}

func TestParseVerifyCommandsFromInstruction(t *testing.T) {
	cmds := parseVerifyCommandsFromInstruction("Please implement.\nverify: go test -run TestSnake\nrun `npm test`")
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 verify commands, got %v", cmds)
	}
}
