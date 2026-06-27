package components

import (
	"strings"
	"testing"
)

func TestAgentHeader_IncludesMetadata(t *testing.T) {
	out := AgentHeader(HeaderInfo{Version: "abc123", BuildTime: "2026-06-26 22:30:45"})
	for _, want := range []string{"omnidev-agent", "Version vabc123", "Built 2026-06-26 22:30:45", "/help"} {
		if !strings.Contains(out, want) {
			t.Fatalf("header missing %q:\n%s", want, out)
		}
	}
}

func TestAgentHeaderCompact_OmitsTagline(t *testing.T) {
	out := AgentHeaderCompact(HeaderInfo{Version: "abc123", BuildTime: "2026-06-26 22:30:45"})
	if strings.Contains(out, agentTagline) {
		t.Fatal("compact header should omit tagline")
	}
}
