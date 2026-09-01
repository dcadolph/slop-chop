package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyBands is the bands flag name.
const KeyBands = "bands"

// DefaultBands is -1, which leaves the drift gate off so drift only reports.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultBands = -1

// FlagBands sets how far a trait may sit outside your range before the run fails, so
// drift can gate CI on a house voice.
//
//nolint:gochecknoglobals // Flag definition.
var FlagBands = pflag.Flag{
	Name:     KeyBands,
	Usage:    "Fail when a trait drifts more than this many bands out of your range (default off).",
	Value:    &FlagValue{Val: strconv.Itoa(DefaultBands), ValType: "int"},
	DefValue: strconv.Itoa(DefaultBands),
}

// Bands returns the configured drift gate, or -1 when unset.
func Bands() int { return loadInt(KeyBands, DefaultBands) }
