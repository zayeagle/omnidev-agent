package llm

import "context"

type Provider interface {
	Chat(ctx context.Context, req *Request) (*Response, error)
	Stream(ctx context.Context, req *Request) (<-chan *Chunk, error)
}

type Request struct {
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Chunk struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
	Done    bool   `json:"done"`
}

type Message struct {
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func NewMessage(role, content string) Message {
	return Message{Role: role, Content: content}
}

func NewToolMessage(toolCallID, content string) Message {
	return Message{Role: "tool", Content: content, ToolCallID: toolCallID}
}
