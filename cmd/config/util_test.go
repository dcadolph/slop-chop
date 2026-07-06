package config

import "testing"

// Tests run serially because flag state is package-global and t.Setenv forbids
// t.Parallel.

// TestEnvKey checks the flag-name to environment-variable mapping.
func TestEnvKey(t *testing.T) {
	if got := envKey("output-dir"); got != "SLOP_CHOP_OUTPUT_DIR" {
		t.Errorf("envKey = %q, want %q", got, "SLOP_CHOP_OUTPUT_DIR")
	}
}

// TestLoadPriority checks the resolution order: flag beats environment beats default.
func TestLoadPriority(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	// Test 0: Nothing set returns the default.
	if JSON() {
		t.Error("JSON() = true, want default false")
	}
	if Model() != DefaultModel {
		t.Errorf("Model() = %q, want default %q", Model(), DefaultModel)
	}

	// Test 1: The environment overrides the default.
	t.Setenv("SLOP_CHOP_JSON", "true")
	t.Setenv("SLOP_CHOP_MODEL", "env-model")
	if !JSON() {
		t.Error("JSON() = false, want env true")
	}
	if Model() != "env-model" {
		t.Errorf("Model() = %q, want env-model", Model())
	}

	// Test 2: A set flag overrides the environment.
	t.Setenv("SLOP_CHOP_JSON", "false")
	if err := FlagJSON.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	FlagJSON.Changed = true
	if !JSON() {
		t.Error("JSON() = false, want flag true")
	}

	// Test 3: A bad environment bool falls back to the default.
	Reset()
	t.Setenv("SLOP_CHOP_JSON", "garbage")
	if JSON() {
		t.Error("JSON() = true, want default false on bad env value")
	}
}

// TestChangedAndReset checks flag-set detection and the reset between runs.
func TestChangedAndReset(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if Changed(KeyModel) {
		t.Error("Changed = true before any set")
	}
	if err := FlagModel.Value.Set("custom"); err != nil {
		t.Fatal(err)
	}
	FlagModel.Changed = true
	if !Changed(KeyModel) {
		t.Error("Changed = false after set")
	}
	if Model() != "custom" {
		t.Errorf("Model() = %q, want custom", Model())
	}

	Reset()
	if Changed(KeyModel) {
		t.Error("Changed = true after Reset")
	}
	if Model() != DefaultModel {
		t.Errorf("Model() = %q, want default %q after Reset", Model(), DefaultModel)
	}
}
