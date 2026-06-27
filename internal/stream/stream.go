package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// RetryChat calls provider.Chat with exponential backoff retry.
// Max 3 retries: 1s, 2s, 4s.
func RetryChat(ctx context.Context, provider llm.Provider, req *llm.Request) (*llm.Response, error) {
	const maxRetries = 3
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		resp, err := provider.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Don't retry if context is cancelled
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if i < maxRetries {
			select {
			case <-time.After(backoffs[i]):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("llm: failed after %d retries: %w", maxRetries+1, lastErr)
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
