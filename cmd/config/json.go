package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyJSON is the json flag name.
const KeyJSON = "json"

// DefaultJSON is false, meaning text output.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultJSON = false

// FlagJSON switches stdout to JSON: findings for check, the result for fix.
//
//nolint:gochecknoglobals // Flag definition.
var FlagJSON = pflag.Flag{
	Name:        KeyJSON,
	Usage:       "Write JSON to stdout (findings for check, result for fix).",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultJSON), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultJSON),
	NoOptDefVal: "true",
}

// JSON reports whether stdout should carry JSON.
func JSON() bool { return loadBool(KeyJSON, DefaultJSON) }
