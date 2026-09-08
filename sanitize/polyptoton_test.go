package sanitize

import (
	"fmt"
	"testing"
)

// TestPolyptoton checks that a stem turned against itself is flagged and that the
// ordinary technical sentence, where the parser parses and the scheduler schedules, is
// not. Stem repetition is everywhere in honest prose, so the turn is what has to carry
// the finding.
func TestPolyptoton(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name    string
		In      string
		WantHit bool
	}{{ // Test 0: The relative-clause turn.
		Name: "verification that does not verify", WantHit: true,
		In: "We shipped a verification step that does not verify anything.",
	}, { // Test 1: The same turn on a copular clause.
		Name: "governance that does not govern", WantHit: true,
		In: "What the vendor sold us was governance that does not govern.",
	}, { // Test 2: "without" is pivot and negation at once.
		Name: "measurement without measuring", WantHit: true,
		In: "It is a measurement without measuring anything real.",
	}, { // Test 3: A contraction negates the same way the spelled-out form does.
		Name: "contracted negation", WantHit: true,
		In: "They sold us an assurance that doesn't assure anyone.",
	}, { // Test 4: A negation with no pivot is a sentence describing a behavior.
		Name: "scheduler does not schedule", WantHit: false,
		In: "The scheduler does not schedule jobs during a backup window.",
	}, { // Test 5: Bare stem repetition is ordinary technical prose.
		Name: "parser parses", WantHit: false,
		In: "The parser parses the header first, then the body.",
	}, { // Test 6: So is the noun and its verb side by side.
		Name: "container contains", WantHit: false,
		In: "The container contains the whole toolchain.",
	}, { // Test 7: A shared prefix is not a shared root.
		Name: "constraint and construct", WantHit: false,
		In: "The constraint that does not construct anything is fine.",
	}, { // Test 8: Two forms too far apart are two thoughts.
		Name: "forms far apart", WantHit: false,
		In: "The verification ran overnight on the whole fleet and that is not how you verify.",
	}, { // Test 9: Code is off limits to every rule.
		Name: "fenced code", WantHit: false,
		In: "```\ngovernance that does not govern\n```",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			hit := false
			for _, f := range s.Check(test.In) {
				if f.Rule != "structural:polyptoton" {
					continue
				}
				if f.Replacement != nil {
					t.Errorf("polyptoton carries a replacement, want flag-only")
				}
				hit = true
			}
			if hit != test.WantHit {
				t.Errorf("polyptoton hit = %v, want %v for %q", hit, test.WantHit, test.In)
			}
		})
	}
}

// TestSameRoot checks the crude root test on its own, since it is the piece that decides
// whether two words are one stem or merely start alike.
func TestSameRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		A, B string
		Want bool
	}{
		{A: "verification", B: "verify", Want: true},   // Test 0: Suffix apart.
		{A: "governance", B: "govern", Want: true},     // Test 1: The bare stem.
		{A: "measurement", B: "measuring", Want: true}, // Test 2: Two suffixes.
		{A: "constraint", B: "construct", Want: false}, // Test 3: A shared opening only.
		{A: "release", B: "relearn", Want: false},      // Test 4: Three letters shared.
		{A: "assurance", B: "assure", Want: true},      // Test 5: Suffix apart again.
		{A: "reporting", B: "repository", Want: false}, // Test 6: Under the share floor.
		{A: "schedule", B: "scheduler", Want: true},    // Test 7: Same root, real pair.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := sameRoot(test.A, test.B); got != test.Want {
				t.Errorf("sameRoot(%q, %q) = %v, want %v", test.A, test.B, got, test.Want)
			}
		})
	}
}
