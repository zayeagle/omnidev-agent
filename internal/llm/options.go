package llm

const (
	DefaultMaxTokens         = 8192
	StrictGatewayMaxTokensCap = 8192
	DefaultTimeoutSec        = 120
	DefaultTemperature       = 0.7
)

// Options holds generation parameters shared by all providers.
type Options struct {
	MaxTokens       int
	Temperature     float64
	TimeoutSec      int
	GatewayMode     string // auto | openai | strict
	OmitTemperature bool
}

// Resolved returns options with safe defaults for API calls.
func (o Options) Resolved() Options {
	out := o
	if out.GatewayMode == "" || out.GatewayMode == GatewayAuto {
		out.GatewayMode = GatewayOpenAI
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = DefaultMaxTokens
	}
	if out.GatewayMode == GatewayStrict {
		out.OmitTemperature = true
		if out.MaxTokens > StrictGatewayMaxTokensCap {
			out.MaxTokens = StrictGatewayMaxTokensCap
		}
	}
	if out.TimeoutSec <= 0 {
		out.TimeoutSec = DefaultTimeoutSec
	}
	if !out.OmitTemperature && out.Temperature <= 0 {
		out.Temperature = DefaultTemperature
	}
	return out
}

// OptionsFromConfig builds Options from config-like values.
func OptionsFromConfig(maxTokens int, temperature float64, timeoutSec int, compatMode string) Options {
	mode := compatMode
	if mode == "" {
		mode = GatewayAuto
	}
	return Options{
		MaxTokens:   maxTokens,
		Temperature: temperature,
		TimeoutSec:  timeoutSec,
		GatewayMode: ResolveGatewayMode(mode, ""),
	}
}
