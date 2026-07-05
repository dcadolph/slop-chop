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
	}, { // Test 7: Honesty filler phrase is removed.
		In: "Giving it to you honestly, it ships", WantResult: "it ships",
	}, { // Test 8: Multi-word block word stays in place.
		In: "the blast radius is small", WantResult: "the blast radius is small",
	}, { // Test 9: A semicolon list is left alone, not split into sentences.
		In: "We support Go; Python; and Rust.", WantResult: "We support Go; Python; and Rust.",
	}, { // Test 10: A semicolon before a conjunction is left alone.
		In: "ship it; and forget it", WantResult: "ship it; and forget it",
	}, { // Test 11: An em-dash with spaces around it leaves no space before the comma.
		In: "word — word", WantResult: "word, word",
	}, { // Test 12: A semicolon at the end of a line does not swallow the newline.
		In: "it works;\nit ships", WantResult: "it works;\nit ships",
	}, { // Test 13: A semicolon before a CRLF line break is left alone.
		In: "it works;\r\nit ships", WantResult: "it works;\r\nit ships",
	}, { // Test 14: A space before a semicolon is dropped after the split.
		In: "a ; b", WantResult: "a. B",
	}, { // Test 15: Indentation before a leading dot is not punctuation debris.
		In: "code:\n    .hidden stays", WantResult: "code:\n    .hidden stays",
	}, { // Test 16: A space before a period is removed mid-line.
		In: "done .", WantResult: "done.",
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
	}, { // Test 4: Multi-word buzzword is flagged.
		In: "the blast radius", WantRules: []string{"word:blast radius"},
	}, { // Test 5: Word boundaries hold. robust in robustness and delve in delved stay clear.
		In: "robustness improved and delved deeper", WantRules: nil,
	}, { // Test 6: A semicolon separating list items is not flagged as a clause join.
		In: "Go; Python; and Rust", WantRules: nil,
	}, { // Test 7: A space before punctuation is flagged.
		In: "word , word", WantRules: []string{"space-before-punct"},
	}, { // Test 8: A semicolon at a line end is not flagged as a clause join.
		In: "it works;\nit ships", WantRules: nil,
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

// TestRuneColumn checks that a column is a rune count, not a byte offset, when a
// multibyte character sits before the match.
func TestRuneColumn(t *testing.T) {
	t.Parallel()
	s := mustSanitizer(t)
	var got Finding
	found := false
	for _, f := range s.Check("a — b robust") {
		if f.Rule == "word:robust" {
			got, found = f, true
		}
	}
	if !found {
		t.Fatal("robust not flagged")
	}
	// The em-dash is three bytes but one rune, so robust starts at rune column 7.
	if got.Line != 1 || got.Col != 7 {
		t.Errorf("line,col = %d,%d, want 1,7", got.Line, got.Col)
	}
}

// TestLoadMalformed checks that invalid JSON returns an error.
func TestLoadMalformed(t *testing.T) {
	t.Parallel()
	if _, err := Load(strings.NewReader("{not valid json")); err == nil {
		t.Error("Load: want error for malformed JSON")
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
