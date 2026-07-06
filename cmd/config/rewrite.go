package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyRewrite is the rewrite flag name.
const KeyRewrite = "rewrite"

// DefaultRewrite is false, meaning rules only.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultRewrite = false

// FlagRewrite runs the model rewrite pass after the rules pass.
//
//nolint:gochecknoglobals // Flag definition.
var FlagRewrite = pflag.Flag{
	Name:        KeyRewrite,
	Usage:       "Send the rules output to a model for a deeper clean.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultRewrite), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultRewrite),
	NoOptDefVal: "true",
}

// Rewrite reports whether the model rewrite pass should run.
func Rewrite() bool { return loadBool(KeyRewrite, DefaultRewrite) }
