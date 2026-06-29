package stream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// RetryConfig controls LLM API retry with exponential backoff.
type RetryConfig struct {
	MaxRetries int               // retries after the first attempt (default 3)
	Backoffs   []time.Duration   // wait before each retry (default 1s, 2s, 4s)
}

// DefaultRetryConfig returns built-in LLM retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		Backoffs:   []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
	}
}

func (rc RetryConfig) normalized() RetryConfig {
	out := rc
	if out.MaxRetries < 0 {
		out.MaxRetries = DefaultRetryConfig().MaxRetries
	}
	if len(out.Backoffs) == 0 {
		out.Backoffs = DefaultRetryConfig().Backoffs
	}
	return out
}

func backoffAt(backoffs []time.Duration, attempt int) time.Duration {
	if attempt < len(backoffs) {
		return backoffs[attempt]
	}
	return backoffs[len(backoffs)-1]
}

// RetryChat calls provider.Chat with exponential backoff retry.
func RetryChat(ctx context.Context, provider llm.Provider, req *llm.Request, cfg RetryConfig) (*llm.Response, error) {
	cfg = cfg.normalized()

	var lastErr error
	for i := 0; i <= cfg.MaxRetries; i++ {
		resp, err := provider.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Don't retry if context is cancelled
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		// Client errors (4xx) won't succeed on retry.
		if isNonRetryableLLMError(err) {
			return nil, err
		}

		if i < cfg.MaxRetries {
			select {
			case <-time.After(backoffAt(cfg.Backoffs, i)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	attempts := cfg.MaxRetries + 1
	return nil, fmt.Errorf("llm: failed after %d attempts: %w", attempts, lastErr)
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
