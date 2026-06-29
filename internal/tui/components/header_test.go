package components

import (
	"strings"
	"testing"
)

func TestAgentHeader_IncludesRequiredFields(t *testing.T) {
	out := AgentHeader(HeaderInfo{Version: "0.0.1", BuildTime: "2026-06-26 22:30:45"}, 80)
	for _, want := range []string{
		"omnidev-agent",
		agentTagline,
		"Version v0.0.1",
		"Built 2026-06-26 22:30:45",
		"Commands:",
		"/help",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("header missing %q:\n%s", want, out)
		}
	}
}

func TestAgentHeader_WrapsAtNarrowWidth(t *testing.T) {
	out := AgentHeader(HeaderInfo{Version: "0.0.1", BuildTime: "2026-06-26 22:30:45"}, 40)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("expected wrapped header (>5 lines) at width 40, got %d lines:\n%s", len(lines), out)
	}
}

func TestHeaderLineCount_ScalesWithWidth(t *testing.T) {
	info := HeaderInfo{Version: "0.0.1", BuildTime: "2026-06-26 22:30:45"}
	wide := HeaderLineCount(info, 120)
	narrow := HeaderLineCount(info, 40)
	if narrow <= wide {
		t.Fatalf("narrow header lines %d should exceed wide %d", narrow, wide)
	}
}

func TestFooterBar_Wraps(t *testing.T) {
	out := FooterBar(30, "deepseek/deepseek-v4-pro", 0.1, "PgUp/PgDn scroll", "yolo")
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected wrapped footer at width 30:\n%s", out)
	}
}

func TestFooterExitHint(t *testing.T) {
	out := FooterExitHint(80)
	if !strings.Contains(out, "quit") || !strings.Contains(out, "Ctrl+C") || !strings.Contains(out, "Ctrl+Y") {
		t.Fatalf("exit hint missing quit/Ctrl+C/Ctrl+Y:\n%s", out)
	}
}
