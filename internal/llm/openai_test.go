package llm

import (
	"encoding/json"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	if got := NormalizeBaseURL("https://api.example.com/v1/"); got != "https://api.example.com/v1" {
		t.Fatalf("NormalizeBaseURL = %q", got)
	}
}

func TestOpenAIRequestIncludesMaxTokens(t *testing.T) {
	client := NewOpenAI("https://api.example.com/v1", "key", "gpt-4o", Options{MaxTokens: 4096, TimeoutSec: 30, GatewayMode: GatewayOpenAI})
	body := client.buildRequest(&Request{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, false)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["max_tokens"] != float64(4096) {
		t.Fatalf("max_tokens = %v, want 4096", m["max_tokens"])
	}
	if _, ok := m["stream"]; ok {
		t.Fatalf("non-stream request should omit stream field, got %v", m["stream"])
	}
}

func TestOpenAIMessageOmitsNullContent(t *testing.T) {
	msg := openAIMessage{
		Role: "assistant",
		ToolCalls: []openAIToolCall{{
			ID: "call-1", Type: "function",
			Function: openAIFuncCall{Name: "read_file", Arguments: `{"path":"a.go"}`},
		}},
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["content"]; ok {
		t.Fatalf("tool-only assistant message should omit content, got %v", m["content"])
	}
}

func TestToolParametersSchemaRequired(t *testing.T) {
	schema := toolParametersSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":     "string",
			"required": true,
		},
		"limit": map[string]interface{}{
			"type": "integer",
		},
	})
	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T", schema["required"])
	}
	if len(req) != 1 || req[0] != "path" {
		t.Fatalf("required = %v, want [path]", req)
	}
}

func TestOpenAIRequestStrictOmitsTemperature(t *testing.T) {
	client := NewOpenAI("https://gateway.example.com/v1", "key", "example-model",
		Options{MaxTokens: 16384, Temperature: 0.7, GatewayMode: GatewayStrict})
	body := client.buildRequest(&Request{
		Messages: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
		},
	}, false)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["temperature"]; ok {
		t.Fatalf("strict request should omit temperature, got %v", m["temperature"])
	}
	if m["max_tokens"] != float64(StrictGatewayMaxTokensCap) {
		t.Fatalf("max_tokens = %v, want %d", m["max_tokens"], StrictGatewayMaxTokensCap)
	}
	msgs := m["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("expected merged single user message, got %d", len(msgs))
	}
}

func TestNewProviderAnthropic(t *testing.T) {
	p := NewProvider("anthropic", "", "key", "claude-3-5-sonnet-20241022", Options{TimeoutSec: 30, GatewayMode: GatewayOpenAI})
	if _, ok := p.(*AnthropicClient); !ok {
		t.Fatalf("expected AnthropicClient, got %T", p)
	}
}

func TestConvertAnthropicMessages(t *testing.T) {
	system, msgs := convertAnthropicMessages([]Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ToolCalls: []ToolCall{{ID: "t1", Name: "read_file", Arguments: map[string]interface{}{"path": "a.go"}}}},
		{Role: "tool", Content: "file data", ToolCallID: "t1"},
	})
	if system != "You are helpful." {
		t.Fatalf("system = %q", system)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages len = %d, want 3", len(msgs))
	}
	if msgs[2].Role != "user" {
		t.Fatalf("tool result role = %q, want user", msgs[2].Role)
	}
}
