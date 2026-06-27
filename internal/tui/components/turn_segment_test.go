package components

import (
	"strings"
	"testing"
)

func TestTurnStatusAndLLMSegments(t *testing.T) {
	tn := NewTurn(1, "hello")
	tn.AddStatusLine("Architecture: minimal — prefer a single file.")
	tn.AppendLLM("我来帮你实现")
	tn.FlushLLM("贪吃蛇游戏。")

	if len(tn.statusLines) != 1 {
		t.Fatalf("statusLines = %d, want 1", len(tn.statusLines))
	}
	out := tn.llmOutput.String()
	if out == "" {
		t.Fatal("expected llm output")
	}
	if !containsSubstring(out, "我来帮你") {
		t.Fatalf("llm output missing text: %q", out)
	}
}

func TestTurnContentGapAfterStatus(t *testing.T) {
	tn := NewTurn(1, "hello")
	tn.AddStatusLine("Project workspace ready: deliverables/snake-game")
	tn.AppendLLM("First line.")
	tn.FlushLLM("")

	out := tn.llmOutput.String()
	if out == "" || out[0] == 'P' {
		t.Fatalf("llm should not merge into status line: %q", out)
	}
}

func TestTurnCommandOutputNotInThinking(t *testing.T) {
	tn := NewTurn(1, "/sessions")
	tn.SetCommandOutput("Archived sessions (newest first):\n\n  1. [session] foo.md")
	body := joinLines(tn.render(80, false))
	if strings.Contains(body, "Thinking") {
		t.Fatalf("command output should not render as Thinking:\n%s", body)
	}
	if !strings.Contains(body, "Archived sessions") {
		t.Fatalf("command output missing from render:\n%s", body)
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
