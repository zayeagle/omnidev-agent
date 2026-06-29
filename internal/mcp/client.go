package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const protocolVersion = "2024-11-05"

// ServerConfig describes how to spawn an MCP server (stdio transport).
type ServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Cwd       string            `json:"cwd"`
	ToolLevel string            `json:"tool_level"` // safe | dangerous (default dangerous)
	Disabled  bool              `json:"disabled"`
}

// ToolDef is an MCP tool exposed to the agent registry.
type ToolDef struct {
	Server      string
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// Client is a minimal MCP JSON-RPC client over stdio.
type Client struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex
	nextID  int
	started time.Time
}

// Connect starts the MCP server subprocess and completes the initialize handshake.
func Connect(ctx context.Context, name string, cfg ServerConfig) (*Client, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("mcp server %q: command is required", name)
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if cfg.Cwd != "" {
		cmd.Dir = cfg.Cwd
	}
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	go io.Copy(io.Discard, stderrPipe)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %q start: %w", name, err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		started: time.Now(),
	}

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := c.call(initCtx, "initialize", map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "omnidev-agent",
			"version": "1.0.0",
		},
	}); err != nil {
		c.Close()
		return nil, fmt.Errorf("mcp %q initialize: %w", name, err)
	}
	_ = c.notify("notifications/initialized", map[string]interface{}{})

	return c, nil
}

func (c *Client) notify(method string, params interface{}) error {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	return c.write(payload)
}

func (c *Client) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := c.nextID
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	if err := c.writeLocked(req); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		line = bytesTrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var envelope struct {
			ID     *json.RawMessage `json:"id"`
			Result json.RawMessage  `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		if envelope.ID == nil {
			continue // notification
		}
		var respID int
		if err := json.Unmarshal(*envelope.ID, &respID); err != nil || respID != id {
			continue
		}
		if envelope.Error != nil {
			return nil, fmt.Errorf("mcp error %d: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return envelope.Result, nil
	}
}

func (c *Client) write(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeLocked(v)
}

func (c *Client) writeLocked(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	return err
}

// ListTools returns tools advertised by the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error) {
	raw, err := c.call(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	defs := make([]ToolDef, 0, len(out.Tools))
	for _, t := range out.Tools {
		defs = append(defs, ToolDef{
			Server:      c.name,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return defs, nil
}

// CallTool invokes an MCP tool and returns text content for the agent.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	raw, err := c.call(ctx, "tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	var parts []string
	for _, block := range out.Content {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	text := strings.Join(parts, "\n")
	if out.IsError && text == "" {
		text = "MCP tool returned isError=true"
	}
	if text == "" {
		text = "(no content)"
	}
	return text, nil
}

// Close shuts down the MCP server process.
func (c *Client) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// AgentToolName returns the registry name for an MCP tool.
func AgentToolName(server, tool string) string {
	safe := sanitizeName(server)
	t := sanitizeName(tool)
	return "mcp_" + safe + "__" + t
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "x"
	}
	return out
}
