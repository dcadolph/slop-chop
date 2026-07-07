package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyVerifyStrict is the verify-strict flag name.
const KeyVerifyStrict = "verify-strict"

// DefaultVerifyStrict is false, meaning a flagged meaning change warns but does not fail.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultVerifyStrict = false

// FlagVerifyStrict makes a flagged meaning change fail the command with a non-zero exit.
//
//nolint:gochecknoglobals // Flag definition.
var FlagVerifyStrict = pflag.Flag{
	Name:        KeyVerifyStrict,
	Usage:       "With --verify, exit non-zero when the meaning check flags the rewrite.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultVerifyStrict), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultVerifyStrict),
	NoOptDefVal: "true",
}

// VerifyStrict reports whether a flagged meaning change should fail the command.
func VerifyStrict() bool { return loadBool(KeyVerifyStrict, DefaultVerifyStrict) }
