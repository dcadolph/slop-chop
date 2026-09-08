package sanitize

import (
	"strings"
	"unicode"
)

// Polyptoton detection covers a stem turned against itself inside one sentence:
// "verification that does not verify", "governance that does not govern". Ordinary
// technical prose repeats a stem all the time, since the parser parses and the scheduler
// schedules, so the repetition alone is not the tell. The tell is the turn: a pivot and a
// negation standing between the two forms, which makes the sentence a rhetorical figure
// rather than a description.

// polyptotonMaxGap is the most words that may sit between the two forms. Past it the
// second form is a new thought rather than the first one turned around.
const polyptotonMaxGap = 5

// polyptotonMinWord is the shortest word that can carry a root here, and polyptotonMinRoot
// is the shortest stem left after a suffix comes off. Short words collide by accident, so
// both floors keep noise out.
const (
	polyptotonMinWord = 5
	polyptotonMinRoot = 4
)

//nolint:gochecknoglobals // Immutable lookups.
var (
	// polyptotonPivots open the clause that turns the stem around. "without" carries its
	// own negation, so it stands as pivot and negation at once.
	polyptotonPivots = map[string]bool{
		"that": true, "which": true, "without": true, "but": true, "yet": true,
	}

	// polyptotonNegations are the words that turn the second form against the first.
	polyptotonNegations = map[string]bool{
		"not": true, "never": true, "no": true, "nothing": true, "none": true,
		"cannot": true, "without": true, "nobody": true,
	}

	// polyptotonSuffixes come off a word to reach its stem, longest first so "ification"
	// wins over "ation" and "ity" wins over "y". Only one comes off per word.
	polyptotonSuffixes = []string{
		"ifications", "ification", "ications", "ication", "izations", "isations",
		"ization", "isation",
		"ements", "ement", "ments", "ment", "ances", "ance", "ences", "ence",
		"ations", "ation", "tions", "tion", "sions", "sion", "ities", "ity",
		"nesses", "ness", "ings", "ing", "ives", "ive", "izes", "ize", "ises",
		"ise", "ers", "er", "ors", "or", "ies", "ied", "ed", "es", "s", "y",
	}

	// polyptotonStopwords are the long function words that would otherwise pair off on a
	// shared prefix without sharing a root.
	polyptotonStopwords = map[string]bool{
		"which": true, "these": true, "those": true, "there": true, "their": true,
		"would": true, "could": true, "should": true, "about": true, "after": true,
		"before": true, "where": true, "while": true, "other": true, "another": true,
		"every": true, "never": true, "cannot": true, "without": true, "because": true,
		"though": true, "although": true, "something": true, "nothing": true,
		"anything": true, "everything": true, "themselves": true, "itself": true,
	}
)

// polyptotonFindings reports every stem turned against itself in text. The finding only
// flags: unwinding the figure means saying plainly what the sentence was dressing up,
// which is the author's call or the rewrite pass's.
func polyptotonFindings(text string, protected [][2]int) []Finding {
	var out []Finding
	for _, sp := range sentenceSpans(text) {
		if inTableRow(text, sp.start) {
			continue
		}
		start, end, ok := polyptotonSpan(text[sp.start:sp.end])
		if !ok {
			continue
		}
		start, end = sp.start+start, sp.start+end
		if overlapsAny(protected, start, end) {
			continue
		}
		out = append(out, Finding{
			Rule:   "structural:polyptoton",
			Match:  text[start:end],
			Offset: start,
			order:  anaphoraOrder,
		})
	}
	return out
}

// polyptotonSpan returns the byte range of the first turned stem in sentence. It reports
// false when the sentence holds none.
func polyptotonSpan(sentence string) (start, end int, ok bool) {
	words := wordSpans(sentence)
	for i := range words {
		first := words[i].word
		if !polyptotonCandidate(first) {
			continue
		}
		for j := i + 1; j < len(words) && j-i <= polyptotonMaxGap+1; j++ {
			second := words[j].word
			if !polyptotonCandidate(second) || second == first {
				continue
			}
			if !sameRoot(first, second) || !polyptotonTurn(words[i+1:j]) {
				continue
			}
			return words[i].start, words[j].end, true
		}
	}
	return 0, 0, false
}

// polyptotonTurn reports whether the words between the two forms turn the second against
// the first. That takes a pivot and a negation, or the single word "without", which is
// both. Repetition without a turn is the ordinary technical sentence where the parser
// parses, and it is left alone.
func polyptotonTurn(between []wordSpan) bool {
	pivot, negation := false, false
	for _, w := range between {
		if polyptotonPivots[w.word] {
			pivot = true
		}
		if polyptotonNegations[w.word] || strings.HasSuffix(w.word, "n't") {
			negation = true
		}
	}
	return pivot && negation
}

// polyptotonCandidate reports whether word can carry a root: a long enough plain word
// that is not a function word.
func polyptotonCandidate(word string) bool {
	if len([]rune(word)) < polyptotonMinWord || polyptotonStopwords[word] {
		return false
	}
	for _, r := range word {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// sameRoot reports whether two words are forms of one root, judged by the stem each one
// is left with once a suffix comes off. A shared opening is not enough on its own:
// "constraint" and "construct" start alike for six letters and share no root, while
// "measurement" and "measuring" agree only after the stripping.
func sameRoot(a, b string) bool {
	return polyptotonStem(a) == polyptotonStem(b)
}

// polyptotonStem reduces a word to a crude stem: one suffix off the end, then a trailing
// "e" so "measure" and "measuring" land together. It is not a real stemmer and does not
// need to be. A suffix that would leave too little behind is left on, which is what keeps
// "final" from becoming "fin".
func polyptotonStem(word string) string {
	for _, suffix := range polyptotonSuffixes {
		stem, ok := strings.CutSuffix(word, suffix)
		if !ok || len([]rune(stem)) < polyptotonMinRoot {
			continue
		}
		// A doubled "s" belongs to the word, so "process" never loses one to the plural.
		if suffix == "s" && strings.HasSuffix(stem, "s") {
			continue
		}
		word = stem
		break
	}
	return strings.TrimSuffix(word, "e")
}

// wordSpan is one located word of a sentence, lower-cased and stripped of the punctuation
// around it.
type wordSpan struct {
	// start and end are the byte range of the bare word within the sentence.
	start, end int
	// word is the bare word, lower-cased.
	word string
}

// wordSpans locates every word of sentence, dropping the punctuation around each one so a
// token compares against the word lists. An apostrophe inside a word is kept, so a
// contraction stays one word.
func wordSpans(sentence string) []wordSpan {
	var out []wordSpan
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		word := strings.Trim(sentence[start:end], "'")
		if word != "" {
			out = append(out, wordSpan{start: start, end: start + len(word), word: strings.ToLower(word)})
		}
		start = -1
	}
	for i, r := range sentence {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '-' {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(sentence))
	return out
}
