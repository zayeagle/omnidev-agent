package agent

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// llmRetryConfig returns retry settings with optional TUI reconnect status lines.
func (a *Agent) llmRetryConfig(msgCh chan<- tea.Msg) stream.RetryConfig {
	cfg := a.retryConfig
	if msgCh == nil {
		return cfg
	}
	cfg.OnReconnect = func(attempt int, err error, nextWait time.Duration, persistent bool) {
		wait := nextWait.Round(time.Second)
		if wait < time.Second {
			wait = time.Second
		}
		msg := fmt.Sprintf("LLM unreachable — retrying in %s (attempt %d)…", wait, attempt)
		if persistent {
			msg = fmt.Sprintf("Network error — auto-reconnecting in %s (attempt %d). Press Ctrl+C to cancel.", wait, attempt)
		}
		emitActivity(msgCh, msg)
		msgCh <- StreamChunkMsg{Content: msg, Done: true}
	}
	return cfg
}
