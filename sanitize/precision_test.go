package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestFixAbbreviations checks that a period closing an abbreviation or an ellipsis never
// reads as a sentence end, so the orphan-comma cleanup does not eat the clause after
// "e.g.," and its kin. Each case was a corruption before the abbreviation guard.
func TestFixAbbreviations(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		In   string
		Want string
	}{
		{ // Test 0: "e.g.," keeps its clause.
			In: "Use a tool, e.g., a hammer.", Want: "Use a tool, e.g., a hammer.",
		},
		{ // Test 1: "i.e.," keeps its clause.
			In: "The fix, i.e., the patch, works.", Want: "The fix, i.e., the patch, works.",
		},
		{ // Test 2: "etc.," keeps its clause.
			In: "Hammers, nails, etc., are tools.", Want: "Hammers, nails, etc., are tools.",
		},
		{ // Test 3: a title abbreviation keeps the aside after it.
			In: "It was Dr., no, Mr. Smith.", Want: "It was Dr., no, Mr. Smith.",
		},
		{ // Test 4: the engine's own ellipsis swap must not manufacture a sentence end.
			In: "It went on…, and on.", Want: "It went on..., and on.",
		},
		{ // Test 5: a genuine orphan comma at the text start is still cleaned.
			In: ", the plan worked.", Want: "The plan worked.",
		},
		{ // Test 6: a genuine orphan comma after a real sentence end is still cleaned.
			In: "It shipped. , then we celebrated.", Want: "It shipped. Then we celebrated.",
		},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, _ := s.Fix(test.In)
			if got != test.Want {
				t.Errorf("Fix(%q) = %q, want %q", test.In, got, test.Want)
			}
		})
	}
}

// TestCRLFBoundaries checks that CRLF text gets the same block and wrap treatment as LF
// text: a blank CRLF line is a paragraph break no structural rule may reach across, and a
// phrase split by a CRLF soft wrap is still caught.
func TestCRLFBoundaries(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Test 0: two CRLF paragraphs must not join into one "not just X but also Y" match.
	across := "This is not just a tool\r\n\r\nbut it is also a way of life\r\n"
	for _, f := range s.Check(across) {
		if f.Rule == "structural:not-just-but-also" {
			t.Errorf("structural rule matched across a CRLF paragraph break: %v", f)
		}
	}

	// Test 1: a phrase split by a CRLF soft wrap is still caught.
	wrapped := "It's worth\r\nnoting that this works."
	hit := false
	for _, f := range s.Check(wrapped) {
		if strings.HasPrefix(f.Rule, "phrase:") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("no phrase finding for a CRLF-wrapped phrase in %q", wrapped)
	}

	// Test 2: a structural tell split by a CRLF soft wrap is still caught.
	structural := "This is not just fast,\r\nit's smart."
	hit = false
	for _, f := range s.Check(structural) {
		if f.Rule == "structural:its-not-x-its-y" {
			hit = true
		}
	}
	if !hit {
		t.Errorf("no structural finding across a CRLF soft wrap in %q", structural)
	}
}

// TestSemicolonWrappedList checks that the semicolon list guard sees the whole sentence,
// not just one line: a deliberate multi-semicolon list stays intact when hard wraps put
// each semicolon on its own line.
func TestSemicolonWrappedList(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := "Whenever I grow grim about the mouth; whenever it is a damp November in my\n" +
		"soul; whenever I pause before coffin warehouses; then I account it time to sail."
	got, _ := s.Fix(in)
	if strings.Count(got, ";") != strings.Count(in, ";") {
		t.Errorf("Fix split a wrapped semicolon list:\n got %q\nwant %q", got, in)
	}
}

// TestDedupeInterleaved checks that a structural finding sorting between a rewrite and a
// flag on the same word does not hide the duplicates from each other: the bare word flag
// must still collapse into the rewrite Fix performs.
func TestDedupeInterleaved(t *testing.T) {
	t.Parallel()
	s, err := New(Profile{
		BlockWords:   []string{"delve"},
		WordReplace:  map[string]string{"delve": "dig"},
		FlagPatterns: map[string]string{"probe": `(?i)\bdelve deeper\b`},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	findings := s.Check("we delve deeper now")
	for _, f := range findings {
		if f.Rule == "word:delve" {
			t.Errorf("word:delve survived dedupe next to its replace finding: %v", findings)
		}
	}
}
