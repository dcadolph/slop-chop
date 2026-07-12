package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyMarkdown is the markdown flag name.
const KeyMarkdown = "markdown"

// DefaultMarkdown is false, meaning plain text output.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultMarkdown = false

// FlagMarkdown switches the tells command to the markdown catalog page.
//
//nolint:gochecknoglobals // Flag definition.
var FlagMarkdown = pflag.Flag{
	Name:        KeyMarkdown,
	Usage:       "Write the catalog as a markdown page.",
	Value:       &FlagValue{Val: strconv.FormatBool(DefaultMarkdown), ValType: "bool"},
	DefValue:    strconv.FormatBool(DefaultMarkdown),
	NoOptDefVal: "true",
}

// Markdown reports whether output should be the markdown catalog page.
func Markdown() bool { return loadBool(KeyMarkdown, DefaultMarkdown) }
