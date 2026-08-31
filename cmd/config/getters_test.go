package config

import (
	"fmt"
	"testing"
)

// Tests run serially because flag state is package-global and t.Setenv forbids
// t.Parallel.

// TestStringGetters checks every string getter against its default and an environment
// override, so the flag-then-env-then-default plumbing holds for each flag.
func TestStringGetters(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	tests := []struct {
		Get  func() string
		Env  string
		Def  string
		Name string
	}{
		{Get: Profile, Env: "SLOP_CHOP_PROFILE", Def: DefaultProfile, Name: "profile"},
		{Get: Voice, Env: "SLOP_CHOP_VOICE", Def: DefaultVoice, Name: "voice"},
		{Get: Provider, Env: "SLOP_CHOP_PROVIDER", Def: DefaultProvider, Name: "provider"},
		{Get: BaseURL, Env: "SLOP_CHOP_BASE_URL", Def: DefaultBaseURL, Name: "base-url"},
		{Get: JudgeProvider, Env: "SLOP_CHOP_JUDGE_PROVIDER", Def: DefaultJudgeProvider, Name: "judge-provider"},
		{Get: JudgeModel, Env: "SLOP_CHOP_JUDGE_MODEL", Def: DefaultJudgeModel, Name: "judge-model"},
		{Get: JudgeBaseURL, Env: "SLOP_CHOP_JUDGE_BASE_URL", Def: DefaultJudgeBaseURL, Name: "judge-base-url"},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			Reset()
			if got := test.Get(); got != test.Def {
				t.Errorf("default = %q, want %q", got, test.Def)
			}
			t.Setenv(test.Env, "from-env")
			if got := test.Get(); got != "from-env" {
				t.Errorf("with env = %q, want %q", got, "from-env")
			}
		})
	}
}

// TestBoolGetters checks every bool getter against its default, an environment override,
// and an unparsable value falling back to the default.
func TestBoolGetters(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	tests := []struct {
		Get  func() bool
		Env  string
		Def  bool
		Name string
	}{
		{Get: Pretty, Env: "SLOP_CHOP_PRETTY", Def: DefaultPretty, Name: "pretty"},
		{Get: Markdown, Env: "SLOP_CHOP_MARKDOWN", Def: DefaultMarkdown, Name: "markdown"},
		{Get: Write, Env: "SLOP_CHOP_WRITE", Def: DefaultWrite, Name: "write"},
		{Get: Rewrite, Env: "SLOP_CHOP_REWRITE", Def: DefaultRewrite, Name: "rewrite"},
		{Get: Verify, Env: "SLOP_CHOP_VERIFY", Def: DefaultVerify, Name: "verify"},
		{Get: VerifyStrict, Env: "SLOP_CHOP_VERIFY_STRICT", Def: DefaultVerifyStrict, Name: "verify-strict"},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			Reset()
			if got := test.Get(); got != test.Def {
				t.Errorf("default = %v, want %v", got, test.Def)
			}
			t.Setenv(test.Env, "true")
			if got := test.Get(); !got {
				t.Errorf("with env true = %v, want true", got)
			}
			t.Setenv(test.Env, "not-a-bool")
			if got := test.Get(); got != test.Def {
				t.Errorf("with junk env = %v, want the default %v", got, test.Def)
			}
		})
	}
}

// TestIntGetters checks Max and VerifyRetry: defaults, environment overrides, junk
// falling back, and VerifyRetry clamping negatives to zero.
func TestIntGetters(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	// Test 0: Max default is the gate-off sentinel.
	if got := Max(); got != DefaultMax {
		t.Errorf("Max default = %d, want %d", got, DefaultMax)
	}
	// Test 1: Max reads the environment.
	t.Setenv("SLOP_CHOP_MAX", "42")
	if got := Max(); got != 42 {
		t.Errorf("Max with env = %d, want 42", got)
	}
	// Test 2: junk falls back to the default.
	t.Setenv("SLOP_CHOP_MAX", "not-a-number")
	if got := Max(); got != DefaultMax {
		t.Errorf("Max with junk env = %d, want %d", got, DefaultMax)
	}
	// Test 3: VerifyRetry clamps a negative to zero.
	t.Setenv("SLOP_CHOP_VERIFY_RETRY", "-3")
	if got := VerifyRetry(); got != 0 {
		t.Errorf("VerifyRetry(-3) = %d, want 0", got)
	}
	// Test 4: VerifyRetry passes a positive through.
	t.Setenv("SLOP_CHOP_VERIFY_RETRY", "2")
	if got := VerifyRetry(); got != 2 {
		t.Errorf("VerifyRetry(2) = %d, want 2", got)
	}
}

// TestFlagValue checks Set validation per value type and the pflag type name.
func TestFlagValue(t *testing.T) {
	tests := []struct {
		In      string
		ValType string
		WantErr bool
	}{{ // Test 0: A valid bool parses.
		In: "true", ValType: "bool", WantErr: false,
	}, { // Test 1: An invalid bool is rejected.
		In: "maybe", ValType: "bool", WantErr: true,
	}, { // Test 2: A valid int parses.
		In: "7", ValType: "int", WantErr: false,
	}, { // Test 3: An invalid int is rejected.
		In: "seven", ValType: "int", WantErr: true,
	}, { // Test 4: A string takes anything.
		In: "anything at all", ValType: "string", WantErr: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			v := &FlagValue{ValType: test.ValType}
			err := v.Set(test.In)
			if (err != nil) != test.WantErr {
				t.Errorf("Set(%q) err = %v, wantErr %v", test.In, err, test.WantErr)
			}
			if !test.WantErr && v.String() != test.In {
				t.Errorf("String() = %q, want %q", v.String(), test.In)
			}
			if got := v.Type(); got != test.ValType {
				t.Errorf("Type() = %q, want %q", got, test.ValType)
			}
		})
	}
}
