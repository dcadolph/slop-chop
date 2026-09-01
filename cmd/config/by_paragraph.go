package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyByParagraph is the by-paragraph flag name.
const KeyByParagraph = "by-paragraph"

// DefaultByParagraph is false, meaning one score for the whole input.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultByParagraph = false

// FlagByParagraph scores each paragraph separately, so a mixed document shows where the
// machine wrote.
//
//nolint:gochecknoglobals // Flag definition.
var FlagByParagraph = pflag.Flag{
	Name:        KeyByParagraph,
	Usage:       "Score each paragraph separately to locate the slop in a mixed document.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultByParagraph), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultByParagraph),
	NoOptDefVal: "true",
}

// ByParagraph reports whether score should break the input down by paragraph.
func ByParagraph() bool { return loadBool(KeyByParagraph, DefaultByParagraph) }
