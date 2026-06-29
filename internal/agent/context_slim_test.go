package agent

import (
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestSlimWriteArguments(t *testing.T) {
	args := map[string]interface{}{
		"path":    "main.go",
		"content": strings.Repeat("x", 5000),
	}
	slim := SlimToolArguments("write_file", args)
	c, ok := slim["content"].(string)
	if !ok || !strings.Contains(c, "omitted") {
		t.Fatalf("expected omitted content ref, got %v", slim["content"])
	}
	if slim["path"] != "main.go" {
		t.Fatal("path should remain")
	}
}

func TestSlimToolResultForHistory(t *testing.T) {
	got := SlimToolResultForHistory("read_file", "[PARTIAL read_file: 100 chars → 50 inline | full: /tmp/spool.txt]\n---\nbody")
	if !strings.Contains(got, "archived") || !strings.Contains(got, "spool") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestCompressGuardAnalysis(t *testing.T) {
	raw := "[PROJECT ANALYSIS]\n" + strings.Repeat("line\n", 500)
	got := CompressGuardAnalysis(raw, 500)
	if len(got) > 600 {
		t.Fatalf("too long: %d", len(got))
	}
	if !strings.Contains(got, "PROJECT ANALYSIS") {
		t.Fatal("missing header")
	}
}

func TestBuildMessagesSlimsOldToolResults(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	a.SetContextSlimOptions(ContextSlimOptions{ToolResultsKeepFull: 1, GuardAnalysisMax: 4000})

	a.session.Add(session.Entry{
		Role: "user", Content: "hi",
	})
	a.addAssistantWithToolCalls("", []llm.ToolCall{{ID: "c1", Name: "read_file", Arguments: map[string]interface{}{"path": "a.go"}}})
	a.session.Add(session.Entry{
		Role: "tool",
		ToolCalls: []session.ToolCallEntry{{
			ID: "c1", Name: "read_file", Result: strings.Repeat("old ", 2000), Allowed: true,
		}},
	})
	a.addAssistantWithToolCalls("", []llm.ToolCall{{ID: "c2", Name: "read_file", Arguments: map[string]interface{}{"path": "b.go"}}})
	a.session.Add(session.Entry{
		Role: "tool",
		ToolCalls: []session.ToolCallEntry{{
			ID: "c2", Name: "read_file", Result: "fresh content", Allowed: true,
		}},
	})

	msgs := a.buildMessages()
	var oldBody, newBody string
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "c1" {
			oldBody = m.Content
		}
		if m.Role == "tool" && m.ToolCallID == "c2" {
			newBody = m.Content
		}
	}
	if !strings.Contains(oldBody, "archived") {
		t.Fatalf("old tool should be archived, got %q", oldBody[:min(80, len(oldBody))])
	}
	if newBody != "fresh content" {
		t.Fatalf("recent tool should be full, got %q", newBody)
	}
}

func TestLooksSimpleTask(t *testing.T) {
	if !looksSimpleTask("fix the bug in main.go") {
		t.Fatal("expected simple")
	}
	if looksSimpleTask("first implement A and then implement B with tests") {
		t.Fatal("expected multi-step")
	}
}

func TestEstimateEntryTokensIncludesAssistantArgs(t *testing.T) {
	e := session.Entry{
		Role: "assistant",
		AssistantToolCalls: []session.ToolCallData{{
			Name: "write_file",
			Arguments: map[string]interface{}{
				"path":    "x.go",
				"content": strings.Repeat("a", 4000),
			},
		}},
	}
	slim := estimateEntryTokens(session.Entry{
		Role: "assistant",
		AssistantToolCalls: []session.ToolCallData{{
			Name:      "write_file",
			Arguments: SlimToolArguments("write_file", e.AssistantToolCalls[0].Arguments),
		}},
	})
	full := estimateEntryTokens(e)
	if slim >= full {
		t.Fatalf("slim %d should be less than full %d", slim, full)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
