package sanitize

import (
	"strings"
	"unicode"
)

// Step voice covers the procedure register, where a step reads best as an instruction that
// says who does what. It is opt-in and weighs nothing by default: the passive is a style
// choice, not a model tell, and the detector trades some precision for staying simple.

// voiceStepRule is the rule name a passive step is reported under.
const voiceStepRule = "voice:step"

//nolint:gochecknoglobals // Immutable lookup.
var (
	// passiveAux holds the forms of "be" and "get" that open a passive verb phrase.
	passiveAux = map[string]bool{
		"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
		"being": true, "get": true, "gets": true, "got": true,
	}
	// irregularParticiples holds common past participles that do not end in "ed".
	irregularParticiples = map[string]bool{
		"done": true, "made": true, "given": true, "taken": true, "written": true, "set": true,
		"put": true, "run": true, "built": true, "sent": true, "kept": true, "left": true,
		"shown": true, "chosen": true, "known": true, "seen": true, "found": true, "used": true,
	}
	// nounLeads holds determiners that, sitting between the auxiliary and the next word,
	// mark that word as a noun rather than a participle, as in "is a speed limit".
	nounLeads = map[string]bool{
		"a": true, "an": true, "the": true, "this": true, "that": true, "these": true,
		"those": true, "my": true, "your": true, "its": true, "our": true, "their": true,
	}
)

// voiceWord is one word of a sentence with its byte range in the text.
type voiceWord struct {
	// text is the word in lower case, with surrounding punctuation trimmed.
	text string
	// start is the offset of the word's first byte.
	start int
	// end is the offset just past the word's last byte.
	end int
}

// stepVoiceFindings reports each list item whose first sentence holds a passive verb phrase:
// a form of "be" or "get" followed, with at most one word between, by a past participle. It
// shares the list item and sentence segmentation of lengthFindings, so a step means the same
// thing to both rules. protected holds the ranges no rule may touch, and allow holds the
// lower-cased allow-listed words, either of which silences a match.
func stepVoiceFindings(text string, protected [][2]int, allow map[string]bool) []Finding {
	masked := lengthMask(text)
	var out []Finding
	for _, blk := range lengthBlocks(masked) {
		if !blk.step {
			continue
		}
		sents := blockSentences(masked, blk.start, blk.end)
		if len(sents) == 0 {
			continue
		}
		start, end, ok := passiveSpan(voiceWords(masked, sents[0][0], sents[0][1]), allow)
		if !ok || overlapsAny(protected, start, end) {
			continue
		}
		out = append(out, Finding{
			Rule:   voiceStepRule,
			Match:  text[start:end],
			Offset: start,
			order:  anaphoraOrder,
		})
	}
	return out
}

// voiceWords splits masked[start:end] into words, trimming punctuation from each end. A
// field with no letter, such as a dash, is dropped.
func voiceWords(masked string, start, end int) []voiceWord {
	var out []voiceWord
	i := start
	for i < end {
		for i < end && unicode.IsSpace(rune(masked[i])) {
			i++
		}
		j := i
		for j < end && !unicode.IsSpace(rune(masked[j])) {
			j++
		}
		ws, we := i, j
		for ws < we && !isWordRune(rune(masked[ws])) {
			ws++
		}
		for we > ws && !isWordRune(rune(masked[we-1])) {
			we--
		}
		if ws < we {
			out = append(out, voiceWord{text: strings.ToLower(masked[ws:we]), start: ws, end: we})
		}
		i = j
	}
	return out
}

// passiveSpan returns the byte range of the first passive verb phrase in words, from the
// auxiliary through the participle. A phrase holding an allow-listed word is skipped.
func passiveSpan(words []voiceWord, allow map[string]bool) (start, end int, ok bool) {
	for i, w := range words {
		if !passiveAux[w.text] {
			continue
		}
		for gap := 1; gap <= 2 && i+gap < len(words); gap++ {
			if gap == 2 && nounLeads[words[i+1].text] {
				break
			}
			part := words[i+gap]
			if !isParticiple(part.text) {
				continue
			}
			if allowedAny(allow, words[i:i+gap+1]) {
				break
			}
			return w.start, part.end, true
		}
	}
	return 0, 0, false
}

// isParticiple reports whether word reads as a past participle: a regular form ending in
// "ed" longer than three letters, so "red" and "bed" stay out, or a listed irregular form.
func isParticiple(word string) bool {
	return len(word) > 3 && strings.HasSuffix(word, "ed") || irregularParticiples[word]
}

// allowedAny reports whether any of words is on the allow list.
func allowedAny(allow map[string]bool, words []voiceWord) bool {
	for _, w := range words {
		if allow[w.text] {
			return true
		}
	}
	return false
}

// voiceRule reports whether rule is a step voice rule.
func voiceRule(rule string) bool {
	return strings.HasPrefix(rule, "voice:")
}

// flagClass returns "length" or "voice" for the flag-only rules that mark text for a reason
// no other rule shares, and "" for every other rule, so dedupeFindings never folds a
// flag-only finding into a finding of another kind.
func flagClass(rule string) string {
	switch {
	case lengthRule(rule):
		return "length"
	case voiceRule(rule):
		return "voice"
	}
	return ""
}
