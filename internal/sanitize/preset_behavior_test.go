package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestPresetBehavior checks that each built-in preset actually rewrites the phrasing it
// targets. The presets are applied to an empty profile so the test isolates the preset's
// own rules from the default profile.
func TestPresetBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Preset string
		In     string
		Want   string
	}{
		{Preset: "corporate", In: "We utilize synergy.", Want: "We use synergy."},
		{Preset: "corporate", In: "Let's circle back.", Want: "Let's follow up."},
		{Preset: "academic", In: "We utilize a demonstrate step.", Want: "We use a show step."},
		{Preset: "marketing", In: "A bespoke, curated kit.", Want: "A custom, chosen kit."},
	}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Preset), func(t *testing.T) {
			t.Parallel()
			p, err := ApplyPresets(Profile{}, test.Preset)
			if err != nil {
				t.Fatalf("ApplyPresets(%q): %v", test.Preset, err)
			}
			s, err := New(p)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("%s Fix mismatch (-want +got):\n%s", test.Preset, diff)
			}
		})
	}
}
