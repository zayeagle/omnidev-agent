package config

import "github.com/zayeagle/omnidev-agent/internal/tools"

// ToolResultLimits maps config to tools.ResultLimits.
func (c *Config) ToolResultLimits() tools.ResultLimits {
	def := tools.DefaultResultLimits()
	out := def
	if c.ToolResultMaxChars > 0 {
		out.MaxChars = c.ToolResultMaxChars
	}
	if c.ToolSpoolDir != "" {
		out.SpoolDir = c.ToolSpoolDir
	}
	if c.SearchCodeMaxLines > 0 {
		out.SearchMaxLines = c.SearchCodeMaxLines
	}
	if c.ListDirMaxEntries > 0 {
		out.ListDirMaxEntries = c.ListDirMaxEntries
	}
	if c.ReadFileDefaultLimit > 0 {
		out.ReadFileDefaultLimit = c.ReadFileDefaultLimit
	}
	return out
}
