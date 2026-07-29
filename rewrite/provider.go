package rewrite

import "fmt"

// Provider names a model backend the rewrite pass can talk to.
type Provider string

const (
	// ProviderAnthropic is the Anthropic Messages API, the default.
	ProviderAnthropic Provider = "anthropic"
	// ProviderOpenAI is any OpenAI-compatible Chat Completions API, including a local
	// server like Ollama when its base URL is set.
	ProviderOpenAI Provider = "openai"
)

// NewCompleter returns the Completer for provider. model and baseURL may be empty, in
// which case the provider's own defaults stand. baseURL only applies to the OpenAI
// provider. An unknown provider is an error.
func NewCompleter(provider Provider, model, baseURL string) (Completer, error) {
	switch provider {
	case ProviderAnthropic, "":
		return NewAnthropicCompleter(model), nil
	case ProviderOpenAI:
		return NewOpenAICompleter(model, baseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider %q: have %s or %s",
			provider, ProviderAnthropic, ProviderOpenAI)
	}
}
