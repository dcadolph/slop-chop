package config

import "github.com/spf13/pflag"

// Judge flag names. Each falls back to its rewrite counterpart when unset, so the meaning
// check runs on the rewriter's backend unless a separate judge is configured.
const (
	KeyJudgeProvider = "judge-provider"
	KeyJudgeModel    = "judge-model"
	KeyJudgeBaseURL  = "judge-base-url"
)

// Judge flag defaults are empty, which means inherit the rewrite setting.
//
//nolint:gochecknoglobals // Flag definitions.
var (
	DefaultJudgeProvider = ""
	DefaultJudgeModel    = ""
	DefaultJudgeBaseURL  = ""

	// FlagJudgeProvider picks the backend for the meaning check.
	FlagJudgeProvider = pflag.Flag{
		Name:     KeyJudgeProvider,
		Usage:    "Backend for the meaning check: anthropic or openai (default: the --provider).",
		Value:    &FlagValue{Val: DefaultJudgeProvider, ValType: "string"},
		DefValue: DefaultJudgeProvider,
	}
	// FlagJudgeModel picks the model for the meaning check, so a judge distinct from the
	// rewriter grades the rewrite.
	FlagJudgeModel = pflag.Flag{
		Name:     KeyJudgeModel,
		Usage:    "Model for the meaning check (default: the --model). Set it so the rewriter does not grade its own work.",
		Value:    &FlagValue{Val: DefaultJudgeModel, ValType: "string"},
		DefValue: DefaultJudgeModel,
	}
	// FlagJudgeBaseURL points the meaning check at an OpenAI-compatible server.
	FlagJudgeBaseURL = pflag.Flag{
		Name:     KeyJudgeBaseURL,
		Usage:    "Base URL for an OpenAI-compatible judge (default: the --base-url).",
		Value:    &FlagValue{Val: DefaultJudgeBaseURL, ValType: "string"},
		DefValue: DefaultJudgeBaseURL,
	}
)

// JudgeProvider returns the configured judge backend, or empty to inherit the rewrite one.
func JudgeProvider() string { return loadString(KeyJudgeProvider, DefaultJudgeProvider) }

// JudgeModel returns the configured judge model, or empty to inherit the rewrite one.
func JudgeModel() string { return loadString(KeyJudgeModel, DefaultJudgeModel) }

// JudgeBaseURL returns the configured judge base URL, or empty to inherit the rewrite one.
func JudgeBaseURL() string { return loadString(KeyJudgeBaseURL, DefaultJudgeBaseURL) }
