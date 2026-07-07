package config

import "github.com/spf13/pflag"

// KeyPreset is the preset flag name.
const KeyPreset = "preset"

// DefaultPreset is empty, meaning no built-in preset is applied.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultPreset = ""

// FlagPreset applies one or more built-in presets on top of the active profile.
//
//nolint:gochecknoglobals // Flag definition.
var FlagPreset = pflag.Flag{
	Name:     KeyPreset,
	Usage:    "Apply built-in presets on top of the profile, comma separated (e.g. plain).",
	Value:    &FlagValue{ValType: "string"},
	DefValue: DefaultPreset,
}

// Preset returns the configured preset names as a raw comma-separated string, or empty
// when none is set.
func Preset() string { return loadString(KeyPreset, DefaultPreset) }
