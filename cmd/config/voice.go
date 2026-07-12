package config

import "github.com/spf13/pflag"

// KeyVoice is the voice flag name.
const KeyVoice = "voice"

// DefaultVoice is empty, which leaves voice discovery to the built-in path
// ~/.slop-chop/voice.json when the flag is not set.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultVoice = ""

// FlagVoice points at a voice file of keep, prefer, and avoid lists.
//
//nolint:gochecknoglobals // Flag definition.
var FlagVoice = pflag.Flag{
	Name:     KeyVoice,
	Usage:    "Path to a voice file (keep/prefer/avoid). Defaults to ~/.slop-chop/voice.json.",
	Value:    &FlagValue{Val: DefaultVoice, ValType: "string"},
	DefValue: DefaultVoice,
}

// Voice returns the configured voice file path, or empty when unset so the built-in path is
// discovered instead.
func Voice() string { return loadString(KeyVoice, DefaultVoice) }
