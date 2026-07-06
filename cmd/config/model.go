package config

import (
	"github.com/spf13/pflag"

	"github.com/dcadolph/slop-chop/internal/rewrite"
)

// KeyModel is the model flag name.
const KeyModel = "model"

// DefaultModel is the model the rewrite pass uses when none is set.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultModel = rewrite.DefaultModel

// FlagModel picks the model for the rewrite pass.
//
//nolint:gochecknoglobals // Flag definition.
var FlagModel = pflag.Flag{
	Name:     KeyModel,
	Usage:    "Model for --rewrite (default " + rewrite.DefaultModel + ").",
	Value:    &FlagValue{Val: rewrite.DefaultModel, ValType: "string"},
	DefValue: rewrite.DefaultModel,
}

// Model returns the model id for the rewrite pass.
func Model() string { return loadString(KeyModel, DefaultModel) }
