package sanitize

import "strings"

// Drumbeat detection covers the landing habit: short sentences that snap shut on a bare
// pronoun and a copula. "That's a product." "That is interesting." "That's weak." One of
// those is a writer landing a line, an old and good move, so copula-landing steps around
// a bare pronoun subject on purpose. A page of them is something else: an assistant
// agreeing with itself in rhythm. The tell is the rate, never the instance, so nothing
// here fires until the habit is a habit.

// drumbeatMaxWords is the longest sentence that still snaps shut. Past it the sentence is
// saying something rather than landing.
const drumbeatMaxWords = 6

// drumbeatMinCount is how many landings a passage needs before the habit reads as a
// drumbeat. Below it a writer is allowed the move, which is the whole point.
const drumbeatMinCount = 4

// drumbeatMinPer100 is the rate those landings have to reach, in landings per hundred
// words. The count alone is not enough: four of them scattered through a long essay is a
// writer with a tic, while four in a page is the rhythm this reads. Both floors have to
// clear, so length can never manufacture the finding on its own.
const drumbeatMinPer100 = 1.0

// drumbeatSubjects are the bare subjects a landing snaps back to. They point at whatever
// was just said instead of naming it, which is what lets the sentence be this short.
//
//nolint:gochecknoglobals // Immutable lookup.
var drumbeatSubjects = map[string]bool{
	"that": true, "this": true, "it": true, "these": true, "those": true,
}

// drumbeatSpans returns the byte range of every bare landing in text, in order. A landing
// is a short sentence whose subject is a bare pronoun, whose verb is a copula spelled out
// or contracted, and which still has a complement after it.
func drumbeatSpans(text string) [][2]int {
	var out [][2]int
	for _, sp := range sentenceSpans(text) {
		if inTableRow(text, sp.start) {
			continue
		}
		if bareLanding(text[sp.start:sp.end]) {
			out = append(out, [2]int{sp.start, sp.end})
		}
	}
	return out
}

// bareLanding reports whether sentence is a short snap on a bare pronoun and a copula.
func bareLanding(sentence string) bool {
	words := strings.Fields(sentence)
	if len(words) < 2 || len(words) > drumbeatMaxWords {
		return false
	}
	first := landingWord(words[0])
	// A contraction carries subject and copula in one token, so "That's" and "That is"
	// are the same sentence with the same rhythm.
	if base, ok := strings.CutSuffix(first, "'s"); ok {
		return drumbeatSubjects[base]
	}
	if !drumbeatSubjects[first] || !landingCopulas[landingWord(words[1])] {
		return false
	}
	return len(words) > 2
}

// drumbeatRate returns the landings in text and their rate per hundred words of prose,
// measured against prose only so a long fenced block cannot dilute the reading.
func drumbeatRate(text string) (spans [][2]int, per100Words float64) {
	prose := maskCode(text)
	spans = drumbeatSpans(prose)
	return spans, per100(len(spans), len(strings.Fields(prose)))
}

// drumbeatSustained reports whether the landings are numerous enough and close enough
// together to read as a habit rather than as a writer landing a line now and then.
func drumbeatSustained(spans [][2]int, per100Words float64) bool {
	return len(spans) >= drumbeatMinCount && per100Words >= drumbeatMinPer100
}

// drumbeatFindings reports the landing habit once, on the first landing, when a passage
// holds enough of them. One finding rather than one per sentence: the tell is the run, and
// naming every member of it would read as thirty separate problems instead of one habit.
func drumbeatFindings(text string, protected [][2]int) []Finding {
	spans, rate := drumbeatRate(text)
	if !drumbeatSustained(spans, rate) {
		return nil
	}
	for _, sp := range spans {
		if overlapsAny(protected, sp[0], sp[1]) {
			continue
		}
		return []Finding{{
			Rule:   "structural:landing-drumbeat",
			Match:  text[sp[0]:sp[1]],
			Offset: sp[0],
			order:  anaphoraOrder,
		}}
	}
	return nil
}
