package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// parseChatCompletionBody parses a chat/completions HTTP body as JSON or SSE.
func parseChatCompletionBody(body []byte) (*openAIResponse, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	if body[0] == '{' || body[0] == '[' {
		var result openAIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("unmarshal: %w (body: %s)", err, truncateBody(body))
		}
		return &result, nil
	}

	if looksLikeSSE(body) {
		if resp, err := parseSSEChatCompletion(body); err == nil && resp != nil {
			return resp, nil
		}
	}

	// Plain-text fallback (some gateways return raw assistant text).
	text := string(body)
	return &openAIResponse{
		Choices: []openAIChoice{{
			Message: openAIMessage{
				Role:    "assistant",
				Content: strPtr(text),
			},
			FinishReason: "stop",
		}},
	}, nil
}

func looksLikeSSE(body []byte) bool {
	return bytes.Contains(body, []byte("data:"))
}

func parseSSEChatCompletion(body []byte) (*openAIResponse, error) {
	var content strings.Builder
	var finishReason string
	var toolCalls []openAIToolCall
	toolArgBuf := map[int]*strings.Builder{}
	toolMeta := map[int]openAIToolCall{}

	lines := bytes.Split(body, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				content.WriteString(*choice.Delta.Content)
			}
			if choice.Message.Content != nil && *choice.Message.Content != "" {
				content.WriteString(*choice.Message.Content)
			}
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				meta := toolMeta[idx]
				if tc.ID != "" {
					meta.ID = tc.ID
				}
				if tc.Type != "" {
					meta.Type = tc.Type
				}
				if tc.Function.Name != "" {
					meta.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					if toolArgBuf[idx] == nil {
						toolArgBuf[idx] = &strings.Builder{}
					}
					toolArgBuf[idx].WriteString(tc.Function.Arguments)
				}
				toolMeta[idx] = meta
			}
			if len(choice.Message.ToolCalls) > 0 {
				toolCalls = append(toolCalls, choice.Message.ToolCalls...)
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}

	if len(toolMeta) > 0 {
		for idx := 0; idx <= maxToolIndex(toolMeta); idx++ {
			meta, ok := toolMeta[idx]
			if !ok {
				continue
			}
			if buf := toolArgBuf[idx]; buf != nil {
				meta.Function.Arguments = buf.String()
			}
			if meta.Type == "" {
				meta.Type = "function"
			}
			toolCalls = append(toolCalls, meta)
		}
	}

	text := content.String()
	if text == "" && len(toolCalls) == 0 {
		return nil, fmt.Errorf("sse response contained no content (body: %s)", truncateBody(body))
	}

	msg := openAIMessage{Role: "assistant", ToolCalls: toolCalls}
	if text != "" {
		msg.Content = strPtr(text)
	}
	if finishReason == "" && len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if finishReason == "" {
		finishReason = "stop"
	}

	return &openAIResponse{
		Choices: []openAIChoice{{
			Message:      msg,
			FinishReason: finishReason,
		}},
	}, nil
}

func maxToolIndex(m map[int]openAIToolCall) int {
	max := 0
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}

func strPtr(s string) *string { return &s }

func truncateBody(body []byte) string {
	const limit = 200
	s := strings.TrimSpace(string(body))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
