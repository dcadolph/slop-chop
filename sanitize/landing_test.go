package sanitize

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestCopulaLanding checks that the copula-abstraction landing is flagged and that the
// honest sentences sharing its surface shape are not. The negatives carry the weight
// here: this shape appears in good writing, so a landing missed costs less than an
// ordinary sentence flagged.
func TestCopulaLanding(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name    string
		In      string
		WantHit bool
	}{{ // Test 0: The sentence that opened the investigation.
		Name: "flagged services-page sentence", WantHit: true,
		In: "It is usually about a third, and that number is the rest of the day.",
	}, { // Test 1: Demonstrative subject, intensified abstraction.
		Name: "demonstrative subject", WantHit: true,
		In: "We ran the numbers again. That gap is the whole point.",
	}, { // Test 2: Definite subject, bare abstraction.
		Name: "definite subject", WantHit: true,
		In: "Two vendors quoted the same scope. The spread is the point.",
	}, { // Test 3: Bare noun subject.
		Name: "bare noun subject", WantHit: true,
		In: "Every team wants more users. Reach is the constraint.",
	}, { // Test 4: An abstraction complement with an "of" tail.
		Name: "of tail", WantHit: true,
		In: "Trust is the currency of the whole arrangement.",
	}, { // Test 5: A concrete complement is a sentence saying something.
		Name: "concrete complement", WantHit: false,
		In: "That column is the primary key for the table.",
	}, { // Test 6: An abstraction on both sides is a definition, not a landing.
		Name: "abstraction subject", WantHit: false,
		In: "The problem is the cost of the extra index.",
	}, { // Test 7: A bare pronoun subject is ordinary reference.
		Name: "pronoun subject", WantHit: false,
		In: "It is the point of the exercise.",
	}, { // Test 8: The "That's the story" idiom is older than any model.
		Name: "bare demonstrative subject", WantHit: false,
		In: "We closed the ticket. That is the story.",
	}, { // Test 9: An indefinite complement describes rather than declares.
		Name: "indefinite complement", WantHit: false,
		In: "The gap is a constraint on the schedule.",
	}, { // Test 10: A negated complement belongs to the negation family.
		Name: "negated complement", WantHit: false,
		In: "That number is not the point at all.",
	}, { // Test 11: A hedge in front of the complement is not a landing.
		Name: "hedged complement", WantHit: false,
		In: "That number is probably the point.",
	}, { // Test 12: A tail on any preposition but "of" means the sentence went on.
		Name: "non-of tail", WantHit: false,
		In: "Memory is the constraint on this box.",
	}, { // Test 13: A long subject is a sentence doing work.
		Name: "long subject", WantHit: false,
		In: "The loud one at the back is the foreman.",
	}, { // Test 14: A relative clause after the noun is exposition.
		Name: "relative clause complement", WantHit: false,
		In: "The bug is the reason the test failed.",
	}, { // Test 15: A table row is alignment, not prose.
		Name: "table row", WantHit: false,
		In: "| metric | reading |\n| --- | --- |\n| The spread is the point | 4 |",
	}, { // Test 16: Code is off limits to every rule.
		Name: "fenced code", WantHit: false,
		In: "```\nThe spread is the point.\n```",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			hit := false
			for _, f := range s.Check(test.In) {
				if f.Rule != "structural:copula-landing" {
					continue
				}
				if f.Replacement != nil {
					t.Errorf("copula-landing carries a replacement, want flag-only")
				}
				hit = true
			}
			if hit != test.WantHit {
				t.Errorf("copula-landing hit = %v, want %v for %q", hit, test.WantHit, test.In)
			}
		})
	}
}

// TestCopulaLandingNoDoubleCount checks the two ways copula-landing can meet
// reversal-aphorism. Their shapes cannot collide inside one sentence, since a reversal
// closes on a bare auxiliary and a landing needs a complement after the copula, so a
// passage carrying both reports two findings on two spans. When a reversal reaches across
// a sentence break and swallows a landing whole, the score's overlap dedup counts the
// nested pair once.
func TestCopulaLandingNoDoubleCount(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name       string
		In         string
		WantRules  []string
		WantNested bool
		WantWeight float64
	}{{ // Test 0: The corpus passage. Two tells, two spans, full weight for each.
		Name:       "adjacent spans",
		In:         "Tools don't fail teams. Blind spots do. That number is the whole story.",
		WantRules:  []string{"structural:reversal-aphorism", "structural:copula-landing"},
		WantNested: false,
		WantWeight: 4,
	}, { // Test 1: The reversal span swallows the landing, so the pair counts once.
		Name:       "nested spans",
		In:         "It never mattered, and that gap is the point. The spread is.",
		WantRules:  []string{"structural:reversal-aphorism", "structural:copula-landing"},
		WantNested: true,
		WantWeight: 2,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			var got []string
			var structural []Finding
			for _, f := range s.Check(test.In) {
				if !strings.HasPrefix(f.Rule, "structural:") {
					continue
				}
				got = append(got, f.Rule)
				structural = append(structural, f)
			}
			if diff := cmp.Diff(test.WantRules, got, cmpopts.EquateEmpty(), cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("structural rules mismatch (-want +got):\n%s", diff)
			}
			if len(structural) != 2 {
				t.Fatalf("want two structural findings, got %d", len(structural))
			}
			first, second := structural[0], structural[1]
			nested := second.Offset < first.Offset+len(first.Match)
			if nested != test.WantNested {
				t.Errorf("spans nested = %v, want %v", nested, test.WantNested)
			}
			if got := s.weightTells(structural); got != test.WantWeight {
				t.Errorf("weightTells = %v, want %v", got, test.WantWeight)
			}
		})
	}
}
