package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// ReconnectHook is called before each retry wait (optional TUI / logging).
type ReconnectHook func(attempt int, err error, nextWait time.Duration, persistent bool)

// RetryConfig controls LLM API retry with exponential backoff.
type RetryConfig struct {
	MaxRetries            int             // retries after the first attempt for non-network errors (default 3)
	Backoffs              []time.Duration // wait before each retry (default 1s, 2s, 4s, then last value)
	PersistNetworkRetry   bool            // when true (default), network errors retry until success or ctx cancel
	OnReconnect           ReconnectHook   // optional status callback (not loaded from JSON)
}

// DefaultRetryConfig returns built-in LLM retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:          3,
		Backoffs:            []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
		PersistNetworkRetry: true,
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

type llmCall func() (*llm.Response, error)

func retryLLM(ctx context.Context, cfg RetryConfig, call llmCall) (*llm.Response, error) {
	cfg = cfg.normalized()

	var lastErr error
	attempts := 0
	for {
		attempts++
		resp, err := call()
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		if isNonRetryableLLMError(err) {
			return nil, err
		}

		persistent := cfg.PersistNetworkRetry && isNetworkError(err)
		if !persistent {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if attempts >= cfg.MaxRetries+1 {
				break
			}
		}

		wait := backoffAt(cfg.Backoffs, attempts-1)
		if cfg.OnReconnect != nil {
			cfg.OnReconnect(attempts, err, wait, persistent)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("llm: failed after %d attempts: %w", attempts, lastErr)
}
