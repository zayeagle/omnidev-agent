package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/mcp"
)

// LoadWithLayers merges configuration from multiple sources with this priority
// (highest to lowest):
//
//	CLI flags > environment variables > project config file > global config file > defaults
func LoadWithLayers(opts LoadOptions) (*Config, error) {
	cfg := Default()

	// Layer 5: defaults (already set via Default())

	// Layer 4: global config file (<UserHomeDir>/.omnidev-agent/config.json)
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
	if v := os.Getenv("OMNIDEV_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.MaxParallel = n
		}
	}
	if v := os.Getenv("OMNIDEV_SUB_AGENT_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.SubAgentTimeout = n
		}
	}
	if v := os.Getenv("OMNIDEV_SUB_AGENT_MAX_TURNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.SubAgentMaxTurns = n
		}
	}
	if v := os.Getenv("OMNIDEV_SUB_AGENT_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			dst.SubAgentMaxRetries = n
		}
	}
	if v := os.Getenv("OMNIDEV_LLM_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			dst.LLMMaxRetries = n
		}
	}
	if v := os.Getenv("OMNIDEV_LLM_RETRY_BACKOFF_SEC"); v != "" {
		parts := strings.Split(v, ",")
		var secs []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if n, err := strconv.Atoi(p); err == nil && n >= 0 {
				secs = append(secs, n)
			}
		}
		if len(secs) > 0 {
			dst.LLMRetryBackoffSec = secs
		}
	}
	if v := os.Getenv("OMNIDEV_LLM_PERSIST_NETWORK_RETRY"); v != "" {
		dst.LLMPersistNetworkRetry = boolPtr(strings.EqualFold(v, "true") || v == "1")
	}
	if v := os.Getenv("OMNIDEV_MAX_CONSECUTIVE_TOOL_DENIALS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			dst.MaxConsecutiveToolDenials = n
		}
	}
	if v := os.Getenv("OMNIDEV_TOOL_RESULT_MAX_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ToolResultMaxChars = n
		}
	}
	if v := os.Getenv("OMNIDEV_TOOL_SPOOL_DIR"); v != "" {
		dst.ToolSpoolDir = v
	}
	if v := os.Getenv("OMNIDEV_SEARCH_CODE_MAX_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.SearchCodeMaxLines = n
		}
	}
	if v := os.Getenv("OMNIDEV_LIST_DIR_MAX_ENTRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ListDirMaxEntries = n
		}
	}
	if v := os.Getenv("OMNIDEV_READ_FILE_DEFAULT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ReadFileDefaultLimit = n
		}
	}
	if v := os.Getenv("OMNIDEV_CONTEXT_TOOL_RESULTS_KEEP_FULL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ContextToolResultsKeepFull = n
		}
	}
	if v := os.Getenv("OMNIDEV_CONTEXT_MIN_KEEP_ENTRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.ContextMinKeepEntries = n
		}
	}
	if v := os.Getenv("OMNIDEV_GUARD_ANALYSIS_MAX_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.GuardAnalysisMaxChars = n
		}
	}
	if v := os.Getenv("OMNIDEV_PIPELINE_USE_LLM_CLASSIFIER"); v != "" {
		dst.PipelineUseLLMClassifier = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("OMNIDEV_PIPELINE_USE_LLM_REQUIREMENTS"); v != "" {
		dst.PipelineUseLLMRequirements = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("OMNIDEV_PIPELINE_USE_LLM_COMPLEXITY"); v != "" {
		dst.PipelineUseLLMComplexity = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("OMNIDEV_PIPELINE_USE_LLM_ACCEPTANCE"); v != "" {
		dst.PipelineUseLLMAcceptance = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("OMNIDEV_PIPELINE_PLAN_MODE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 2 {
			dst.PipelinePlanMode = n
		}
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
	if src.MaxParallel > 0 {
		dst.MaxParallel = src.MaxParallel
	}
	if src.SubAgentTimeout > 0 {
		dst.SubAgentTimeout = src.SubAgentTimeout
	}
	if src.SubAgentMaxTurns > 0 {
		dst.SubAgentMaxTurns = src.SubAgentMaxTurns
	}
	if src.SubAgentMaxRetries > 0 {
		dst.SubAgentMaxRetries = src.SubAgentMaxRetries
	}
	if src.LLMMaxRetries > 0 {
		dst.LLMMaxRetries = src.LLMMaxRetries
	}
	if len(src.LLMRetryBackoffSec) > 0 {
		dst.LLMRetryBackoffSec = append([]int(nil), src.LLMRetryBackoffSec...)
	}
	if src.LLMPersistNetworkRetry != nil {
		v := *src.LLMPersistNetworkRetry
		dst.LLMPersistNetworkRetry = &v
	}
	if src.MaxConsecutiveToolDenials > 0 {
		dst.MaxConsecutiveToolDenials = src.MaxConsecutiveToolDenials
	}
	if src.ToolResultMaxChars > 0 {
		dst.ToolResultMaxChars = src.ToolResultMaxChars
	}
	if src.ToolSpoolDir != "" {
		dst.ToolSpoolDir = src.ToolSpoolDir
	}
	if src.SearchCodeMaxLines > 0 {
		dst.SearchCodeMaxLines = src.SearchCodeMaxLines
	}
	if src.ListDirMaxEntries > 0 {
		dst.ListDirMaxEntries = src.ListDirMaxEntries
	}
	if src.ReadFileDefaultLimit > 0 {
		dst.ReadFileDefaultLimit = src.ReadFileDefaultLimit
	}
	if src.ContextToolResultsKeepFull > 0 {
		dst.ContextToolResultsKeepFull = src.ContextToolResultsKeepFull
	}
	if src.ContextMinKeepEntries > 0 {
		dst.ContextMinKeepEntries = src.ContextMinKeepEntries
	}
	if src.GuardAnalysisMaxChars > 0 {
		dst.GuardAnalysisMaxChars = src.GuardAnalysisMaxChars
	}
	if src.PipelineUseLLMClassifier {
		dst.PipelineUseLLMClassifier = true
	}
	if src.PipelineUseLLMRequirements {
		dst.PipelineUseLLMRequirements = true
	}
	if src.PipelineUseLLMComplexity {
		dst.PipelineUseLLMComplexity = true
	}
	if src.PipelineUseLLMAcceptance {
		dst.PipelineUseLLMAcceptance = true
	}
	if src.PipelinePlanMode > 0 {
		dst.PipelinePlanMode = src.PipelinePlanMode
	}
	if len(src.SkillsDirs) > 0 {
		dst.SkillsDirs = append([]string(nil), src.SkillsDirs...)
	}
	if len(src.MCPServers) > 0 {
		if dst.MCPServers == nil {
			dst.MCPServers = make(map[string]mcp.ServerConfig)
		}
		for k, v := range src.MCPServers {
			dst.MCPServers[k] = v
		}
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

func boolPtr(v bool) *bool {
	return &v
}
