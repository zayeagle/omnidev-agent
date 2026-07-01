package agent

import (
	"testing"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestHeuristicCriterionRequiresWritesAndExploration(t *testing.T) {
	plan := AcceptancePlan{Criteria: []string{"Address the user request: implement snake game"}}
	mech := mechanicalVerifyResult{WorkspaceOK: true, CustomOK: true}
	statuses, ok := judgeAcceptanceHeuristic("implement snake game", plan, sessionWithTools(1, 1, 0, 0), nil, mech)
	if ok {
		t.Fatal("expected failure with only 1 exploration op")
	}

	statuses, ok = judgeAcceptanceHeuristic("implement snake game", plan, sessionWithTools(1, 2, 0, 0), nil, mech)
	if !ok || len(statuses) != 1 || !statuses[0].Met {
		t.Fatalf("expected pass with write+exploration, statuses=%+v ok=%v", statuses, ok)
	}
}

func TestExplorationCountsSearchAndListDir(t *testing.T) {
	plan := AcceptancePlan{Criteria: []string{"Address the user request: implement snake game"}}
	mech := mechanicalVerifyResult{WorkspaceOK: true, CustomOK: true}
	// 1 write + 1 search + 1 list = 2 exploration
	sess := sessionWithTools(1, 0, 1, 1)
	statuses, ok := judgeAcceptanceHeuristic("implement snake game", plan, sess, nil, mech)
	if !ok || !statuses[0].Met {
		t.Fatalf("expected pass with search+list exploration, statuses=%+v ok=%v", statuses, ok)
	}
}

func TestMechanicalCrossCheckRejectsBuildWithoutWorkspaceOK(t *testing.T) {
	statuses := []CriterionStatus{{Index: 0, Text: "Project builds (go build ./...)", Met: true, Evidence: "llm said ok"}}
	mech := mechanicalVerifyResult{WorkspaceOK: false, CustomOK: true}
	out := applyMechanicalCrossCheck(statuses, AcceptancePlan{}, mech, session.New())
	if out[0].Met {
		t.Fatal("expected build criterion rejected when workspace verify failed")
	}
}

func TestHeuristicCriterionBuildUsesBuildOK(t *testing.T) {
	plan := AcceptancePlan{Criteria: []string{"Project builds (go build ./...) when go.mod is present"}}
	mech := mechanicalVerifyResult{BuildOK: true, CustomOK: true}
	statuses, ok := judgeAcceptanceHeuristic("x", plan, session.New(), nil, mech)
	if !ok || len(statuses) != 1 || !statuses[0].Met {
		t.Fatalf("expected build met via BuildOK, got ok=%v statuses=%+v", ok, statuses)
	}
}

func TestAllCriteriaMet(t *testing.T) {
	if allCriteriaMet([]CriterionStatus{{Met: true}, {Met: false}}) {
		t.Fatal("expected false")
	}
	if !allCriteriaMet([]CriterionStatus{{Met: true}, {Met: true}}) {
		t.Fatal("expected true")
	}
}

func TestScaleSubAgentLimits(t *testing.T) {
	turns, timeout := scaleSubAgentLimits(Task{ID: "1", Description: repeatString("x", 250)}, 50, defaultScaledTimeout())
	if turns < 60 {
		t.Fatalf("expected scaled turns >= 60, got %d", turns)
	}
	if timeout < 3*time.Minute {
		t.Fatalf("expected longer timeout, got %v", timeout)
	}
}

func sessionWithTools(writes, reads, searches, lists int) *session.Session {
	s := session.New()
	for i := 0; i < writes; i++ {
		s.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "write_file"}}})
	}
	for i := 0; i < reads; i++ {
		s.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "read_file"}}})
	}
	for i := 0; i < searches; i++ {
		s.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "search_code"}}})
	}
	for i := 0; i < lists; i++ {
		s.Add(session.Entry{Role: "assistant", AssistantToolCalls: []session.ToolCallData{{Name: "list_dir"}}})
	}
	return s
}

func repeatString(s string, n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = s[0]
	}
	return string(out)
}

func defaultScaledTimeout() time.Duration {
	return DefaultDispatcherOptions().SubAgentTimeout
}
