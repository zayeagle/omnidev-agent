package agent

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestContextManagerUsagePercent(t *testing.T) {
	cm := NewContextManager(nil, 1000, 0.95, "")
	entries := []session.Entry{{Content: stringsRepeat("a", 3000)}}
	pct := cm.UsagePercent(entries)
	if pct < 99 {
		t.Fatalf("expected high usage, got %.1f", pct)
	}
	if !cm.ShouldSummarize(entries) {
		t.Fatal("expected ShouldSummarize true at ~100%")
	}
}

func TestAgentContextUsagePct(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	cm := NewContextManager(nil, 1000, 0.95, "")
	a.SetContextManager(cm)
	a.session.AddWithState("user", stringsRepeat("x", 1500), "Thinking", 0)
	pct := a.ContextUsagePct()
	if pct < 40 {
		t.Fatalf("expected meaningful usage pct, got %.1f", pct)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
