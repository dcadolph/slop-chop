package sanitize

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// clauseHeavy carries a comma in every one of its seven sentences and no lexical tell.
const clauseHeavy = "The tool watches the directory, uploading each new file as it lands. " +
	"Uploads run in the background, so the terminal stays free for other work. " +
	"When an upload fails, the error is written to a log in the home directory. " +
	"The log names the file, the time, and the reason the upload did not go through. " +
	"On the next pass, the tool tries the file again. " +
	"If it keeps failing, the log is the place to look. " +
	"Removing the tool leaves the configuration in place, along with the logs."

// clausePlain says the same things in sentences with no comma in any of them.
const clausePlain = "The tool watches the directory and uploads each new file as it lands. " +
	"Uploads run in the background so the terminal stays free for other work. " +
	"A failed upload is written to a log in the home directory. " +
	"The log names the file and the time and the reason it did not go through. " +
	"The tool tries the file again on the next pass. " +
	"The log is the place to look if it keeps failing. " +
	"Removing the tool leaves the configuration and the logs in place."

// TestClauseShare checks which sentences count and which commas count.
func TestClauseShare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name          string
		In            string
		WantShare     float64
		WantSentences int
	}{{ // Test 0: Every sentence carries a comma.
		Name: "all", In: clauseHeavy, WantShare: 1, WantSentences: 7,
	}, { // Test 1: No sentence carries a comma.
		Name: "none", In: clausePlain, WantShare: 0, WantSentences: 7,
	}, { // Test 2: A thousands separator is not a clause.
		Name: "number", In: "The fleet carried 14,000 tons across the strait.",
		WantShare: 0, WantSentences: 1,
	}, { // Test 3: A heading is not prose. A list item that is a sentence is.
		Name:      "markup",
		In:        "# Install, then run\n\n- brew install it, then run it.\n\nThe tool watches the directory and uploads.\n",
		WantShare: 0.5, WantSentences: 2,
	}, { // Test 4: A numbered item splits at its own marker, and its body counts as prose.
		Name:      "numbered",
		In:        "1. Install it, then run it.\n\n2019 was the year it shipped, late.\n",
		WantShare: 1, WantSentences: 2,
	}, { // Test 5: A table row is not prose.
		Name:      "table",
		In:        "| a, b | c |\n|---|---|\n| d, e | f |\n\nThe table above lists the pairs, in order.\n",
		WantShare: 1, WantSentences: 1,
	}, { // Test 6: Short fragments do not count either way.
		Name: "short", In: "Yes, really. The tool watches the directory, then uploads.",
		WantShare: 1, WantSentences: 1,
	}, { // Test 7: Code is not prose.
		Name:      "fenced",
		In:        "The tool watches the directory and uploads.\n\n```\nx, y = 1, 2\na, b = 3, 4\n```\n",
		WantShare: 0, WantSentences: 1,
	}, { // Test 8: Nothing to read.
		Name: "empty", In: "", WantShare: 0, WantSentences: 0,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			share, sentences := clauseShare(test.In)
			if diff := cmp.Diff(test.WantSentences, sentences); diff != "" {
				t.Errorf("sentences mismatch (-want +got):\n%s", diff)
			}
			if math.Abs(share-test.WantShare) > 1e-9 {
				t.Errorf("share = %v, want %v", share, test.WantShare)
			}
		})
	}
}

// TestClausePoints checks the floor, the threshold, the gradient, and the cap.
func TestClausePoints(t *testing.T) {
	t.Parallel()
	// sixOf builds six prose sentences, the first n of them carrying a comma.
	sixOf := func(n int) string {
		out := ""
		for i := range 6 {
			if i < n {
				out += "The tool watches the directory, then uploads the file. "
			} else {
				out += "The tool watches the directory and uploads the file. "
			}
		}
		return out
	}
	tests := []struct {
		Name       string
		In         string
		WantPoints float64
	}{{ // Test 0: Under the sentence floor nothing counts, however dense.
		Name: "floor", In: "A, b c d e. F, g h i j. K, l m n o. P, q r s t. U, v w x y.", WantPoints: 0,
	}, { // Test 1: At the threshold nothing is charged.
		Name: "threshold", In: sixOf(4), WantPoints: 0,
	}, { // Test 2: Five of six is over the threshold and on the gradient.
		Name: "gradient", In: sixOf(5), WantPoints: clauseMaxPoints * (5.0/6 - clauseThreshold) / (1 - clauseThreshold),
	}, { // Test 3: A comma in every sentence reaches the cap.
		Name: "cap", In: sixOf(6), WantPoints: clauseMaxPoints,
	}, { // Test 4: Plain prose adds nothing.
		Name: "plain", In: clausePlain, WantPoints: 0,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := clausePoints(test.In); math.Abs(got-test.WantPoints) > 1e-9 {
				t.Errorf("clausePoints = %v, want %v", got, test.WantPoints)
			}
		})
	}
}

// TestClausesScored checks the habit reaches the score through its own component, and that
// the same passage written in plain sentences loses exactly that component.
func TestClausesScored(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	heavy, plain := s.Score(clauseHeavy), s.Score(clausePlain)
	if heavy.Clauses != clauseMaxPoints {
		t.Errorf("heavy Clauses = %d, want the cap of %d", heavy.Clauses, clauseMaxPoints)
	}
	if plain.Clauses != 0 {
		t.Errorf("plain Clauses = %d, want 0", plain.Clauses)
	}
	if heavy.Value <= plain.Value {
		t.Errorf("heavy Value %d is not above plain Value %d", heavy.Value, plain.Value)
	}
	// The habit alone cannot carry a verdict: the cap is under the reads-clean line.
	if heavy.Value >= 25 {
		t.Errorf("heavy Value %d reached the line on the clause habit alone", heavy.Value)
	}
}
