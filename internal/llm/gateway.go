package llm

import (
	"strings"
)

const (
	GatewayAuto   = "auto"
	GatewayOpenAI = "openai"
	GatewayStrict = "strict"
)

// ResolveGatewayMode maps compat_mode / provider hints to a gateway profile.
// "auto" uses standard OpenAI request shape (no URL sniffing).
func ResolveGatewayMode(compatMode, provider string) string {
	mode := strings.ToLower(strings.TrimSpace(compatMode))
	switch mode {
	case GatewayStrict:
		return GatewayStrict
	case GatewayOpenAI:
		return GatewayOpenAI
	}
	if strings.ToLower(strings.TrimSpace(provider)) == GatewayStrict {
		return GatewayStrict
	}
	return GatewayOpenAI
}

// NormalizeStrictGatewayMessages adapts messages for strict OpenAI-compatible gateways.
// Some proxies reject role=system or out-of-order system messages.
func NormalizeStrictGatewayMessages(msgs []Message) []Message {
	var systemParts []string
	var rest []Message
	for _, m := range msgs {
		if m.Role == "system" && m.Content != "" {
			systemParts = append(systemParts, m.Content)
			continue
		}
		rest = append(rest, m)
	}
	if len(systemParts) == 0 {
		return msgs
	}
	prefix := strings.Join(systemParts, "\n\n")
	for i := range rest {
		if rest[i].Role == "user" {
			if rest[i].Content != "" {
				rest[i].Content = prefix + "\n\n" + rest[i].Content
			} else {
				rest[i].Content = prefix
			}
			return rest
		}
	}
	return append([]Message{{Role: "user", Content: prefix}}, rest...)
}
