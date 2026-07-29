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
		{Preset: "cleaver", In: "We leverage robust, seamless workflows.", Want: "We use solid, smooth workflows."},
		{
			Preset: "cleaver",
			In:     "This empowers teams to deep-dive into actionable insights.",
			Want:   "This helps teams to dig into useful findings.",
		},
		{Preset: "cleaver", In: "In today's digital-first landscape, delve deeper.", Want: "Dig deeper."},
		{Preset: "cleaver", In: "Leveraging it unlocks the full potential of the team.", Want: "Using it gets the most from the team."},
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

// TestCleaverCleanup checks the cleaver preset on top of the default profile, the way
// the web app and CLI compose it. It covers word drops that must not leave debris and
// the swaps that were tightened so a rewrite never reads worse than the slop it cut.
func TestCleaverCleanup(t *testing.T) {
	t.Parallel()
	p, err := ApplyPresets(DefaultProfile(), "cleaver")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		In   string
		Want string
	}{
		// Test 0: A dropped adverb mid-sentence leaves no double space.
		{In: "It integrates seamlessly with the stack.", Want: "It integrates with the stack."},
		// Test 1: A dropped opener restores the sentence's capital.
		{In: "Seamlessly the app scales.", Want: "The app scales."},
		// Test 2: A dropped opener with a trailing comma leaves a clean start.
		{In: "Seamlessly, the app scales.", Want: "The app scales."},
		// Test 3: An em-dash swap plus a drop leaves no comma before the period.
		{In: "We ship it—seamlessly.", Want: "We ship it."},
		// Test 4: A drop between two commas does not leave a double comma.
		{In: "It is known, seamlessly, for speed.", Want: "It is known, for speed."},
		// Test 5: A recap fires after a sentence that ends mid-line.
		{In: "We built it. Seamlessly it scales.", Want: "We built it. It scales."},
		// Test 6: The "a myriad of" phrase overrides the bare word so it is not "a many of".
		{In: "A myriad of options exist.", Want: "Many options exist."},
		// Test 7: revolutionize drops its hype to a plain verb.
		{In: "This will revolutionize the API.", Want: "This will change the API."},
		// Test 8: forward-thinking swaps to a consonant word so the article stays correct.
		{In: "A forward-thinking company.", Want: "A modern company."},
		// Test 9: A role-template phrase collapses to a plain one.
		{In: "It plays a crucial role in growth.", Want: "It is key to growth."},
		// Test 10: The passive "empowered to" idiom reads cleanly.
		{In: "Users are empowered to decide.", Want: "Users are able to decide."},
		// Test 11: Ordinary list commas are left alone.
		{In: "We sell apples, oranges, and pears.", Want: "We sell apples, oranges, and pears."},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("Fix(%q) mismatch (-want +got):\n%s", test.In, diff)
			}
		})
	}
}
