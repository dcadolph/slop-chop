package config

import (
	"github.com/spf13/pflag"

	"github.com/dcadolph/slop-chop/internal/rewrite"
)

// KeyProvider is the provider flag name.
const KeyProvider = "provider"

// DefaultProvider is the model backend used when the flag is not set.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultProvider = string(rewrite.ProviderAnthropic)

// FlagProvider picks the model backend for the rewrite pass.
//
//nolint:gochecknoglobals // Flag definition.
var FlagProvider = pflag.Flag{
	Name:     KeyProvider,
	Usage:    "Backend for --rewrite: anthropic or openai (default anthropic).",
	Value:    &FlagValue{Val: DefaultProvider, ValType: "string"},
	DefValue: DefaultProvider,
}

// Provider returns the configured rewrite backend.
func Provider() string { return loadString(KeyProvider, DefaultProvider) }
