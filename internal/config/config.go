package config

import (
	"os"
	"path/filepath"

	"github.com/zayeagle/omnidev-agent/internal/mcp"
)

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
	// Task dispatcher (parallel sub-agents)
	MaxParallel           int   `json:"max_parallel"`             // default 4
	SubAgentTimeout       int   `json:"sub_agent_timeout"`        // seconds, default 120
	SubAgentMaxTurns      int   `json:"sub_agent_max_turns"`      // default 10
	SubAgentMaxRetries    int   `json:"sub_agent_max_retries"`    // re-run failed sub-tasks, default 0
	// LLM request retry (transient API / network errors)
	LLMMaxRetries              int   `json:"llm_max_retries"`                        // default 3 (4 attempts total) for non-network errors
	LLMRetryBackoffSec           []int `json:"llm_retry_backoff_sec"`                  // default [1, 2, 4]
	LLMPersistNetworkRetry       *bool `json:"llm_persist_network_retry,omitempty"`    // default true — network errors retry until success or cancel
	// Agent loop safety
	MaxConsecutiveToolDenials int `json:"max_consecutive_tool_denials"` // default 3; 0 = disable abort
	// Tool output (PARTIAL + spool — full content never silently dropped)
	ToolResultMaxChars  int    `json:"tool_result_max_chars"`  // default 8000 inline budget
	ToolSpoolDir        string `json:"tool_spool_dir"`         // default .ai_history/tool_spool/
	SearchCodeMaxLines  int    `json:"search_code_max_lines"`  // default 100
	ListDirMaxEntries   int    `json:"list_dir_max_entries"`   // default 200
	ReadFileDefaultLimit int   `json:"read_file_default_limit"` // default 300 lines when limit omitted
	// Context slimming (reduce tokens sent each LLM turn)
	ContextToolResultsKeepFull int `json:"context_tool_results_keep_full"` // default 3
	ContextMinKeepEntries      int `json:"context_min_keep_entries"`       // default 10
	GuardAnalysisMaxChars      int `json:"guard_analysis_max_chars"`       // default 4000
	// Pipeline LLM stages (false = heuristic / skip — saves tokens)
	PipelineUseLLMClassifier   bool `json:"pipeline_use_llm_classifier"`
	PipelineUseLLMRequirements bool `json:"pipeline_use_llm_requirements"`
	PipelineUseLLMComplexity    bool `json:"pipeline_use_llm_complexity"`
	PipelineUseLLMAcceptance    bool `json:"pipeline_use_llm_acceptance"`
	PipelinePlanMode            int  `json:"pipeline_plan_mode"` // 0=auto LLM plans (default), 1=same as 0, 2=skip LLM plan
	// Skills (Cursor-style SKILL.md directories)
	SkillsDirs []string `json:"skills_dirs"`
	// MCP (Model Context Protocol) stdio servers → extra agent tools
	MCPServers map[string]mcp.ServerConfig `json:"mcp_servers"`
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

// DefaultGlobalConfigPath returns <user-home>/.omnidev-agent/config.json using os.UserHomeDir()
// so global config resolves correctly on Linux, macOS, and Windows (USERPROFILE).
func DefaultGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".omnidev-agent", "config.json")
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
		MaxParallel:               4,
		SubAgentTimeout:           180,
		SubAgentMaxTurns:          15,
		SubAgentMaxRetries:        0,
		LLMMaxRetries:             3,
		LLMRetryBackoffSec:        []int{1, 2, 4},
		MaxConsecutiveToolDenials: 3,
		ToolResultMaxChars:        8000,
		ToolSpoolDir:              ".ai_history/tool_spool/",
		SearchCodeMaxLines:        100,
		ListDirMaxEntries:         200,
		ReadFileDefaultLimit:      300,
		ContextToolResultsKeepFull: 8,
		ContextMinKeepEntries:      10,
		GuardAnalysisMaxChars:      4000,
		PipelineUseLLMAcceptance:   false,
		PipelinePlanMode:           0,
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
