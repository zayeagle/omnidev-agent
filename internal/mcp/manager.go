package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// Manager connects configured MCP servers and registers their tools.
type Manager struct {
	mu      sync.Mutex
	servers map[string]*Client
	tools   []ToolDef
}

// Start connects to all configured MCP servers. Failures are logged; other servers continue.
func Start(ctx context.Context, configs map[string]ServerConfig) (*Manager, error) {
	m := &Manager{servers: make(map[string]*Client)}
	if len(configs) == 0 {
		return m, nil
	}
	for name, cfg := range configs {
		if cfg.Disabled || cfg.Command == "" {
			continue
		}
		client, err := Connect(ctx, name, cfg)
		if err != nil {
			log.Printf("mcp: skip server %q: %v", name, err)
			continue
		}
		toolsCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defs, err := client.ListTools(toolsCtx)
		cancel()
		if err != nil {
			log.Printf("mcp: tools/list failed for %q: %v", name, err)
			_ = client.Close()
			continue
		}
		m.mu.Lock()
		m.servers[name] = client
		for _, d := range defs {
			m.tools = append(m.tools, d)
		}
		m.mu.Unlock()
		log.Printf("mcp: connected %q (%d tools)", name, len(defs))
	}
	return m, nil
}

// RegisterTools adds MCP tools to the agent toolbox.
func (m *Manager) RegisterTools(reg *tools.Registry, configs map[string]ServerConfig) {
	if m == nil || reg == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, def := range m.tools {
		client := m.servers[def.Server]
		if client == nil {
			continue
		}
		cfg := configs[def.Server]
		level := permissions.LevelDangerous
		if strings.EqualFold(cfg.ToolLevel, "safe") {
			level = permissions.LevelSafe
		}
		reg.Register(newMCPTool(client, def, level))
	}
}

// Close shuts down all MCP server processes.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, c := range m.servers {
		if err := c.Close(); err != nil {
			log.Printf("mcp: close %q: %v", name, err)
		}
	}
	m.servers = nil
}

// Summary returns a human-readable status line for /status.
func (m *Manager) Summary() string {
	if m == nil {
		return "MCP: not configured"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.servers) == 0 {
		return "MCP: no servers connected"
	}
	var names []string
	for n := range m.servers {
		names = append(names, n)
	}
	return fmt.Sprintf("MCP: %d server(s), %d tool(s) [%s]", len(m.servers), len(m.tools), strings.Join(names, ", "))
}

// ToolCount returns registered MCP tool count.
func (m *Manager) ToolCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tools)
}

type mcpTool struct {
	client *Client
	def    ToolDef
	level  permissions.Level
}

func newMCPTool(client *Client, def ToolDef, level permissions.Level) *mcpTool {
	return &mcpTool{client: client, def: def, level: level}
}

func (t *mcpTool) Name() string { return AgentToolName(t.def.Server, t.def.Name) }

func (t *mcpTool) Description() string {
	desc := t.def.Description
	if desc == "" {
		desc = "MCP tool"
	}
	return fmt.Sprintf("[MCP %s] %s", t.def.Server, desc)
}

func (t *mcpTool) Level() permissions.Level { return t.level }

func (t *mcpTool) Parameters() map[string]interface{} {
	if t.def.InputSchema != nil {
		return t.def.InputSchema
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *mcpTool) Execute(ctx context.Context, args map[string]interface{}) *tools.Result {
	text, err := t.client.CallTool(ctx, t.def.Name, args)
	if err != nil {
		return tools.ErrResult(err.Error())
	}
	return tools.LimitedResult(t.Name(), text)
}
