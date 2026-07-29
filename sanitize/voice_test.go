package sanitize

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestWithVoice checks how a voice layers onto a profile: prefer swaps win, keep silences a
// cut, an empty prefer target drops a word, and an empty voice changes nothing.
func TestWithVoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name    string
		Base    Profile
		Voice   Voice
		In      string
		WantOut string
	}{{ // Test 0: prefer overrides a preset word swap.
		Name:  "prefer word",
		Base:  Profile{WordReplace: map[string]string{"robust": "solid"}},
		Voice: Voice{Prefer: map[string]string{"robust": "sturdy"}},
		In:    "a robust tool", WantOut: "a sturdy tool",
	}, { // Test 1: prefer overrides a preset phrase swap.
		Name:  "prefer phrase",
		Base:  Profile{PhraseReplace: map[string]string{"a myriad of": "many"}},
		Voice: Voice{Prefer: map[string]string{"a myriad of": "a bunch of"}},
		In:    "a myriad of tools", WantOut: "a bunch of tools",
	}, { // Test 2: keep suppresses a preset swap, so the word survives.
		Name:  "keep suppresses swap",
		Base:  Profile{WordReplace: map[string]string{"robust": "solid"}},
		Voice: Voice{Keep: []string{"robust"}},
		In:    "a robust tool", WantOut: "a robust tool",
	}, { // Test 3: prefer with an empty replacement drops the word.
		Name:  "prefer drops",
		Base:  Profile{CollapseSpaces: true},
		Voice: Voice{Prefer: map[string]string{"basically": ""}},
		In:    "this is basically fine", WantOut: "this is fine",
	}, { // Test 4: an empty voice leaves the base behavior intact.
		Name:  "empty voice",
		Base:  Profile{WordReplace: map[string]string{"robust": "solid"}},
		Voice: Voice{},
		In:    "a robust tool", WantOut: "a solid tool",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			s, err := New(test.Base.WithVoice(test.Voice))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantOut, got); diff != "" {
				t.Errorf("Fix mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestVoiceAvoidFlags checks that an avoid word is reported as a tell.
func TestVoiceAvoidFlags(t *testing.T) {
	t.Parallel()
	s, err := New(Profile{}.WithVoice(Voice{Avoid: []string{"synergy"}}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	findings := s.Check("pure synergy here")
	hit := slices.ContainsFunc(findings, func(f Finding) bool {
		return strings.Contains(strings.ToLower(fmt.Sprint(f)), "synergy")
	})
	if !hit {
		t.Errorf("avoid: findings = %v, want a synergy flag", findings)
	}
}

// TestVoiceOverlayPrecedence checks project over voice over preset: a voice beats a preset
// swap, and a project profile re-applied on top beats the voice.
func TestVoiceOverlayPrecedence(t *testing.T) {
	t.Parallel()
	preset := Profile{WordReplace: map[string]string{"utilize": "employ"}}
	withVoice := preset.WithVoice(Voice{Prefer: map[string]string{"utilize": "use"}})

	// Voice beats the preset when nothing else overrides the key.
	sVoice, err := New(withVoice)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, _ := sVoice.Fix("we utilize it"); got != "we use it" {
		t.Errorf("voice over preset: got %q, want %q", got, "we use it")
	}

	// A project profile re-applied on top of the voice wins.
	project := Profile{WordReplace: map[string]string{"utilize": "consume"}}
	sProject, err := New(withVoice.Overlay(project))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, _ := sProject.Fix("we utilize it"); got != "we consume it" {
		t.Errorf("project over voice: got %q, want %q", got, "we consume it")
	}
}

// TestVoiceAsProfile checks the mapping from voice lists to profile fields, including the
// single-word versus phrase split and the skip of an empty key.
func TestVoiceAsProfile(t *testing.T) {
	t.Parallel()
	v := Voice{
		Keep:   []string{"alpha"},
		Avoid:  []string{"beta"},
		Prefer: map[string]string{"one": "1", "two words": "2", "": "skip", "drop": ""},
		Tone:   []string{"dry humor"},
	}
	want := Profile{
		Allow:         []string{"alpha"},
		BlockWords:    []string{"beta"},
		WordReplace:   map[string]string{"one": "1", "drop": ""},
		PhraseReplace: map[string]string{"two words": "2"},
		Tone:          []string{"dry humor"},
	}
	if diff := cmp.Diff(want, v.asProfile(), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("asProfile mismatch (-want +got):\n%s", diff)
	}
}

// TestLoadVoice checks decoding a voice, an error on malformed JSON, and the Empty helper.
func TestLoadVoice(t *testing.T) {
	t.Parallel()

	// Test 0: a valid voice decodes into its fields.
	got, err := LoadVoice(strings.NewReader(`{"keep":["x"],"prefer":{"a":"b"},"avoid":["c"]}`))
	if err != nil {
		t.Fatalf("LoadVoice: %v", err)
	}
	want := Voice{Keep: []string{"x"}, Prefer: map[string]string{"a": "b"}, Avoid: []string{"c"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LoadVoice mismatch (-want +got):\n%s", diff)
	}

	// Test 1: malformed JSON is an error.
	if _, err := LoadVoice(strings.NewReader("{bad")); err == nil {
		t.Errorf("LoadVoice(malformed): err = nil, want a decode error")
	}

	// Test 2: Empty reports whether the voice sets anything.
	if !(Voice{}).Empty() {
		t.Errorf("zero Voice: Empty = false, want true")
	}
	if (Voice{Keep: []string{"x"}}).Empty() {
		t.Errorf("Voice with a keep entry: Empty = true, want false")
	}
	if (Voice{Tone: []string{"x"}}).Empty() {
		t.Errorf("Voice with a tone entry: Empty = true, want false")
	}
}

// TestVoiceToneFlows checks that a voice's tone lines land on the profile and union with
// what a project overlay carries, so the rewrite prompt sees both.
func TestVoiceToneFlows(t *testing.T) {
	t.Parallel()
	base := Profile{Tone: []string{"house style"}}
	got := base.WithVoice(Voice{Tone: []string{"dry humor", "house style"}})
	want := []string{"house style", "dry humor"}
	if diff := cmp.Diff(want, got.Tone); diff != "" {
		t.Errorf("tone mismatch (-want +got):\n%s", diff)
	}
}
