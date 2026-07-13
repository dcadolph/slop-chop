package config

import "github.com/spf13/pflag"

// flags indexes every flag by key so lookups and resets can walk them.
//
//nolint:gochecknoglobals // Flag registry, written once at init.
var flags = map[string]*pflag.Flag{
	KeyProfile:       &FlagProfile,
	KeyDialect:       &FlagDialect,
	KeyPreset:        &FlagPreset,
	KeyVoice:         &FlagVoice,
	KeyJSON:          &FlagJSON,
	KeyPretty:        &FlagPretty,
	KeyMarkdown:      &FlagMarkdown,
	KeyWrite:         &FlagWrite,
	KeyRewrite:       &FlagRewrite,
	KeyProvider:      &FlagProvider,
	KeyModel:         &FlagModel,
	KeyBaseURL:       &FlagBaseURL,
	KeyJudgeProvider: &FlagJudgeProvider,
	KeyJudgeModel:    &FlagJudgeModel,
	KeyJudgeBaseURL:  &FlagJudgeBaseURL,
	KeyVerify:        &FlagVerify,
	KeyVerifyStrict:  &FlagVerifyStrict,
	KeyVerifyRetry:   &FlagVerifyRetry,
	KeyMax:           &FlagMax,
}

// Changed reports whether the flag for key was set on the command line.
func Changed(key string) bool {
	f, ok := flags[key]
	return ok && f.Changed
}

// Reset puts every flag back to its default. Tests use it between command runs because
// flag state is shared package-wide.
func Reset() {
	for _, f := range flags {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
}
