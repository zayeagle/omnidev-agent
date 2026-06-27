package llm

import "testing"

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
