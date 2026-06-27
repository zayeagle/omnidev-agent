package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProviderGatewayMode returns the gateway profile when the provider exposes it.
func ProviderGatewayMode(p Provider) string {
	type gatewayModeProvider interface {
		GatewayMode() string
	}
	if g, ok := p.(gatewayModeProvider); ok {
		return g.GatewayMode()
	}
	return GatewayOpenAI
}

// AdaptToolsForGateway builds the LLM request for tool use. Strict gateways often
// reject native function tools (SSE error 400) but accept JSON plans in message text.
func AdaptToolsForGateway(req *Request, tools []Tool, mode string) *Request {
	out := *req
	if len(tools) == 0 {
		return &out
	}
	if mode != GatewayStrict {
		out.Tools = tools
		return &out
	}
	out.Messages = injectStructuredToolInstructions(req.Messages, tools)
	return &out
}

func injectStructuredToolInstructions(msgs []Message, tools []Tool) []Message {
	var b strings.Builder
	b.WriteString("\n\nTOOL USE: When you need tools, reply with ONLY one JSON object (no markdown fences):\n")
	b.WriteString(`{"steps":[{"id":"1","action":"reasoning","text":"brief plan"},{"id":"2","action":"tool_call","tool":"TOOL_NAME","args":{}}]}` + "\n")
	b.WriteString("\nAvailable tools:\n")
	for _, t := range tools {
		b.WriteString(fmt.Sprintf("- %s: %s", t.Name, t.Description))
		if len(t.Parameters) > 0 {
			if raw, err := json.Marshal(toolParametersSchema(t.Parameters)); err == nil {
				b.WriteString(" params=" + string(raw))
			}
		}
		b.WriteByte('\n')
	}

	out := make([]Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if out[i].Role == "system" {
			out[i].Content += b.String()
			return out
		}
	}
	return append([]Message{{Role: "system", Content: strings.TrimSpace(b.String())}}, out...)
}
