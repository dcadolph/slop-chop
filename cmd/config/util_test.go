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

// TestSet checks that Set reports a value coming from a flag or the environment, but not the
// bare default, so an env-only setting like SLOP_CHOP_MODEL is not mistaken for unset.
func TestSet(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	// Test 0: Nothing set: Set is false even though Model() returns the default.
	if Set(KeyModel) {
		t.Error("Set(model) = true with nothing set, want false")
	}

	// Test 1: An environment value counts as set, where Changed (flag-only) would not.
	t.Setenv("SLOP_CHOP_MODEL", "env-model")
	if !Set(KeyModel) {
		t.Error("Set(model) = false with env set, want true")
	}
	if Changed(KeyModel) {
		t.Error("Changed(model) = true with only env set, want false")
	}

	// Test 2: A set flag also counts.
	Reset()
	if err := FlagModel.Value.Set("flag-model"); err != nil {
		t.Fatal(err)
	}
	FlagModel.Changed = true
	if !Set(KeyModel) {
		t.Error("Set(model) = false with flag set, want true")
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

// TestDialectPresetPriority checks that the dialect and preset accessors resolve in the
// order flag, then environment, then the empty default that lets a profile stand.
func TestDialectPresetPriority(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	// Test 0: Unset, both are empty so a profile's own value wins downstream.
	if Dialect() != "" {
		t.Errorf("Dialect() = %q, want empty default", Dialect())
	}
	if Preset() != "" {
		t.Errorf("Preset() = %q, want empty default", Preset())
	}

	// Test 1: The environment sets each.
	t.Setenv("SLOP_CHOP_DIALECT", "british")
	t.Setenv("SLOP_CHOP_PRESET", "plain")
	if Dialect() != "british" {
		t.Errorf("Dialect() = %q, want env british", Dialect())
	}
	if Preset() != "plain" {
		t.Errorf("Preset() = %q, want env plain", Preset())
	}

	// Test 2: A set flag overrides the environment.
	if err := FlagDialect.Value.Set("american"); err != nil {
		t.Fatal(err)
	}
	FlagDialect.Changed = true
	if Dialect() != "american" {
		t.Errorf("Dialect() = %q, want flag american", Dialect())
	}
}
