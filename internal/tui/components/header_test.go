package components

import (
	"strings"
	"testing"
)

func TestAgentHeader_IncludesRequiredFields(t *testing.T) {
	out := AgentHeader(HeaderInfo{Version: "abc123", BuildTime: "2026-06-26 22:30:45"})
	for _, want := range []string{
		"omnidev-agent",
		agentTagline,
		"Version vabc123",
		"Built 2026-06-26 22:30:45",
		"Commands:",
		"/help",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("header missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Examples") || strings.Contains(out, "Type a message") {
		t.Fatalf("header should not duplicate welcome/examples text:\n%s", out)
	}
}

func TestHeaderLineCount(t *testing.T) {
	if HeaderLineCount() != 5 {
		t.Fatalf("HeaderLineCount = %d, want 5", HeaderLineCount())
	}
}
