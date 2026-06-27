package agent

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// Sub-agents must not inject the user turn before RunLoop; duplicate consecutive
// user messages break strict gateway requests (400 bad_request).
func TestSubAgentBuildMessagesSingleUserTurn(t *testing.T) {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	mock := llm.NewMockProvider([]*llm.Response{{Content: "ok"}})
	a := New(mock, permissions.NewChecker(false), toolbox, session.New())
	a.SetOutputDir("deliverables/snake-game")
	a.SetProjectLayout(LayoutMinimal)
	a.subAgent = true

	// RunLoop adds exactly one user entry at start.
	a.session.AddWithState("user", "Implement the snake game", StateThinking.String(), 0)

	msgs := a.buildMessages()
	userCount := 0
	for _, m := range msgs {
		if m.Role == "user" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("expected 1 user message in sub-agent session, got %d", userCount)
	}
}
