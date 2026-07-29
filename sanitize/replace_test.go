package sanitize

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestWordReplace checks the case-preserving, whole-word single-word swap.
func TestWordReplace(t *testing.T) {
	t.Parallel()
	words := map[string]string{"utilize": "use", "leverage": "use"}
	tests := []struct {
		In         string
		WantResult string
	}{{ // Test 0: A lower-case word is swapped.
		In: "we utilize it", WantResult: "we use it",
	}, { // Test 1: A leading capital carries over.
		In: "Utilize it", WantResult: "Use it",
	}, { // Test 2: All caps carries over.
		In: "UTILIZE it", WantResult: "USE it",
	}, { // Test 3: A longer word that contains the key is left alone.
		In: "disutilize stays", WantResult: "disutilize stays",
	}, { // Test 4: A word whose tail is the key is left alone.
		In: "the utilization stays", WantResult: "the utilization stays",
	}, { // Test 5: Two different keys both swap.
		In: "utilize and leverage", WantResult: "use and use",
	}, { // Test 6: A code span is protected.
		In: "utilize `utilize` end", WantResult: "use `utilize` end",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{WordReplace: words})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRegexReplace checks user regular-expression rules, including group expansion, the
// zero-width guard, and code protection.
func TestRegexReplace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Patterns   map[string]string
		In         string
		WantResult string
	}{{ // Test 0: A capture group expands into the replacement.
		Patterns: map[string]string{"([0-9]+) ?%": "$1 percent"}, In: "up 50% today",
		WantResult: "up 50 percent today",
	}, { // Test 1: A user boundary keeps the match whole.
		Patterns: map[string]string{`\bTODO\b`: "done"}, In: "TODO not TODOS",
		WantResult: "done not TODOS",
	}, { // Test 2: A pattern that can match nothing does not insert between characters.
		Patterns: map[string]string{"x*": "Z"}, In: "abc", WantResult: "abc",
	}, { // Test 3: A match inside a code span is protected.
		Patterns: map[string]string{"[0-9]+%": "pct"}, In: "50% `50%` end",
		WantResult: "pct `50%` end",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{RegexReplace: test.Patterns})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRegexReplaceCompileError checks that a malformed pattern is an error from New.
func TestRegexReplaceCompileError(t *testing.T) {
	t.Parallel()
	_, err := New(Profile{RegexReplace: map[string]string{"(unclosed": "x"}})
	if !errors.Is(err, ErrCompile) {
		t.Errorf("err = %v, want ErrCompile", err)
	}
}

// TestAllow checks that an allow list exempts a match from both flagging and rewriting,
// case-insensitively.
func TestAllow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Profile   Profile
		In        string
		WantRules []string
	}{{ // Test 0: An allowed block word is not flagged, an un-allowed one is.
		Profile:   Profile{BlockWords: []string{"robust", "comprehensive"}, Allow: []string{"comprehensive"}},
		In:        "a robust comprehensive plan",
		WantRules: []string{"word:robust"},
	}, { // Test 1: Allow is case-insensitive.
		Profile:   Profile{BlockWords: []string{"robust"}, Allow: []string{"ROBUST"}},
		In:        "a Robust plan",
		WantRules: nil,
	}, { // Test 2: Allow also exempts a rewrite, so an allowed spelling is left in place.
		Profile:   Profile{Dialect: DialectAmerican, Allow: []string{"colour"}},
		In:        "the colour and the behaviour",
		WantRules: []string{"spelling"},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(test.Profile)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			var got []string
			for _, f := range s.Check(test.In) {
				got = append(got, f.Rule)
			}
			if diff := cmp.Diff(test.WantRules, got); diff != "" {
				t.Errorf("rules mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
