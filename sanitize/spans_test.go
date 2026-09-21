package sanitize

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// sentenceSpans is the seam six detectors sit on: anaphora, shape, landing, polyptoton,
// drumbeat, and the voice fingerprint all read sentences through it. Until these tests it
// was covered only through those six, which meant a subtle change to it could be wrong in
// a way that moved every detector slightly and failed almost nothing. These pin the
// contract directly, so a break shows up here by name instead of as a distant symptom.

// spanTexts returns the exact substring each span covers, which is the part a caller
// actually reads.
func spanTexts(text string) []string {
	spans := sentenceSpans(text)
	out := make([]string, 0, len(spans))
	for _, sp := range spans {
		out = append(out, text[sp.start:sp.end])
	}
	return out
}

// TestSentenceSpansExact pins what a span covers, down to the byte. The terminal
// punctuation belongs to the sentence: an off-by-one that drops it compiles, reads
// plausibly, and quietly changes what every detector sees.
func TestSentenceSpansExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want []string
	}{{ // Test 0: The period belongs to the sentence it ends.
		Name: "keeps terminal punctuation",
		In:   "One thing. Two things.",
		Want: []string{"One thing.", "Two things."},
	}, { // Test 1: Every terminator ends a sentence, not just the period.
		Name: "all three terminators",
		In:   "Who? Nobody! Fine.",
		Want: []string{"Who?", "Nobody!", "Fine."},
	}, { // Test 2: A run of terminators is consumed whole rather than split.
		Name: "runs of terminators",
		In:   "Really?! Yes... Good.",
		Want: []string{"Really?!", "Yes...", "Good."},
	}, { // Test 3: An abbreviation is not a sentence end.
		Name: "abbreviation",
		In:   "Use a tool, e.g. a hammer. Then stop.",
		Want: []string{"Use a tool, e.g. a hammer.", "Then stop."},
	}, { // Test 4: A blank line ends a sentence that never closed.
		Name: "unterminated across a paragraph break",
		In:   "# A heading\n\nA sentence after it.",
		Want: []string{"# A heading", "A sentence after it."},
	}, { // Test 5: Text that simply runs out still yields its span.
		Name: "unterminated at the end",
		In:   "No terminator here",
		Want: []string{"No terminator here"},
	}, { // Test 6: Nothing at all yields nothing.
		Name: "empty",
		In:   "   \n\n  ",
		Want: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, spanTexts(test.In), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("spans mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSentenceSpansAdjacency pins the flag anaphora runs depend on: a paragraph break
// breaks the run, and a soft wrap does not.
func TestSentenceSpansAdjacency(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want []bool
	}{
		{Name: "first is never adjacent", In: "One. Two.", Want: []bool{false, true}},              // Test 0.
		{Name: "blank line breaks it", In: "One.\n\nTwo.", Want: []bool{false, false}},             // Test 1.
		{Name: "single newline does not", In: "One.\nTwo.", Want: []bool{false, true}},             // Test 2.
		{Name: "break then continue", In: "One.\n\nTwo. Three.", Want: []bool{false, false, true}}, // Test 3.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			var got []bool
			for _, sp := range sentenceSpans(test.In) {
				got = append(got, sp.adjacent)
			}
			if diff := cmp.Diff(test.Want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("adjacency mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSentenceSpansInvariants runs the real corpus through the seam and checks the
// properties every caller assumes but none of them states: spans are in order, never
// overlap, stay inside the text, are never empty, and drop no visible character. A
// detector reading a span that lost a word is wrong in a way its own tests cannot see.
func TestSentenceSpansInvariants(t *testing.T) {
	t.Parallel()
	for i, p := range loadCorpus(t) {
		spans := sentenceSpans(p.Text)
		prevEnd := 0
		for j, sp := range spans {
			switch {
			case sp.start < 0 || sp.end > len(p.Text):
				t.Fatalf("passage %d span %d is out of bounds: [%d,%d) of %d", i, j, sp.start, sp.end, len(p.Text))
			case sp.end <= sp.start:
				t.Fatalf("passage %d span %d is empty: [%d,%d)", i, j, sp.start, sp.end)
			case sp.start < prevEnd:
				t.Fatalf("passage %d span %d starts at %d, inside the previous span ending %d", i, j, sp.start, prevEnd)
			}
			// Whatever sits between two spans is text no detector will ever be shown, so
			// it has to be whitespace. Spans can also abut with no gap at all, which is
			// why this checks the gap rather than rebuilding the text from the spans.
			if gap := p.Text[prevEnd:sp.start]; strings.TrimSpace(gap) != "" {
				t.Errorf("passage %d (%s) skips %q before span %d", i, p.Note, clipRunes(gap, 60), j)
			}
			prevEnd = sp.end
		}
		if tail := p.Text[prevEnd:]; strings.TrimSpace(tail) != "" {
			t.Errorf("passage %d (%s) skips %q after the last span", i, p.Note, clipRunes(tail, 60))
		}
	}
}

// clipRunes shortens a string for a failure message without splitting a rune.
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// TestSentenceSpansKey pins the opener key anaphora groups runs by, including the two
// lengths that deliberately produce no key at all.
func TestSentenceSpansKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want string
	}{
		{Name: "two words lower-cased", In: "We Needed retries.", Want: "we needed"}, // Test 0.
		// Test 1: Only the second word is trimmed. The key keeps punctuation on the first,
		// so "We, needed" and "We needed" do not group into one run. That asymmetry is the
		// behavior as written rather than a decision anyone recorded, and it is pinned here
		// so changing it is a choice instead of an accident.
		{Name: "first word keeps its punctuation", In: "We, needed retries.", Want: "we, needed"},
		{Name: "one word has no key", In: "Stop.", Want: ""},                           // Test 2.
		{Name: "too long has no key", In: strings.Repeat("word ", 13) + ".", Want: ""}, // Test 3.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			spans := sentenceSpans(test.In)
			if len(spans) == 0 {
				t.Fatalf("no spans for %q", test.In)
			}
			if spans[0].key != test.Want {
				t.Errorf("key = %q, want %q", spans[0].key, test.Want)
			}
		})
	}
}
