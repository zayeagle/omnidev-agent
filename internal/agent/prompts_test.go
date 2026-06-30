package agent

import (
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func TestBuildMessagesIncludesCodeExplorationGuidance(t *testing.T) {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	a := New(llm.NewMockProvider([]*llm.Response{{Content: "ok"}}), permissions.NewChecker(false), toolbox, session.New())
	msgs := a.buildMessages()
	if len(msgs) == 0 || msgs[0].Role != "system" {
		t.Fatal("expected system message first")
	}
	if !strings.Contains(msgs[0].Content, "search_code") || !strings.Contains(msgs[0].Content, "offset") {
		t.Fatalf("system prompt missing code exploration guidance: %q", msgs[0].Content)
	}
}
