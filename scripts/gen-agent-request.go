//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func main() {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)

	sys := "You are a helpful coding assistant. You have access to tools. Use them when needed. Always respond in English."
	sys += "\n\nIMPORTANT: All generated code MUST be written under the directory: deliverables/snake-game"
	sys += "\nUse paths relative to that directory (e.g. main.go). NEVER create new project files in the repository root, internal/, cmd/, or tests/."
	sys += "\n\nPROJECT LAYOUT: minimal.\nUse the smallest correct solution — often a single file (e.g. main.go) or at most 2–3 files in the workspace.\nDo NOT create DDD layer directories (domain/application/infrastructure/interfaces) unless the user explicitly asks for them."

	msgs := llm.NormalizeStrictGatewayMessages([]llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "帮我写一个贪吃蛇的小游戏"},
	})

	defs := make([]llm.Tool, 0, toolbox.Count())
	for _, t := range sortedTools(toolbox) {
		defs = append(defs, llm.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}

	openaiTools := make([]map[string]any, len(defs))
	for i, t := range defs {
		openaiTools[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  schema(t.Parameters),
			},
		}
	}

	openaiMsgs := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		openaiMsgs[i] = map[string]any{"role": m.Role, "content": m.Content}
	}

	body := map[string]any{
		"model":       "deepseek/deepseek-v4-pro",
		"max_tokens":  8192,
		"messages":    openaiMsgs,
		"tools":       openaiTools,
		"tool_choice": "auto",
	}

	raw, _ := json.MarshalIndent(body, "", "  ")
	outPath := "scripts/agent-full-request.json"
	_ = os.WriteFile(outPath, raw, 0644)
	fmt.Println(string(raw))
	fmt.Fprintf(os.Stderr, "\nwritten %s (%d bytes)\n", outPath, len(raw))
}

func sortedTools(r *tools.Registry) []tools.Tool {
	list := r.List()
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list
}

func schema(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	required := []string{}
	props := map[string]any{}
	for name, raw := range params {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		prop := map[string]any{}
		for k, v := range s {
			if k == "required" {
				if req, ok := v.(bool); ok && req {
					required = append(required, name)
				}
				continue
			}
			prop[k] = v
		}
		props[name] = prop
	}
	sort.Strings(required)
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
