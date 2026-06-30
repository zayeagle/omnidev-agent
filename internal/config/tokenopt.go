package config

import "github.com/zayeagle/omnidev-agent/internal/agent"

// PipelineOptions maps config to agent.PipelineOptions.
func (c *Config) PipelineOptions() agent.PipelineOptions {
	def := agent.DefaultPipelineOptions()
	out := def
	if c.PipelineUseLLMClassifier {
		out.UseLLMClassifier = true
	}
	if c.PipelineUseLLMRequirements {
		out.UseLLMRequirements = true
	}
	if c.PipelineUseLLMComplexity {
		out.UseLLMComplexity = true
	}
	if c.PipelineUseLLMAcceptance {
		out.UseLLMAcceptance = true
	} else {
		out.UseLLMAcceptance = false
	}
	if c.PipelinePlanMode > 0 {
		out.PlanMode = c.PipelinePlanMode
	}
	return out
}

// ContextSlimOptions maps config to agent.ContextSlimOptions.
func (c *Config) ContextSlimOptions() agent.ContextSlimOptions {
	def := agent.DefaultContextSlimOptions()
	out := def
	if c.ContextToolResultsKeepFull > 0 {
		out.ToolResultsKeepFull = c.ContextToolResultsKeepFull
	}
	if c.ContextMinKeepEntries > 0 {
		out.MinKeepEntries = c.ContextMinKeepEntries
	}
	if c.GuardAnalysisMaxChars > 0 {
		out.GuardAnalysisMax = c.GuardAnalysisMaxChars
	}
	return out
}
