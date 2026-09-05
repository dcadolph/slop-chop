package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyWhy is the why flag name.
const KeyWhy = "why"

// DefaultWhy is false, keeping the one-line-per-finding output.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultWhy = false

// FlagWhy adds a plain-words explanation under each finding, so nobody has to decode a
// rule name to know what fired or what happens to it.
//
//nolint:gochecknoglobals // Flag definition.
var FlagWhy = pflag.Flag{
	Name:        KeyWhy,
	Usage:       "Explain each finding in plain words under its line.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultWhy), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultWhy),
	NoOptDefVal: "true",
}

// Why reports whether check explains each finding.
func Why() bool { return loadBool(KeyWhy, DefaultWhy) }
