package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/config"
)

// TestConfigLayeredMerge verifies the priority chain: CLI > env > project > global > defaults.
func TestConfigLayeredMerge(t *testing.T) {
	// Create a temp global config
	tmpDir := t.TempDir()
	globalConf := filepath.Join(tmpDir, "global.json")
	os.WriteFile(globalConf, []byte(`{"model": "gpt-4", "timeout": 30}`), 0644)

	// Create a temp project config
	projectConf := filepath.Join(tmpDir, "project.json")
	os.WriteFile(projectConf, []byte(`{"model": "gpt-3.5", "log_level": "debug"}`), 0644)

	// Test: project overrides global
	cfg, err := config.LoadWithLayers(config.LoadOptions{
		ProjectConfigPath: projectConf,
		GlobalConfigPath:  globalConf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "gpt-3.5" {
		t.Errorf("expected project override (gpt-3.5), got %s", cfg.Model)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected global fallback (30), got %d", cfg.Timeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level from project (debug), got %s", cfg.LogLevel)
	}
}

// TestConfigEnvOverride verifies environment variable overrides.
func TestConfigEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	projectConf := filepath.Join(tmpDir, "project.json")
	os.WriteFile(projectConf, []byte(`{"model": "gpt-3.5"}`), 0644)

	// Set env var
	os.Setenv("OMNIDEV_MODEL", "deepseek-chat")
	defer os.Unsetenv("OMNIDEV_MODEL")

	cfg, err := config.LoadWithLayers(config.LoadOptions{
		ProjectConfigPath: projectConf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "deepseek-chat" {
		t.Errorf("expected env override (deepseek-chat), got %s", cfg.Model)
	}
}

// TestConfigCLIOverride verifies explicit options beat everything.
func TestConfigCLIOverride(t *testing.T) {
	tmpDir := t.TempDir()
	projectConf := filepath.Join(tmpDir, "project.json")
	os.WriteFile(projectConf, []byte(`{"model": "gpt-3.5"}`), 0644)

	os.Setenv("OMNIDEV_MODEL", "deepseek-chat")
	defer os.Unsetenv("OMNIDEV_MODEL")

	cfg, err := config.LoadWithLayers(config.LoadOptions{
		ProjectConfigPath: projectConf,
		Model:             "gpt-4o", // CLI flag beats env
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("expected CLI override (gpt-4o), got %s", cfg.Model)
	}
}

// TestConfigDefaults verifies the Default() values.
func TestConfigDefaults(t *testing.T) {
	cfg := config.Default()
	if cfg.Provider != "openai" {
		t.Errorf("expected default provider openai, got %s", cfg.Provider)
	}
	if cfg.Timeout != 0 {
		t.Errorf("expected default timeout 0, got %d", cfg.Timeout)
	}
	if cfg.MaxTurns != 20 {
		t.Errorf("expected default maxTurns 20, got %d", cfg.MaxTurns)
	}
	if cfg.MaxParallel != 2 {
		t.Errorf("expected default max_parallel 2, got %d", cfg.MaxParallel)
	}
	if cfg.SubAgentTimeout != 120 {
		t.Errorf("expected default sub_agent_timeout 120, got %d", cfg.SubAgentTimeout)
	}
	if cfg.SubAgentMaxTurns != 10 {
		t.Errorf("expected default sub_agent_max_turns 10, got %d", cfg.SubAgentMaxTurns)
	}
}

// TestConfigMustMatchProvider validates provider name normalization.
func TestConfigMustMatchProvider(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"openai", "openai"},
		{"OpenAI", "openai"},
		{"deepseek", "deepseek"},
		{"DeepSeek", "deepseek"},
		{"anthropic", "anthropic"},
		{"claude", "anthropic"},
		{"strict", "strict"},
		{"custom-gateway", "openai"},
		{"", "openai"},
	}
	for _, tc := range tests {
		got := config.MustMatchProvider(tc.input)
		if got != tc.expected {
			t.Errorf("MustMatchProvider(%q): expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}
