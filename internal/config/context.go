package config

// EffectiveContextMaxTokens returns the context window cap (default 120000).
func (c *Config) EffectiveContextMaxTokens() int {
	if c.ContextMaxTokens > 0 {
		return c.ContextMaxTokens
	}
	return Default().ContextMaxTokens
}

// EffectiveContextSummarizeThreshold returns the fraction of max tokens at which
// early history is summarized (default 0.95 = 95%).
func (c *Config) EffectiveContextSummarizeThreshold() float64 {
	if c.ContextSummarizeThreshold > 0 && c.ContextSummarizeThreshold <= 1 {
		return c.ContextSummarizeThreshold
	}
	return Default().ContextSummarizeThreshold
}
