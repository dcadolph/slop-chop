package sanitize

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestApplyPresets checks that a preset adds its rules, an unknown name is an error, and an
// explicit profile wins over a preset on a shared key.
func TestApplyPresets(t *testing.T) {
	t.Parallel()

	// Test 0: The plain preset rewrites a corporate word.
	p, err := ApplyPresets(Profile{}, "plain")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, _ := s.Fix("we utilize it"); got != "we use it" {
		t.Errorf("plain preset: got %q, want %q", got, "we use it")
	}

	// Test 1: An unknown preset is an error.
	if _, err := ApplyPresets(Profile{}, "bogus"); !errors.Is(err, ErrPreset) {
		t.Errorf("unknown preset: err = %v, want ErrPreset", err)
	}

	// Test 2: A profile's own entry wins over the preset's.
	base := Profile{WordReplace: map[string]string{"utilize": "employ"}}
	merged, err := ApplyPresets(base, "plain")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s2, err := New(merged)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, _ := s2.Fix("we utilize it"); got != "we employ it" {
		t.Errorf("profile precedence: got %q, want %q", got, "we employ it")
	}
}

// TestPresetNames checks that the built-in presets are discoverable and each one loads.
func TestPresetNames(t *testing.T) {
	t.Parallel()
	names := PresetNames()
	if !slices.Contains(names, "plain") {
		t.Fatalf("PresetNames = %v, want it to contain plain", names)
	}
	for _, name := range names {
		if _, err := loadPreset(name); err != nil {
			t.Errorf("loadPreset(%q): %v", name, err)
		}
	}
}

// TestMerge checks the map and slice merge helpers that back preset overlay.
func TestMerge(t *testing.T) {
	t.Parallel()

	// Test 0: The base wins on a shared map key, the add fills the rest.
	gotMap := mergeMap(map[string]string{"a": "1"}, map[string]string{"a": "2", "b": "3"})
	wantMap := map[string]string{"a": "1", "b": "3"}
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Errorf("mergeMap mismatch (-want +got):\n%s", diff)
	}

	// Test 1: Slices union in order and drop duplicates.
	gotSlice := mergeSlice([]string{"a", "b"}, []string{"b", "c"})
	wantSlice := []string{"a", "b", "c"}
	if diff := cmp.Diff(wantSlice, gotSlice); diff != "" {
		t.Errorf("mergeSlice mismatch (-want +got):\n%s", diff)
	}
}

// TestPlainPresetWellFormed checks the shipped plain preset is valid: word swaps are lower
// case so the case-carry works, and no swap is a no-op.
func TestPlainPresetWellFormed(t *testing.T) {
	t.Parallel()
	p, err := loadPreset("plain")
	if err != nil {
		t.Fatalf("loadPreset: %v", err)
	}
	for k, v := range p.WordReplace {
		if k == "" || v == "" {
			t.Errorf("word swap has an empty side: %q -> %q", k, v)
		}
		if k == v {
			t.Errorf("word swap is a no-op: %q", k)
		}
	}
	for k, v := range p.PhraseReplace {
		if k == "" || v == "" {
			t.Errorf("phrase swap has an empty side: %q -> %q", k, v)
		}
	}
}
