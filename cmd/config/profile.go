package config

import "github.com/spf13/pflag"

// KeyProfile is the profile flag name.
const KeyProfile = "profile"

// DefaultProfile is empty, meaning the built-in profile.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultProfile = ""

// FlagProfile points at a JSON style profile instead of the built-in one.
//
//nolint:gochecknoglobals // Flag definition.
var FlagProfile = pflag.Flag{
	Name:     KeyProfile,
	Usage:    "Path to a JSON style profile (default: built-in).",
	Value:    &FlagValue{ValType: "string"},
	DefValue: DefaultProfile,
}

// Profile returns the configured profile path, or empty for the built-in profile.
func Profile() string { return loadString(KeyProfile, DefaultProfile) }
