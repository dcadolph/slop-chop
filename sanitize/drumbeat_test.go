package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestLandingDrumbeat checks that the landing habit is flagged only once it is a habit.
// The negatives carry this rule: a writer snapping a sentence shut on "that's it" is
// doing something old and deliberate, so everything here turns on the rate.
func TestLandingDrumbeat(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name    string
		In      string
		WantHit bool
	}{{ // Test 0: Four contracted landings in a short passage.
		Name: "contracted drumbeat", WantHit: true,
		In: "The proposal is sound. That's the point. It's also cheap. That's the appeal. " +
			"The team knows the tooling. That's the real advantage here.",
	}, { // Test 1: The spelled-out copula is the same rhythm.
		Name: "spelled-out drumbeat", WantHit: true,
		In: "The migration finished early. That is the headline. Nobody worked the weekend. " +
			"That is the real win. It is rare. That is why it matters.",
	}, { // Test 2: One landing is a writer landing a line.
		Name: "single landing", WantHit: false,
		In: "We shipped on Friday after a long week of small fixes. That's the point.",
	}, { // Test 3: Three is still under the run, which is the writer's room to move.
		Name: "three landings", WantHit: false,
		In: "That's the thing about him. He never says sorry. That's fine. I stopped needing it years ago. It's easier that way.",
	}, { // Test 4: A long sentence does not snap shut, so it is not a landing.
		Name: "long sentences", WantHit: false,
		In: "That is the part of the argument nobody wanted to make out loud. " +
			"That is the reason the meeting ran an hour past its slot. " +
			"That is how the whole quarter ended up rearranged around one estimate. " +
			"That is what everyone remembered afterward.",
	}, { // Test 5: Code is off limits to every rule.
		Name: "fenced code", WantHit: false,
		In: "```\nThat's the point. It's cheap. That's the appeal. That's it.\n```",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			hit := false
			for _, f := range s.Check(test.In) {
				if f.Rule != "structural:landing-drumbeat" {
					continue
				}
				if f.Replacement != nil {
					t.Errorf("landing-drumbeat carries a replacement, want flag-only")
				}
				hit = true
			}
			if hit != test.WantHit {
				t.Errorf("landing-drumbeat hit = %v, want %v", hit, test.WantHit)
			}
		})
	}
}

// TestLandingDrumbeatNeedsRate checks the floor that the count alone cannot carry. Four
// landings scattered through a long piece is a writer with a tic; four in a page is the
// rhythm. Without the rate floor, length alone would eventually manufacture the finding
// on any long enough document.
func TestLandingDrumbeatNeedsRate(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Four landings, well spaced. The filler is plain prose carrying no tell of its own.
	filler := strings.Repeat("The survey crew reached the ridge before noon and set a benchmark on the granite outcrop that had shed its snow. ", 6)
	long := filler + "That's the point. " + filler + "It's cheap. " + filler + "That's the appeal. " + filler + "That's it."

	spans, rate := drumbeatRate(long)
	if len(spans) < drumbeatMinCount {
		t.Fatalf("want at least %d landings to test the rate floor, got %d", drumbeatMinCount, len(spans))
	}
	if rate >= drumbeatMinPer100 {
		t.Fatalf("passage is too dense to test the floor: %.2f per 100", rate)
	}
	for _, f := range s.Check(long) {
		if f.Rule == "structural:landing-drumbeat" {
			t.Errorf("landing-drumbeat fired at %.2f landings per 100 words, below the floor of %.1f", rate, drumbeatMinPer100)
		}
	}
	if got := s.Score(long).Drumbeat; got != 0 {
		t.Errorf("drumbeat score = %d, want 0 below the rate floor", got)
	}
}

// TestDrumbeatScored checks that the habit moves the score, since a document-level shape
// is invisible to tell density: one finding spread over hundreds of words rounds away.
func TestDrumbeatScored(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const beat = "The proposal is sound. That's the point. It's also cheap. That's the appeal. " +
		"The team knows the tooling. That's the real advantage here."
	const plain = "The proposal covers the migration, the rollback, and the on-call rota. " +
		"We reviewed it on Tuesday and sent it back with two questions about the cutover window."

	if got := s.Score(beat).Drumbeat; got <= 0 {
		t.Errorf("drumbeat score = %d, want a positive reading on a drumbeat passage", got)
	}
	if got := s.Score(plain).Drumbeat; got != 0 {
		t.Errorf("drumbeat score = %d, want 0 on plain prose", got)
	}
}
