package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/stream"
)

func TestLLMRetryConfigFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	projectConf := filepath.Join(tmpDir, "project.json")
	if err := os.WriteFile(projectConf, []byte(`{"llm_max_retries": 2}`), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("OMNIDEV_LLM_MAX_RETRIES", "1")
	os.Setenv("OMNIDEV_LLM_RETRY_BACKOFF_SEC", "5,10")
	t.Cleanup(func() {
		os.Unsetenv("OMNIDEV_LLM_MAX_RETRIES")
		os.Unsetenv("OMNIDEV_LLM_RETRY_BACKOFF_SEC")
	})

	cfg, err := LoadWithLayers(LoadOptions{ProjectConfigPath: projectConf})
	if err != nil {
		t.Fatal(err)
	}
	rc := cfg.LLMRetryConfig()
	if rc.MaxRetries != 1 {
		t.Fatalf("env override max retries: got %d want 1", rc.MaxRetries)
	}
	if len(rc.Backoffs) != 2 || rc.Backoffs[0] != 5*time.Second {
		t.Fatalf("backoffs: %+v", rc.Backoffs)
	}
}

func TestLLMRetryConfigZeroRetries(t *testing.T) {
	cfg := Default()
	cfg.LLMMaxRetries = 0
	rc := cfg.LLMRetryConfig()
	if rc.MaxRetries != 0 {
		t.Fatalf("expected 0 retries, got %d", rc.MaxRetries)
	}
	_ = stream.RetryConfig(rc) // compile-time shape check
}
