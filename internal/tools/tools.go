package tools

import (
	"context"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

type Result struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func OkResult(data string) *Result {
	return &Result{Success: true, Data: data}
}

func ErrResult(err string) *Result {
	return &Result{Success: false, Error: err}
}

// LimitedResult applies PARTIAL/spool delivery budgets to tool output.
func LimitedResult(toolName, content string) *Result {
	return okLimited(toolName, content)
}

// Tool defines the contract for every tool callable by the LLM.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{} // JSON Schema for OpenAI function calling
	Level() permissions.Level
	Execute(ctx context.Context, args map[string]interface{}) *Result
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	list := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

func (r *Registry) Count() int {
	return len(r.tools)
}
