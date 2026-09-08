package sanitize

import (
	"fmt"
	"testing"
)

// TestPunchiness checks the reported drumbeat signal: the share of sentences sitting in a
// run of short ones. It carries no score weight, so what matters here is that the reading
// itself is right, including the sentinel for a passage too short to judge.
func TestPunchiness(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want float64
	}{{ // Test 0: Four landings in a row, the flat machine drumbeat.
		Name: "all short", Want: 1,
		In: "The system is efficient. The design is modern. The output is clean. The result is strong.",
	}, { // Test 1: Two short sentences are not yet a run.
		Name: "run of two", Want: 0,
		In: "It shipped. It held. The next release pulled in the whole vendor tree and slowed the build down badly.",
	}, { // Test 2: A run of three inside longer prose counts only itself.
		Name: "run inside prose", Want: 0.6,
		In: "The survey crew reached the ridge before noon and set the first benchmark on an outcrop of granite. " +
			"It held. The weather turned. Nobody minded. " +
			"They walked the line again the following morning with the theodolite and a fresh set of stakes.",
	}, { // Test 3: Too few sentences to read a rhythm.
		Name: "too few sentences", Want: -1,
		In: "It shipped. It held.",
	}, { // Test 4: One long sentence has no rhythm to read either.
		Name: "single sentence", Want: -1,
		In: "The mail arrived late on Thursday and nobody in the office minded very much at all.",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := cadenceReport(punchiness(test.In)); got != test.Want {
				t.Errorf("punchiness = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestPunchinessUnweighted checks that the drumbeat signal is reported and nothing more.
// A page of short human sentences reads punchy and must still score clean, which is the
// reason the signal carries no weight: terse is a voice, not a tell.
func TestPunchinessUnweighted(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const terse = "He fixed the leak in twenty minutes. Turned out a washer had cracked. Cheap part, easy swap, no drama."
	got := s.Score(terse)
	if got.Punchiness <= 0 {
		t.Errorf("punchiness = %v, want a positive reading on terse prose", got.Punchiness)
	}
	if got.Value != 0 {
		t.Errorf("score = %d, want 0: a punchy human passage carries no penalty", got.Value)
	}
}
