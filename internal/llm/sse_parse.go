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
		if err := extractSSEError(body); err != nil {
			return nil, err
		}
		resp, err := parseSSEChatCompletion(body)
		if err != nil {
			return nil, err
		}
		return resp, nil
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

		if err := parseAPIErrorJSON(data); err != nil {
			return nil, err
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

type apiErrorBody struct {
	Error *apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func parseAPIErrorJSON(data []byte) error {
	var body apiErrorBody
	if err := json.Unmarshal(data, &body); err != nil || body.Error == nil {
		return nil
	}
	e := body.Error
	msg := e.Message
	if e.Type != "" {
		if msg != "" {
			msg += " (" + e.Type + ")"
		} else {
			msg = e.Type
		}
	}
	if msg == "" {
		msg = "unknown error"
	}
	if e.Code != "" {
		return fmt.Errorf("llm: %s %s", e.Code, msg)
	}
	return fmt.Errorf("llm: %s", msg)
}

func extractSSEError(body []byte) error {
	var found error
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		if err := parseAPIErrorJSON(data); err != nil {
			found = err
		}
	}
	return found
}

func truncateBody(body []byte) string {
	const limit = 200
	s := strings.TrimSpace(string(body))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
