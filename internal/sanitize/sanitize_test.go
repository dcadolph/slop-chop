package sanitize

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestFix checks that the default profile rewrites text as expected.
func TestFix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		WantResult string
	}{{ // Test 0: Em-dash becomes a comma.
		In: "fast—clean", WantResult: "fast, clean",
	}, { // Test 1: Smart quotes become straight quotes.
		In: "“hi” it’s", WantResult: `"hi" it's`,
	}, { // Test 2: Leading padding phrase is removed.
		In: "In summary, it works", WantResult: "it works",
	}, { // Test 3: Semicolon splits into a sentence with capitalized next word.
		In: "it works; it ships", WantResult: "it works. It ships",
	}, { // Test 4: Runs of spaces collapse to one.
		In: "a    b", WantResult: "a b",
	}, { // Test 5: Block words are left in place.
		In: "a robust plan", WantResult: "a robust plan",
	}, { // Test 6: Clean text is unchanged.
		In: "a plain sentence", WantResult: "a plain sentence",
	}}

	s := mustSanitizer(t)
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCheck checks the rule names reported for a given input.
func TestCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In        string
		WantRules []string
	}{{ // Test 0: Em-dash is flagged.
		In: "fast—clean", WantRules: []string{"char:—"},
	}, { // Test 1: Block word is flagged.
		In: "a robust plan", WantRules: []string{"word:robust"},
	}, { // Test 2: Clean text yields nothing.
		In: "a plain sentence", WantRules: nil,
	}, { // Test 3: Multiple tells are all flagged.
		In: "robust; nice", WantRules: []string{"word:robust", "semicolon"},
	}}

	s := mustSanitizer(t)
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, f := range s.Check(test.In) {
				got = append(got, f.Rule)
			}
			less := func(a, b string) bool { return a < b }
			if diff := cmp.Diff(test.WantRules, got,
				cmpopts.EquateEmpty(), cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestLoadPartialProfile checks that a partial JSON profile decodes with zero values
// for the omitted fields.
func TestLoadPartialProfile(t *testing.T) {
	t.Parallel()
	p, err := Load(strings.NewReader(`{"collapseSpaces": true}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !p.CollapseSpaces {
		t.Error("CollapseSpaces: want true")
	}
	if p.SplitSemicolons {
		t.Error("SplitSemicolons: want false")
	}
}

// mustSanitizer builds a Sanitizer from the default profile or fails the test.
func mustSanitizer(t *testing.T) *Sanitizer {
	t.Helper()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}
