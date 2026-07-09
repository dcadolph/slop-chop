package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyMax is the max flag name.
const KeyMax = "max"

// DefaultMax is -1, which leaves the score gate off so score only reports.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultMax = -1

// FlagMax sets the highest score that still passes, so score can gate CI.
//
//nolint:gochecknoglobals // Flag definition.
var FlagMax = pflag.Flag{
	Name:     KeyMax,
	Usage:    "Fail when the score is above this 0-100 value (default off).",
	Value:    &FlagValue{Val: strconv.Itoa(DefaultMax), ValType: "int"},
	DefValue: strconv.Itoa(DefaultMax),
}

// Max returns the configured score gate, or -1 when unset.
func Max() int { return loadInt(KeyMax, DefaultMax) }
