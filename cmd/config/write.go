package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyWrite is the write flag name.
const KeyWrite = "write"

// DefaultWrite is false, meaning results go to stdout.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultWrite = false

// FlagWrite saves the result back to each file instead of writing to stdout.
//
//nolint:gochecknoglobals // Flag definition.
var FlagWrite = pflag.Flag{
	Name:        KeyWrite,
	Shorthand:   "w",
	Usage:       "Write the result back to the file instead of stdout.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultWrite), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultWrite),
	NoOptDefVal: "true",
}

// Write reports whether results should be written back to their files.
func Write() bool { return loadBool(KeyWrite, DefaultWrite) }
