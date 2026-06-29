package config

import (
	"os"
	"path/filepath"

	"github.com/zayeagle/omnidev-agent/internal/mcp"
)

// SkillsSearchDirs returns skill scan paths (project overrides global when names collide).
func (c *Config) SkillsSearchDirs() []string {
	if len(c.SkillsDirs) > 0 {
		out := make([]string, len(c.SkillsDirs))
		copy(out, c.SkillsDirs)
		return out
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".omnidev-agent", "skills"))
	}
	dirs = append(dirs, ".omnidev-agent/skills")
	return dirs
}

// MCPServerConfigs returns MCP server definitions (nil when empty).
func (c *Config) MCPServerConfigs() map[string]mcp.ServerConfig {
	if len(c.MCPServers) == 0 {
		return nil
	}
	out := make(map[string]mcp.ServerConfig, len(c.MCPServers))
	for k, v := range c.MCPServers {
		out[k] = v
	}
	return out
}
