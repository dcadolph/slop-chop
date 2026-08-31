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

// TestScoreFlatCadence checks that a perfectly uniform sentence rhythm is still reported
// as a zero coefficient of variation, while carrying no score weight: plain, even
// sentences are common in competent human writing, so clean flat prose must score clean.
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
	if got.Value != 0 {
		t.Errorf("clean flat prose score = %d, want 0 (%+v)", got.Value, got)
	}
}

// TestScoreBreakdown checks that the score reports named contributions and that a hedge-heavy
// register adds to the hedging signal, while clean prose does not.
func TestScoreBreakdown(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := s.Score("This might possibly work and could arguably help, though results may generally vary.")
	if got.Hedging == 0 {
		t.Errorf("hedge-heavy text scored 0 hedging (%+v)", got)
	}
	if got.Value < got.Density {
		t.Errorf("value %d below its density contribution (%+v)", got.Value, got)
	}
	clean := s.Score("The cat sat. A dog ran far across the wide field. She left before dawn broke.")
	if clean.Hedging != 0 {
		t.Errorf("clean prose reported hedging %d (%+v)", clean.Hedging, clean)
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

// TestScoreWeights checks that a profile's scoreWeights overrides move the score: an
// exact rule name silences one tell, a class entry rescales a whole kind, and an exact
// name wins over its class.
func TestScoreWeights(t *testing.T) {
	t.Parallel()

	// Test 0: zeroing the em-dash by exact name drops its density to nothing.
	quiet, err := New(Profile{
		CharReplace:  map[string]string{"—": ", "},
		ScoreWeights: map[string]float64{"char:—": 0},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := quiet.Score("The plan—which shipped—works fine for us all today."); got.Density != 0 {
		t.Errorf("silenced em-dash density = %d, want 0 (%+v)", got.Density, got)
	}

	// Test 1: a class entry rescales every rule in the class.
	loud, err := New(Profile{
		BlockWords:   []string{"robust"},
		ScoreWeights: map[string]float64{"word": 3},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	base, err := New(Profile{BlockWords: []string{"robust"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	text := "A robust plan needs more than words to hold up in the field."
	if l, b := loud.Score(text).Density, base.Score(text).Density; l <= b {
		t.Errorf("word weight 3 density = %d, want above the default %d", l, b)
	}

	// Test 2: an exact name wins over its class.
	mixed, err := New(Profile{
		BlockWords:   []string{"robust", "delve"},
		ScoreWeights: map[string]float64{"word": 2, "word:delve": 0},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	withDelve := mixed.Score("They delve into the plan before lunch arrives for everyone.")
	if withDelve.Density != 0 {
		t.Errorf("word:delve weight 0 density = %d, want 0 (%+v)", withDelve.Density, withDelve)
	}
}

// TestScoreRepetitionDecay checks that repeats of one rule count with halving weight
// while the same number of distinct tells keeps full weight, so a repeated habit scores
// below diverse slop of equal density.
func TestScoreRepetitionDecay(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	filler := " The road ran west between low fences and the light held until supper."
	repeated := s.Score("They delve and delve and delve and delve into it." + strings.Repeat(filler, 3))
	diverse := s.Score("They delve into robust, seamless, comprehensive work." + strings.Repeat(filler, 3))
	if repeated.Tells != 4 || diverse.Tells != 4 {
		t.Fatalf("tells = %d and %d, want 4 and 4 (%+v / %+v)", repeated.Tells, diverse.Tells, repeated, diverse)
	}
	if repeated.Density >= diverse.Density {
		t.Errorf("repeated density %d not below diverse density %d", repeated.Density, diverse.Density)
	}
}
