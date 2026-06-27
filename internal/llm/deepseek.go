package llm

// DefaultDeepSeekBaseURL is the standard DeepSeek API endpoint.
const DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"

// DeepSeek model constants.
const (
	DeepSeekChat     = "deepseek-chat"
	DeepSeekReasoner = "deepseek-reasoner"
)

// NewDeepSeek creates an OpenAI-compatible Provider pointed at DeepSeek's API.
func NewDeepSeek(apiKey, model string, opts Options) Provider {
	if model == "" {
		model = DeepSeekChat
	}
	return NewProvider("deepseek", "", apiKey, model, opts)
}
