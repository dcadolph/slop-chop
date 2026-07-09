package config

import "github.com/spf13/pflag"

// KeyBaseURL is the base-url flag name.
const KeyBaseURL = "base-url"

// DefaultBaseURL is empty, which leaves the provider on its own default endpoint.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultBaseURL = ""

// FlagBaseURL overrides the API endpoint for the openai provider, so the rewrite pass can
// reach any OpenAI-compatible server, including a local one like Ollama.
//
//nolint:gochecknoglobals // Flag definition.
var FlagBaseURL = pflag.Flag{
	Name:     KeyBaseURL,
	Usage:    "Override the openai endpoint, e.g. http://localhost:11434/v1 for a local server.",
	Value:    &FlagValue{Val: DefaultBaseURL, ValType: "string"},
	DefValue: DefaultBaseURL,
}

// BaseURL returns the configured endpoint override, or empty when unset.
func BaseURL() string { return loadString(KeyBaseURL, DefaultBaseURL) }
