package llm

import (
	"strings"
)

const (
	DefaultOpenAIBaseURL     = "https://api.openai.com/v1"
	DefaultAnthropicBaseURL  = "https://api.anthropic.com/v1"
)

// NewProvider creates the appropriate LLM client from provider name and connection settings.
// provider: openai | deepseek | anthropic | claude
func NewProvider(provider, baseURL, apiKey, model string, opts Options) Provider {
	opts.GatewayMode = ResolveGatewayMode(opts.GatewayMode, provider)
	opts = opts.Resolved()

	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "anthropic", "claude":
		url := baseURL
		if url == "" {
			url = DefaultAnthropicBaseURL
		}
		return NewAnthropic(url, apiKey, model, opts)
	case "deepseek":
		url := baseURL
		if url == "" || url == DefaultOpenAIBaseURL {
			url = DefaultDeepSeekBaseURL
		}
		return NewOpenAI(url, apiKey, model, opts)
	default:
		url := baseURL
		if url == "" {
			url = DefaultOpenAIBaseURL
		}
		return NewOpenAI(url, apiKey, model, opts)
	}
}

// NormalizeBaseURL trims whitespace and trailing slashes from an API base URL.
func NormalizeBaseURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}
