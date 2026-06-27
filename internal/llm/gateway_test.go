package llm

import (
	"strings"
	"testing"
)

func TestResolveGatewayMode_Strict(t *testing.T) {
	got := ResolveGatewayMode("strict", "openai")
	if got != GatewayStrict {
		t.Fatalf("ResolveGatewayMode = %q, want strict", got)
	}
}

func TestResolveGatewayMode_AutoIsOpenAI(t *testing.T) {
	got := ResolveGatewayMode("auto", "openai")
	if got != GatewayOpenAI {
		t.Fatalf("ResolveGatewayMode = %q, want openai", got)
	}
}

func TestNormalizeStrictGatewayMessages(t *testing.T) {
	out := NormalizeStrictGatewayMessages([]Message{
		{Role: "system", Content: "You are a planner."},
		{Role: "user", Content: "hello"},
	})
	if len(out) != 1 || out[0].Role != "user" {
		t.Fatalf("unexpected messages: %+v", out)
	}
}

func TestStrictGatewayOptionsCapMaxTokens(t *testing.T) {
	opts := Options{MaxTokens: 16384, GatewayMode: GatewayStrict}.Resolved()
	if opts.MaxTokens != StrictGatewayMaxTokensCap {
		t.Fatalf("max_tokens = %d, want %d", opts.MaxTokens, StrictGatewayMaxTokensCap)
	}
	if !opts.OmitTemperature {
		t.Fatal("strict mode should omit temperature")
	}
}

func TestAdaptToolsForGatewayStrictOmitsNativeTools(t *testing.T) {
	tools := []Tool{{Name: "read_file", Description: "Read a file", Parameters: map[string]interface{}{
		"path": map[string]interface{}{"type": "string", "required": true},
	}}}
	req := AdaptToolsForGateway(&Request{
		Messages: []Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
	}, tools, GatewayStrict)
	if len(req.Tools) != 0 {
		t.Fatalf("strict mode should omit native tools, got %d", len(req.Tools))
	}
	if !strings.Contains(req.Messages[0].Content, "TOOL USE:") {
		t.Fatalf("expected structured tool instructions in system message, got %q", req.Messages[0].Content)
	}
}

func TestAdaptToolsForGatewayOpenAIKeepsNativeTools(t *testing.T) {
	tools := []Tool{{Name: "read_file", Description: "Read a file"}}
	req := AdaptToolsForGateway(&Request{Messages: []Message{{Role: "user", Content: "hi"}}}, tools, GatewayOpenAI)
	if len(req.Tools) != 1 {
		t.Fatalf("openai mode should keep native tools, got %d", len(req.Tools))
	}
}
