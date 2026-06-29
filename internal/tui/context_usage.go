package tui

import (
	"github.com/zayeagle/omnidev-agent/internal/agent"
)

// contextUsagePct returns context window fill % (same estimator/threshold as compaction).
func contextUsagePct(a *agent.Agent) float64 {
	if a == nil {
		return 0
	}
	return a.ContextUsagePct()
}
