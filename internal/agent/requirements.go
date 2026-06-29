package agent

import (
	"context"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

const requirementsPrompt = `You are a senior engineer analyzing a development request before implementation.

Output a concise analysis in the same language as the user (Chinese if the user wrote in Chinese). Use this structure:

Goal: (one sentence)
Acceptance: (2-4 bullet points)
Scope: (likely files/modules)
Risks: (1-2 items or "none")

Keep under 200 words. No code.`

// analyzeRequirements runs a lightweight LLM pass before task decomposition.
func (a *Agent) analyzeRequirements(ctx context.Context, instruction string) string {
	messages := []llm.Message{
		{Role: "system", Content: requirementsPrompt},
		{Role: "user", Content: instruction},
	}
	resp, err := stream.RetryChat(ctx, a.provider, &llm.Request{Messages: messages}, a.retryConfig)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return ""
	}
	return "Requirements analysis:\n" + content
}
