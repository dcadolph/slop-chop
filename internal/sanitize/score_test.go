package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestScore checks that dense slop scores higher than clean, varied prose and that a flat
// cadence lifts the score even without word tells.
func TestScore(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		Name    string
		In      string
		WantMin int
		WantMax int
	}{
		{
			Name:    "clean varied prose",
			In:      "The dog barked. Rain fell for hours across the valley, cold and steady. She left.",
			WantMin: 0, WantMax: 25,
		},
		{
			Name:    "dense buzzwords",
			In:      "We leverage cutting-edge synergy to revolutionize a robust, seamless paradigm shift.",
			WantMin: 50, WantMax: 100,
		},
		{
			Name:    "structural tell",
			In:      "It's not just fast, it's revolutionary. Let's dive into the details right now.",
			WantMin: 20, WantMax: 100,
		},
	}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := s.Score(test.In)
			if got.Value < test.WantMin || got.Value > test.WantMax {
				t.Errorf("score = %d, want in [%d,%d] (%+v)", got.Value, test.WantMin, test.WantMax, got)
			}
		})
	}
}

// TestScoreEmpty checks that empty text scores zero and does not divide by zero.
func TestScoreEmpty(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Score(""); got.Value != 0 {
		t.Errorf("empty score = %d, want 0 (%+v)", got.Value, got)
	}
}

// TestScoreFlatCadence checks that a perfectly uniform sentence rhythm, the flattest and
// most machine-like cadence, earns the cadence penalty instead of escaping it as a zero.
func TestScoreFlatCadence(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := s.Score("The cat sat on the mat. The dog ran in the park. The bird flew to the tree.")
	if got.CadenceCV != 0 {
		t.Errorf("cadenceCV = %v, want 0 for equal-length sentences (%+v)", got.CadenceCV, got)
	}
	if got.Value < 15 {
		t.Errorf("flat cadence score = %d, want the cadence penalty applied (%+v)", got.Value, got)
	}
}

// TestScoreExcludesCode checks that words inside a code block do not dilute the tell density,
// so wrapping slop in a large fenced block cannot tank the score.
func TestScoreExcludesCode(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	prose := "This is a robust, seamless, comprehensive tool for you."
	bare := s.Score(prose)
	padded := s.Score(prose + "\n\n```\n" + strings.Repeat("token ", 200) + "\n```\n")
	if padded.Value < bare.Value-5 {
		t.Errorf("code padding diluted the score: bare %d, padded %d", bare.Value, padded.Value)
	}
}
