package config

import "github.com/spf13/pflag"

// KeyDialect is the dialect flag name.
const KeyDialect = "dialect"

// DefaultDialect is empty, which leaves the spelling pass off and lets a profile's own
// dialect field stand when the flag is not set.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultDialect = ""

// FlagDialect selects a spelling variant to enforce.
//
//nolint:gochecknoglobals // Flag definition.
var FlagDialect = pflag.Flag{
	Name:     KeyDialect,
	Usage:    "Enforce a spelling variant: american, british, or off (default off).",
	Value:    &FlagValue{Val: DefaultDialect, ValType: "string"},
	DefValue: DefaultDialect,
}

// Dialect returns the configured dialect, or empty when unset so a profile's own value
// stands.
func Dialect() string { return loadString(KeyDialect, DefaultDialect) }
