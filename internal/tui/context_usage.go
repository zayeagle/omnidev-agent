package tui

import (
	"github.com/zayeagle/omnidev-agent/internal/agent"
)

// contextWindowChars approximates a 128k-token context window (~4 chars/token).
const contextWindowChars = 512_000

// contextUsagePct estimates how much of the context window the session consumes.
func contextUsagePct(a *agent.Agent) float64 {
	if a == nil || a.Session() == nil {
		return 0
	}
	total := 0
	for _, e := range a.Session().Entries {
		total += len(e.Content)
	}
	pct := float64(total) / contextWindowChars * 100
	if pct > 99.9 {
		return 99.9
	}
	if pct < 0.1 && total > 0 {
		return 0.1
	}
	return pct
}
