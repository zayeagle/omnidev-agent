package stream

import (
	"context"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// RetryChat calls provider.Chat with backoff; network errors retry until ctx cancel when enabled.
func RetryChat(ctx context.Context, provider llm.Provider, req *llm.Request, cfg RetryConfig) (*llm.Response, error) {
	return retryLLM(ctx, cfg, func() (*llm.Response, error) {
		return provider.Chat(ctx, req)
	})
}

func isNonRetryableLLMError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "llm: 400") ||
		strings.Contains(msg, "llm: 401") ||
		strings.Contains(msg, "llm: 403") ||
		strings.Contains(msg, "llm: 404") ||
		strings.Contains(msg, "openai: 400") ||
		strings.Contains(msg, "openai: 401") ||
		strings.Contains(msg, "openai: 403") ||
		strings.Contains(msg, "openai: 404")
}

// SSEParser holds state for parsing Server-Sent Events chunks.
// OpenAI-compatible SSE format: "data: {...}\n\n"
type SSEParser struct {
	buf []byte // leftover from previous partial line
}

// ParseSSE accepts raw bytes and returns complete JSON payloads.
// Incomplete lines are buffered internally for the next call.
// Returns nil slice when no complete event is available.
func (p *SSEParser) ParseSSE(raw []byte) [][]byte {
	p.buf = append(p.buf, raw...)
	var events [][]byte

	for {
		idx := findDoubleNewline(p.buf)
		if idx < 0 {
			break
		}
		line := p.buf[:idx]
		p.buf = p.buf[idx+2:] // skip "\n\n"

		data := extractDataField(line)
		if data == nil {
			continue
		}
		if string(data) == "[DONE]" {
			events = append(events, []byte("__DONE__"))
			continue
		}
		events = append(events, data)
	}
	return events
}

// findDoubleNewline returns the index of "\n\n" or -1.
func findDoubleNewline(p []byte) int {
	for i := 0; i < len(p)-1; i++ {
		if p[i] == '\n' && p[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// extractDataField extracts the JSON payload after "data: " prefix.
func extractDataField(line []byte) []byte {
	const prefix = "data: "
	for i := 0; i <= len(line)-len(prefix); i++ {
		if string(line[i:i+len(prefix)]) == prefix {
			payload := line[i+len(prefix):]
			for len(payload) > 0 && (payload[len(payload)-1] == '\n' || payload[len(payload)-1] == '\r' || payload[len(payload)-1] == ' ') {
				payload = payload[:len(payload)-1]
			}
			return payload
		}
	}
	return nil
}

// Reset clears the internal buffer.
func (p *SSEParser) Reset() {
	p.buf = p.buf[:0]
}
