package config

import "testing"

func TestEffectiveContextSettings(t *testing.T) {
	cfg := Default()
	if cfg.EffectiveContextMaxTokens() != 120000 {
		t.Fatalf("max tokens: got %d", cfg.EffectiveContextMaxTokens())
	}
	if cfg.EffectiveContextSummarizeThreshold() != 0.95 {
		t.Fatalf("threshold: got %v", cfg.EffectiveContextSummarizeThreshold())
	}

	cfg.ContextSummarizeThreshold = 0.80
	if cfg.EffectiveContextSummarizeThreshold() != 0.80 {
		t.Fatalf("override threshold: got %v", cfg.EffectiveContextSummarizeThreshold())
	}

	cfg.ContextSummarizeThreshold = 0
	if cfg.EffectiveContextSummarizeThreshold() != 0.95 {
		t.Fatalf("zero should fall back to default: got %v", cfg.EffectiveContextSummarizeThreshold())
	}
}
