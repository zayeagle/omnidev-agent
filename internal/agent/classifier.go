package agent

import (
	"context"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// IntentClass categorizes a user's instruction.
type IntentClass string

const (
	// IntentChat — pure conversation, no file modification required.
	IntentChat IntentClass = "chat"

	// IntentCodeMod — the user wants to add, modify, or delete code files.
	IntentCodeMod IntentClass = "code_modification"
)

const classifierPrompt = "Classify the user's intent into exactly one of these two categories:\n\n1. chat — the user is asking a question, having a conversation, or seeking information. No code files need to be read, created, or modified.\n2. code_modification — the user wants to add, modify, delete, or create code files in a project. This includes debugging, refactoring, implementing features, verifying/building existing work, follow-up tasks, or any file-system changes.\n\nReply with ONLY one word: chat or code_modification."

// Classifier uses an LLM call to determine whether a user instruction is
// simple chat or requires code modification.
type Classifier struct {
	provider    llm.Provider
	retryConfig stream.RetryConfig
}

// NewClassifier creates a classifier using the given LLM provider.
func NewClassifier(provider llm.Provider) *Classifier {
	return &Classifier{provider: provider, retryConfig: stream.DefaultRetryConfig()}
}

func (c *Classifier) SetRetryConfig(cfg stream.RetryConfig) { c.retryConfig = cfg }

// Classify sends a fast LLM call to categorize the user intent.
func (c *Classifier) Classify(ctx context.Context, instruction string) IntentClass {
	messages := []llm.Message{
		{Role: "system", Content: classifierPrompt},
		{Role: "user", Content: instruction},
	}

	resp, err := stream.RetryChat(ctx, c.provider, &llm.Request{
		Messages: messages,
	}, c.retryConfig)
	if err != nil {
		if looksLikeSimpleChat(instruction) {
			return IntentChat
		}
		return IntentCodeMod
	}

	content := strings.TrimSpace(strings.ToLower(resp.Content))
	if strings.Contains(content, "chat") && !strings.Contains(content, "code") {
		return IntentChat
	}
	return IntentCodeMod
}
