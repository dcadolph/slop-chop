package cmd

import (
	"strings"
	"testing"
)

// TestFixPresetFlag checks that --preset applies a built-in pack in fix mode.
func TestFixPresetFlag(t *testing.T) {
	out, _, err := runCLI(t, []string{"fix", "--preset", "plain"}, "we utilize it")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if out != "we use it" {
		t.Errorf("out = %q, want %q", out, "we use it")
	}
}

// TestPresetUnknown checks that an unknown preset is a clear error naming what exists.
func TestPresetUnknown(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "--preset", "bogus"}, "text")
	if err == nil || !strings.Contains(err.Error(), "unknown preset") {
		t.Errorf("err = %v, want an unknown-preset error", err)
	}
}

// TestPresetProfilePrecedence checks that a user profile overrides a preset on a shared key.
func TestPresetProfilePrecedence(t *testing.T) {
	dir := t.TempDir()
	prof := writeTemp(t, dir, "p.json", `{"wordReplace": {"utilize": "employ"}}`)
	out, _, err := runCLI(t, []string{"fix", "--preset", "plain", "--profile", prof}, "we utilize it")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if out != "we employ it" {
		t.Errorf("out = %q, want %q", out, "we employ it")
	}
}

// TestPresetEnv checks that the environment variable applies a preset with no flag passed.
func TestPresetEnv(t *testing.T) {
	t.Setenv("SLOP_CHOP_PRESET", "plain")
	out, _, err := runCLI(t, []string{"fix"}, "we utilize it")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if out != "we use it" {
		t.Errorf("out = %q, want %q", out, "we use it")
	}
}
