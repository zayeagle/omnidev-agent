package config

import (
	"time"

	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// LLMRetryConfig builds stream retry settings from config (with defaults).
func (c *Config) LLMRetryConfig() stream.RetryConfig {
	maxRetries := c.LLMMaxRetries
	if maxRetries < 0 {
		maxRetries = Default().LLMMaxRetries
	}
	secs := c.LLMRetryBackoffSec
	if len(secs) == 0 {
		secs = Default().LLMRetryBackoffSec
	}
	backoffs := make([]time.Duration, len(secs))
	for i, s := range secs {
		if s < 0 {
			s = 0
		}
		backoffs[i] = time.Duration(s) * time.Second
	}
	return stream.RetryConfig{MaxRetries: maxRetries, Backoffs: backoffs}
}

// EffectiveMaxConsecutiveToolDenials returns the denial abort threshold (0 = never abort).
func (c *Config) EffectiveMaxConsecutiveToolDenials() int {
	if c.MaxConsecutiveToolDenials >= 0 {
		return c.MaxConsecutiveToolDenials
	}
	return Default().MaxConsecutiveToolDenials
}
