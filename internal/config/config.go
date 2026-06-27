package config

type Config struct {
	Provider    string  `json:"provider"`    // openai | deepseek | anthropic
	BaseURL     string  `json:"base_url"`    // https://api.openai.com/v1
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	CompatMode  string  `json:"compat_mode"` // auto | openai | strict
	Timeout     int     `json:"timeout"`
	MaxTurns    int     `json:"max_turns"`
	LogLevel    string  `json:"log_level"`
	SessionDir  string  `json:"session_dir"` // omnidev-agent runtime snapshots → .ai_history/sessions/
	LogDir      string  `json:"log_dir"`     // deprecated alias for session_dir
	// Context window management (Cursor-style)
	ContextMaxTokens          int     `json:"context_max_tokens"`          // 120000 default
	ContextSummarizeThreshold float64 `json:"context_summarize_threshold"` // 0.95 default
}

// Load is the legacy entry point used by tests and simple setups.
// It calls LoadWithLayers with no overrides.
func Load(paths ...string) (*Config, error) {
	opts := LoadOptions{}
	for _, p := range paths {
		if opts.ProjectConfigPath == "" {
			opts.ProjectConfigPath = p
		} else if opts.GlobalConfigPath == "" {
			opts.GlobalConfigPath = p
		}
	}
	return LoadWithLayers(opts)
}

func Default() *Config {
	return &Config{
		Provider: "openai",
		BaseURL:  "https://api.openai.com/v1",
		Model:    "gpt-4",
		Timeout:  0,
		MaxTurns: 20,
		LogLevel: "info",
		SessionDir:                 ".ai_history/sessions/",
		LogDir:                     ".ai_history/sessions/", // legacy alias
		ContextMaxTokens:           120000,
		ContextSummarizeThreshold:  0.95,
	}
}

func (c *Config) Save(path string) error {
	// TODO: implement config persistence
	return nil
}

// RuntimeSessionDir returns where omnidev-agent stores its own TUI/headless session snapshots.
// Collaboration logs for Cursor/Codex belong in .ai_history/logs/ (see AGENTS.md), not here.
func (c *Config) RuntimeSessionDir() string {
	if c.SessionDir != "" {
		return c.SessionDir
	}
	if c.LogDir != "" {
		return c.LogDir
	}
	return ".ai_history/sessions/"
}
