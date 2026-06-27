package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// LoadWithLayers merges configuration from multiple sources with this priority
// (highest to lowest):
//
//	CLI flags > environment variables > project config file > global config file > defaults
func LoadWithLayers(opts LoadOptions) (*Config, error) {
	cfg := Default()

	// Layer 5: defaults (already set via Default())

	// Layer 4: global config file (~/.omnidev-agent/config.json)
	if opts.GlobalConfigPath != "" {
		mergeFile(cfg, opts.GlobalConfigPath)
	}

	// Layer 3: project config file (./.omnidev-agent.json)
	if opts.ProjectConfigPath != "" {
		mergeFile(cfg, opts.ProjectConfigPath)
	}

	// Layer 2: environment variables
	mergeEnv(cfg)

	// Layer 1: CLI flags / explicit overrides
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	if opts.BaseURL != "" {
		cfg.BaseURL = opts.BaseURL
	}
	if opts.APIKey != "" {
		cfg.APIKey = opts.APIKey
	}
	if opts.Model != "" {
		cfg.Model = opts.Model
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.LogLevel != "" {
		cfg.LogLevel = opts.LogLevel
	}
	if opts.MaxTurns > 0 {
		cfg.MaxTurns = opts.MaxTurns
	}
	if opts.LogDir != "" {
		cfg.LogDir = opts.LogDir
	}
	if opts.SessionDir != "" {
		cfg.SessionDir = opts.SessionDir
	}
	if opts.ContextMaxTokens > 0 {
		cfg.ContextMaxTokens = opts.ContextMaxTokens
	}
	if opts.ContextSummarizeThreshold > 0 {
		cfg.ContextSummarizeThreshold = opts.ContextSummarizeThreshold
	}

	return cfg, nil
}

// LoadOptions holds all possible overrides from CLI flags or explicit programmatic input.
type LoadOptions struct {
	Provider          string
	BaseURL           string
	APIKey            string
	Model             string
	Timeout           int
	LogLevel          string
	MaxTurns          int
	LogDir            string
	SessionDir        string
	ProjectConfigPath string
	GlobalConfigPath  string
	ContextMaxTokens          int
	ContextSummarizeThreshold float64
}

// mergeFile reads a JSON config file and applies non-zero values on top of dst.
func mergeFile(dst *Config, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // file not found or unreadable → skip silently
	}
	var src Config
	if err := json.Unmarshal(data, &src); err != nil {
		return
	}
	applyNonZero(dst, &src)
}

// mergeEnv reads env vars and applies them to dst.
func mergeEnv(dst *Config) {
	if v := os.Getenv("OMNIDEV_PROVIDER"); v != "" {
		dst.Provider = v
	}
	if v := os.Getenv("OMNIDEV_BASE_URL"); v != "" {
		dst.BaseURL = v
	}
	if v := os.Getenv("OMNIDEV_API_KEY"); v != "" {
		dst.APIKey = v
	}
	if v := os.Getenv("OMNIDEV_MODEL"); v != "" {
		dst.Model = v
	}
	if v := os.Getenv("OMNIDEV_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.Timeout = n
		}
	}
	if v := os.Getenv("OMNIDEV_LOG_LEVEL"); v != "" {
		dst.LogLevel = v
	}
	if v := os.Getenv("OMNIDEV_MAX_TURNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.MaxTurns = n
		}
	}
	if v := os.Getenv("OMNIDEV_SESSION_DIR"); v != "" {
		dst.SessionDir = v
	}
	if v := os.Getenv("OMNIDEV_LOG_DIR"); v != "" {
		// legacy alias for session_dir (omnidev-agent runtime snapshots)
		dst.LogDir = v
		if dst.SessionDir == "" {
			dst.SessionDir = v
		}
	}
	if v := os.Getenv("OMNIDEV_CONTEXT_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ContextMaxTokens = n
		}
	}
	if v := os.Getenv("OMNIDEV_CONTEXT_SUMMARIZE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
			dst.ContextSummarizeThreshold = f
		}
	}
	if v := os.Getenv("OMNIDEV_COMPAT_MODE"); v != "" {
		dst.CompatMode = v
	}
}

// applyNonZero copies src values to dst only when the src value is non-zero.
func applyNonZero(dst, src *Config) {
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Timeout > 0 {
		dst.Timeout = src.Timeout
	}
	if src.MaxTurns > 0 {
		dst.MaxTurns = src.MaxTurns
	}
	if src.LogLevel != "" {
		dst.LogLevel = src.LogLevel
	}
	if src.LogDir != "" {
		dst.LogDir = src.LogDir
		if dst.SessionDir == "" {
			dst.SessionDir = src.LogDir
		}
	}
	if src.SessionDir != "" {
		dst.SessionDir = src.SessionDir
	}
	if src.Temperature > 0 {
		dst.Temperature = src.Temperature
	}
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.ContextMaxTokens > 0 {
		dst.ContextMaxTokens = src.ContextMaxTokens
	}
	if src.ContextSummarizeThreshold > 0 {
		dst.ContextSummarizeThreshold = src.ContextSummarizeThreshold
	}
	if src.CompatMode != "" {
		dst.CompatMode = src.CompatMode
	}
}

// MustMatchProvider returns the normalized provider name.
func MustMatchProvider(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "openai", "deepseek", "anthropic", "claude", "strict":
		if p == "claude" {
			return "anthropic"
		}
		return p
	default:
		// Unknown names use OpenAI-compatible client with custom base_url.
		return "openai"
	}
}
