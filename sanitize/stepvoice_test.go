package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// voiceMatches returns the match of every step voice finding in findings, in order.
func voiceMatches(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		if f.Rule == voiceStepRule {
			out = append(out, f.Match)
		}
	}
	return out
}

// TestStepVoice checks the passive step rule under the procedure preset.
func TestStepVoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantMatches []string
		In          string
	}{{ // Test 0: A passive step is flagged on its verb phrase.
		In: "1. The file is copied to /etc.\n", WantMatches: []string{"is copied"},
	}, { // Test 1: An imperative step is not flagged.
		In: "1. Copy the file to /etc.\n", WantMatches: nil,
	}, { // Test 2: A passive sentence outside a list is a description, not a step.
		In: "The file is copied to /etc.\n", WantMatches: nil,
	}, { // Test 3: "is needed" reads as passive and is flagged, a choice made on purpose,
		// since a step that says what is needed hides who supplies it.
		In: "- A restart is needed after the upgrade.\n", WantMatches: []string{"is needed"},
	}, { // Test 4: "is required" is flagged for the same reason as "is needed".
		In: "- A token is required.\n", WantMatches: []string{"is required"},
	}, { // Test 5: One word may sit between the auxiliary and the participle.
		In: "- The cache is then cleared.\n", WantMatches: []string{"is then cleared"},
	}, { // Test 6: An irregular participle is flagged.
		In: "* The key was written to disk.\n", WantMatches: []string{"was written"},
	}, { // Test 7: A "get" passive is flagged.
		In: "1. The job gets started by cron.\n", WantMatches: []string{"gets started"},
	}, { // Test 8: A determiner between the auxiliary and an -ed word marks a noun.
		In: "1. There is a speed limit.\n", WantMatches: nil,
	}, { // Test 9: A short -ed word is not a participle.
		In: "1. The light is red.\n", WantMatches: nil,
	}, { // Test 10: Only the first sentence of a step counts.
		In: "1. Copy the file. It is read at boot.\n", WantMatches: nil,
	}, { // Test 11: A passive inside inline code is protected.
		In: "1. Run `it is copied` once.\n", WantMatches: nil,
	}, { // Test 12: An ignore directive silences the step.
		In: "<!-- slop-chop-ignore-next-line -->\n1. The file is copied to /etc.\n", WantMatches: nil,
	}, { // Test 13: Each passive step gets its own finding.
		In:          "1. The file is copied.\n2. Restart the service.\n3. Logs are kept.\n",
		WantMatches: []string{"is copied", "are kept"},
	}, { // Test 14: A task-list step is a step.
		In: "- [ ] The backup is verified.\n", WantMatches: []string{"is verified"},
	}}
	s := procedureSanitizer(t)
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := voiceMatches(s.Check(test.In))
			if diff := cmp.Diff(test.WantMatches, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("voice matches mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStepVoiceGate checks the rule stays silent unless the profile switches it on, and
// that the allow list silences it like any other rule.
func TestStepVoiceGate(t *testing.T) {
	t.Parallel()
	const in = "1. The file is copied to /etc.\n"
	procedure, err := ApplyPresets(DefaultProfile(), "procedure")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	allowed := procedure
	allowed.Allow = append([]string{"copied"}, procedure.Allow...)
	tests := []struct {
		Profile     Profile
		WantMatches []string
	}{{ // Test 0: The default profile leaves the rule off.
		Profile: DefaultProfile(), WantMatches: nil,
	}, { // Test 1: The procedure preset switches it on.
		Profile: procedure, WantMatches: []string{"is copied"},
	}, { // Test 2: An allow-listed participle silences the match.
		Profile: allowed, WantMatches: nil,
	}, { // Test 3: A profile can switch it on without the preset.
		Profile: Profile{StepVoice: true}, WantMatches: []string{"is copied"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(test.Profile)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got := voiceMatches(s.Check(in))
			if diff := cmp.Diff(test.WantMatches, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("voice matches mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStepVoiceWeight checks a step voice finding carries no score weight, so the procedure
// preset never moves a score through it, and that its reason reads as the rule intends.
func TestStepVoiceWeight(t *testing.T) {
	t.Parallel()
	const in = "1. The file is copied to /etc.\n2. Logs are kept.\n"
	base, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s := procedureSanitizer(t)
	if w := s.Weight(voiceStepRule); w != 0 {
		t.Errorf("Weight(%q) = %v, want 0", voiceStepRule, w)
	}
	if got, want := s.Score(in).Value, base.Score(in).Value; got != want {
		t.Errorf("procedure score = %d, default score = %d, want equal", got, want)
	}
	const wantReason = "A step in the passive voice. Procedures read best as an instruction: say who does what."
	if diff := cmp.Diff(wantReason, Reason(voiceStepRule)); diff != "" {
		t.Errorf("Reason mismatch (-want +got):\n%s", diff)
	}
}

// TestStepVoiceMerge checks the stepVoice switch survives a preset merge and an overlay,
// and that neither can switch it off.
func TestStepVoiceMerge(t *testing.T) {
	t.Parallel()
	on := Profile{StepVoice: true}
	tests := []struct {
		Got  Profile
		Want bool
	}{{ // Test 0: A preset switches it on.
		Got: Profile{}.withPreset(on), Want: true,
	}, { // Test 1: A preset without it leaves it on.
		Got: on.withPreset(Profile{}), Want: true,
	}, { // Test 2: An overlay switches it on.
		Got: Profile{}.Overlay(on), Want: true,
	}, { // Test 3: An overlay without it leaves it on.
		Got: on.Overlay(Profile{}), Want: true,
	}, { // Test 4: Neither side set leaves it off.
		Got: Profile{}.withPreset(Profile{}), Want: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, test.Got.StepVoice); diff != "" {
				t.Errorf("StepVoice mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestFlagClass checks the flag-only rules sort into their own classes for deduplication.
func TestFlagClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantClass string
		Rule      string
	}{{ // Test 0: A length rule.
		Rule: "length:step", WantClass: "length",
	}, { // Test 1: A step voice rule.
		Rule: "voice:step", WantClass: "voice",
	}, { // Test 2: Any other rule.
		Rule: "word:robust", WantClass: "",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.WantClass, flagClass(test.Rule)); diff != "" {
				t.Errorf("flagClass mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
