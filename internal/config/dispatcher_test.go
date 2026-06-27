package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDispatcherConfigFileAndEnv(t *testing.T) {
	tmpDir := t.TempDir()
	projectConf := filepath.Join(tmpDir, "project.json")
	if err := os.WriteFile(projectConf, []byte(`{
		"max_parallel": 3,
		"sub_agent_timeout": 90,
		"sub_agent_max_turns": 8
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithLayers(configLoadOpts(projectConf))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxParallel != 3 || cfg.SubAgentTimeout != 90 || cfg.SubAgentMaxTurns != 8 {
		t.Fatalf("file merge: got parallel=%d timeout=%d turns=%d", cfg.MaxParallel, cfg.SubAgentTimeout, cfg.SubAgentMaxTurns)
	}

	os.Setenv("OMNIDEV_MAX_PARALLEL", "1")
	os.Setenv("OMNIDEV_SUB_AGENT_TIMEOUT", "60")
	os.Setenv("OMNIDEV_SUB_AGENT_MAX_TURNS", "5")
	t.Cleanup(func() {
		os.Unsetenv("OMNIDEV_MAX_PARALLEL")
		os.Unsetenv("OMNIDEV_SUB_AGENT_TIMEOUT")
		os.Unsetenv("OMNIDEV_SUB_AGENT_MAX_TURNS")
	})

	cfg, err = LoadWithLayers(configLoadOpts(projectConf))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxParallel != 1 || cfg.SubAgentTimeout != 60 || cfg.SubAgentMaxTurns != 5 {
		t.Fatalf("env override: got parallel=%d timeout=%d turns=%d", cfg.MaxParallel, cfg.SubAgentTimeout, cfg.SubAgentMaxTurns)
	}
}

func configLoadOpts(projectPath string) LoadOptions {
	return LoadOptions{ProjectConfigPath: projectPath}
}
