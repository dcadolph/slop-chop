package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyVerify is the verify flag name.
const KeyVerify = "verify"

// DefaultVerify is false, meaning no meaning check.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultVerify = false

// FlagVerify runs the model meaning check after the rewrite pass.
//
//nolint:gochecknoglobals // Flag definition.
var FlagVerify = pflag.Flag{
	Name:        KeyVerify,
	Usage:       "After --rewrite, ask a model to check the rewrite kept the meaning.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultVerify), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultVerify),
	NoOptDefVal: "true",
}

// Verify reports whether the model meaning check should run.
func Verify() bool { return loadBool(KeyVerify, DefaultVerify) }
