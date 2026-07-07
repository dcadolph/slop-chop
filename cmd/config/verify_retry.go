package config

import (
	"strconv"

	"github.com/spf13/pflag"
)

// KeyVerifyRetry is the verify-retry flag name.
const KeyVerifyRetry = "verify-retry"

// DefaultVerifyRetry is 0, meaning no re-rewrite when the meaning check flags a change.
//
//nolint:gochecknoglobals // Flag definition.
var DefaultVerifyRetry = 0

// FlagVerifyRetry sets how many times to re-rewrite when the meaning check flags a change,
// feeding the flagged issues back to the model each time.
//
//nolint:gochecknoglobals // Flag definition.
var FlagVerifyRetry = pflag.Flag{
	Name:     KeyVerifyRetry,
	Usage:    "With --verify, re-rewrite up to N times when the meaning check flags a change.",
	Value:    &FlagValue{Val: strconv.Itoa(DefaultVerifyRetry), ValType: "int"},
	DefValue: strconv.Itoa(DefaultVerifyRetry),
}

// VerifyRetry returns how many re-rewrites to attempt when the meaning check flags a
// change. A negative value is treated as zero.
func VerifyRetry() int {
	if n := loadInt(KeyVerifyRetry, DefaultVerifyRetry); n > 0 {
		return n
	}
	return 0
}
