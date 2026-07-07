package sanitize

import (
	"errors"
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
	}, { // Test 2: Leading padding phrase is removed and the capital is restored.
		In: "In summary, it works", WantResult: "It works",
	}, { // Test 3: Semicolon splits into a sentence with capitalized next word.
		In: "it works; it ships", WantResult: "it works. It ships",
	}, { // Test 4: Runs of spaces collapse to one.
		In: "a    b", WantResult: "a b",
	}, { // Test 5: Block words are left in place.
		In: "a robust plan", WantResult: "a robust plan",
	}, { // Test 6: Clean text is unchanged.
		In: "a plain sentence", WantResult: "a plain sentence",
	}, { // Test 7: Honesty filler phrase is removed and the capital is restored.
		In: "Giving it to you honestly, it ships", WantResult: "It ships",
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
	}, { // Test 17: A phrase deleted mid-sentence leaves the next word lowercase.
		In: "and to be honest, it works", WantResult: "and it works",
	}, { // Test 18: A phrase deleted after a period starts the new sentence with a capital.
		In: "It builds. In summary, it ships.", WantResult: "It builds. It ships.",
	}, { // Test 19: A phrase deleted after a line break starts the line with a capital.
		In: "line one\nin summary, it ships", WantResult: "line one\nIt ships",
	}, { // Test 20: A phrase followed by a digit is deleted with nothing to capitalize.
		In: "To recap, 42 wins", WantResult: "42 wins",
	}, { // Test 21: A fenced code block comes through untouched.
		In:         "```\na — b; c  d\n```\n",
		WantResult: "```\na — b; c  d\n```\n",
	}, { // Test 22: An inline code span comes through untouched.
		In: "run `a; b` now", WantResult: "run `a; b` now",
	}, { // Test 23: Prose around code is still cleaned.
		In: "fast—clean `x—y` fast—clean", WantResult: "fast, clean `x—y` fast, clean",
	}, { // Test 24: A phrase split by a line wrap is still deleted, capital restored.
		In: "It's worth\nnoting that it works", WantResult: "It works",
	}, { // Test 25: A phrase whose trailing space is a line break still matches.
		In: "In summary,\nit ships", WantResult: "It ships",
	}, { // Test 26: A stock opener from the expanded defaults is deleted with the
		// capital restored.
		In: "Needless to say, it passes", WantResult: "It passes",
	}, { // Test 27: Alignment padding on a markdown table row stays.
		In:         "| Kind    | Action    |\n| chop    | rewrite   |",
		WantResult: "| Kind    | Action    |\n| chop    | rewrite   |",
	}, { // Test 28: An indented table row keeps its padding too.
		In: "  | a  b |", WantResult: "  | a  b |",
	}, { // Test 29: Two trailing spaces are a markdown hard break and stay.
		In: "line one  \nline two", WantResult: "line one  \nline two",
	}, { // Test 30: A space run at the very end of the text stays.
		In: "line one  ", WantResult: "line one  ",
	}, { // Test 31: A phrase ending a line does not eat the newline into a code fence.
		In: "note in summary,\n```\ncode  x\n```", WantResult: "note \n```\ncode  x\n```",
	}, { // Test 32: A phrase does not match inside another word.
		In: "Let me begin summary, then details.", WantResult: "Let me begin summary, then details.",
	}, { // Test 33: A semicolon after a period does not make a double period.
		In: "See 2.; the rest wins.", WantResult: "See 2. The rest wins.",
	}, { // Test 34: A phrase before a blank-set indented block leaves the block alone.
		In: "text in summary,\n\n    robust code\n\nmore", WantResult: "text \n\n    robust code\n\nmore",
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
	}, { // Test 5: Word boundaries hold. robust inside robustness stays clear.
		In: "robustness improved", WantRules: nil,
	}, { // Test 6: A semicolon separating list items is not flagged as a clause join.
		In: "Go; Python; and Rust", WantRules: nil,
	}, { // Test 7: A space before punctuation is flagged.
		In: "word , word", WantRules: []string{"space-before-punct"},
	}, { // Test 8: A semicolon at a line end is not flagged as a clause join.
		In: "it works;\nit ships", WantRules: nil,
	}, { // Test 9: A block word inside an inline code span is not flagged.
		In: "the `robust` flag", WantRules: nil,
	}, { // Test 10: Nothing inside a fenced block is flagged.
		In: "```\nrobust — plan; done\n```", WantRules: nil,
	}, { // Test 11: A lone backtick does not hide the rest of the line.
		In: "a ` robust plan", WantRules: []string{"word:robust"},
	}, { // Test 12: A multi-word term split by a line wrap is still flagged.
		In: "a small blast\nradius here", WantRules: []string{"word:blast radius"},
	}, { // Test 13: A multi-word term with extra spaces between words is still flagged,
		// along with the double space itself.
		In: "the blast  radius", WantRules: []string{"word:blast radius", "double-space"},
	}, { // Test 14: Inflected forms of a block word are flagged.
		In: "it delves into the details", WantRules: []string{"word:delves"},
	}, { // Test 15: The past tense form is flagged too.
		In: "we delved deeper", WantRules: []string{"word:delved"},
	}, { // Test 16: A stock buzzword phrase is flagged.
		In: "a testament to quality", WantRules: []string{"word:testament to"},
	}, { // Test 17: Table padding is not a double space.
		In: "| flags  only |", WantRules: nil,
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

// TestCheckOrder checks that findings come back in text order, not rule order, so a
// match on line 1 never prints below a match on line 2.
func TestCheckOrder(t *testing.T) {
	t.Parallel()
	s := mustSanitizer(t)
	var got []string
	for _, f := range s.Check("robust\nx — y") {
		got = append(got, f.Rule)
	}
	want := []string{"word:robust", "char:—"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestNewCompileError checks that a profile entry that cannot compile, like one holding
// invalid UTF-8, returns ErrCompile instead of panicking.
func TestNewCompileError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Profile Profile
	}{{ // Test 0: Invalid UTF-8 in a char swap.
		Profile: Profile{CharReplace: map[string]string{"\xff": "x"}},
	}, { // Test 1: Invalid UTF-8 in a phrase.
		Profile: Profile{PhraseReplace: map[string]string{"\xff": ""}},
	}, { // Test 2: Invalid UTF-8 in a block word.
		Profile: Profile{BlockWords: []string{"\xff"}},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if _, err := New(test.Profile); !errors.Is(err, ErrCompile) {
				t.Errorf("New: err = %v, want ErrCompile", err)
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

// TestPhraseWordBoundary checks that a phrase ending in a word character matches only as a
// whole word, so a key never fires inside a longer word, while a phrase ending in
// punctuation stays bounded by that punctuation.
func TestPhraseWordBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Phrases    map[string]string
		In         string
		WantResult string
	}{{ // Test 0: A bare-word swap does not fire inside a longer word.
		Phrases: map[string]string{"cat": "dog"}, In: "the category holds a cat",
		WantResult: "the category holds a dog",
	}, { // Test 1: A leading boundary keeps the key off a word's tail.
		Phrases: map[string]string{"cat": "dog"}, In: "scatter the cat",
		WantResult: "scatter the dog",
	}, { // Test 2: A word-ending multi-word swap fires only on the whole phrase.
		Phrases: map[string]string{"in order to": "to"}, In: "in order to win, in order toward loss",
		WantResult: "to win, in order toward loss",
	}, { // Test 3: A word-ending deletion does not fire inside a longer word.
		Phrases: map[string]string{"basically": ""}, In: "so basically it works but basicallyglued stays",
		WantResult: "so it works but basicallyglued stays",
	}, { // Test 4: A comma-ending phrase stays bounded by its comma.
		Phrases: map[string]string{"in summary, ": ""}, In: "let me begin summary, then go",
		WantResult: "let me begin summary, then go",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{PhraseReplace: test.Phrases})
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

// mustSanitizer builds a Sanitizer from the default profile or fails the test.
func mustSanitizer(t *testing.T) *Sanitizer {
	t.Helper()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}
