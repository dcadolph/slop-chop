package sanitize

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// tailHeavy closes four of its six sentences on a participle tail.
const tailHeavy = "The Allies flew fourteen thousand sorties, completely neutralizing the air force. " +
	"A large fleet sat offshore, deploying a great deal of firepower. " +
	"The landings began at dawn. " +
	"Signals intelligence read the orders, mapping out enemy reserves in advance. " +
	"The defenders were spread thin. " +
	"Bombers struck the rail lines, severely impairing the movement of reserves."

// TestTailCount checks which sentence endings count as a participle tail.
func TestTailCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name          string
		In            string
		WantTails     int
		WantSentences int
	}{{ // Test 0: A comma and an -ing clause closing the sentence counts.
		Name: "tail", In: "The fleet sat offshore, deploying its guns at dawn.",
		WantTails: 1, WantSentences: 1,
	}, { // Test 1: An adverb before the participle still counts.
		Name: "adverb", In: "The planes flew all day, completely neutralizing the defense.",
		WantTails: 1, WantSentences: 1,
	}, { // Test 2: A preposition ending in -ing opens an ordinary phrase, not a tail.
		Name: "preposition", In: "We packed everything, including the tent and the stove.",
		WantTails: 0, WantSentences: 1,
	}, { // Test 3: A noun ending in -ing is not a participle.
		Name: "noun", In: "She looked at the sky, something odd about the light.",
		WantTails: 0, WantSentences: 1,
	}, { // Test 4: An -ing word with no clause after it is a list item, not a tail.
		Name: "bare", In: "We tried hiking, swimming, and running.",
		WantTails: 0, WantSentences: 1,
	}, { // Test 5: Two tails in one sentence count once.
		Name:      "once per sentence",
		In:        "The guns fired, shaking the cliffs, shattering the bunkers along the coast.",
		WantTails: 1, WantSentences: 1,
	}, { // Test 6: No comma, no tail.
		Name: "no comma", In: "Running the numbers took most of the afternoon.",
		WantTails: 0, WantSentences: 1,
	}, { // Test 7: The heavy passage.
		Name: "heavy", In: tailHeavy, WantTails: 4, WantSentences: 6,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			tails, sentences := tailCount(test.In)
			if diff := cmp.Diff(test.WantTails, tails); diff != "" {
				t.Errorf("tails mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantSentences, sentences); diff != "" {
				t.Errorf("sentences mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestTailPoints checks the floors and the cap: one tail is free, a short text cannot read
// as a habit, code is not prose, and a saturated passage stops at the cap.
func TestTailPoints(t *testing.T) {
	t.Parallel()
	plain := "The landings began at dawn. The defenders were spread thin. " +
		"The weather was poor. The tide was low. The beaches were wide."
	tests := []struct {
		Name       string
		In         string
		WantPoints float64
	}{{ // Test 0: Two tails in seven sentences: one past the free one, over seven sentences.
		Name: "rate",
		In: plain + " The fleet sat offshore, deploying its guns at dawn." +
			" The planes flew all day, neutralizing the defense.",
		WantPoints: 1.0 / 7 * tailPointsPerRate,
	}, { // Test 1: One tail is a writer's ordinary move and adds nothing.
		Name: "one tail", In: plain + " The fleet sat offshore, deploying its guns at dawn.",
		WantPoints: 0,
	}, { // Test 2: Plain prose adds nothing.
		Name: "plain", In: plain, WantPoints: 0,
	}, { // Test 3: Under the sentence floor nothing counts, however dense.
		Name:       "short",
		In:         "The fleet sat offshore, deploying its guns. The planes flew, neutralizing the defense.",
		WantPoints: 0,
	}, { // Test 4: Tails inside a fenced block are code, not prose.
		Name:       "fenced",
		In:         plain + "\n\n```\nx, deploying the stack to prod\ny, rolling back the release now\n```\n",
		WantPoints: 0,
	}, { // Test 5: Every sentence a tail stops at the cap.
		Name: "cap",
		In: "A, deploying the guns at dawn. B, mapping the reserves in advance. " +
			"C, shaking the cliffs all morning. D, neutralizing the defense entirely. " +
			"E, impairing the movement of reserves.",
		WantPoints: tailMaxPoints,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := tailPoints(test.In); math.Abs(got-test.WantPoints) > 1e-9 {
				t.Errorf("tailPoints = %v, want %v", got, test.WantPoints)
			}
		})
	}
}

// TestTailsScored checks the habit reaches the score and its own component, and that the
// same passage with its tails rewritten as plain sentences loses exactly that component.
func TestTailsScored(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rewritten := "The Allies flew fourteen thousand sorties. The air force could not respond. " +
		"A large fleet sat offshore with a great deal of firepower. " +
		"The landings began at dawn. " +
		"Signals intelligence read the orders and mapped out enemy reserves in advance. " +
		"The defenders were spread thin. " +
		"Bombers struck the rail lines and severely impaired the movement of reserves."
	heavy, plain := s.Score(tailHeavy), s.Score(rewritten)
	if heavy.Tails != tailMaxPoints {
		t.Errorf("heavy Tails = %d, want the cap of %d", heavy.Tails, tailMaxPoints)
	}
	if plain.Tails != 0 {
		t.Errorf("rewritten Tails = %d, want 0", plain.Tails)
	}
	if heavy.Value <= plain.Value {
		t.Errorf("heavy Value %d is not above rewritten Value %d", heavy.Value, plain.Value)
	}
}
