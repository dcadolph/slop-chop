package sanitize

import (
	"regexp"
	"strings"
)

// Fixing the article after a rewrite changes the word it introduces, so cutting a
// buzzword never leaves "a robust plan" as "a plan" when it should read "an idea".

// articleRe matches an "a" or "an" article and the word that follows, so the article can be
// corrected to the sound of that word.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var articleRe = regexp.MustCompile(`\b([Aa]n?)\b([ \t]+)([A-Za-z][A-Za-z.'-]*)`)

// silentH lists words whose leading h is silent, so they take "an" despite the consonant.
//
//nolint:gochecknoglobals // Immutable lookup.
var silentH = []string{"honest", "honor", "honour", "hour", "heir"}

// consonantVowel lists vowel-spelled prefixes that open on a consonant sound, the "you" of
// "user" and the "wun" of "one", so they take "a" despite the leading vowel.
//
//nolint:gochecknoglobals // Immutable lookup.
var consonantVowel = []string{
	"use", "user", "usu", "uni", "unit", "uniqu", "unif", "unio", "util",
	"euro", "eu", "ubiq", "ukulele", "one", "once", "ewe",
}

// fixArticle rewrites an "a"/"an" match so the article matches the sound of the next word,
// keeping the article's capitalization and the original spacing.
func fixArticle(text string, loc []int) string {
	m := articleRe.FindStringSubmatch(text[loc[0]:loc[1]])
	if m == nil {
		return text[loc[0]:loc[1]]
	}
	article, gap, word := m[1], m[2], m[3]
	corrected := "a"
	if startsWithVowelSound(word) {
		corrected = "an"
	}
	if article[0] == 'A' {
		corrected = "A" + corrected[1:]
	}
	return corrected + gap + word
}

// articleNeedsFix reports whether the article in the match disagrees with the sound of the
// word that follows, so the rule fires, and reports a finding, only when a correction is
// actually needed and never on an already-correct "a" or "an".
func articleNeedsFix(text string, start, end int) bool {
	m := articleRe.FindStringSubmatch(text[start:end])
	if m == nil {
		return false
	}
	// A capital "A" mid-sentence is a label, "Option A is ready", not the article, and
	// "correcting" it to "An" corrupts the sentence.
	if m[1] == "A" && !sentenceStart(text, start) {
		return false
	}
	// An article is followed by a noun phrase. A function word after "a" means the "a"
	// is something else, a label, a variable, a list marker, so leave it alone: the
	// em-dash pair drop can produce "a and b", and "an and b" is worse.
	if articleStopWords[strings.ToLower(m[3])] {
		return false
	}
	return startsWithVowelSound(m[3]) != (len(m[1]) == 2)
}

// articleStopWords are words that never head the noun phrase of an article, so an "a" or
// "an" directly before one is not an article at all.
//
//nolint:gochecknoglobals // Immutable lookup.
var articleStopWords = map[string]bool{
	"and": true, "or": true, "but": true, "nor": true, "yet": true, "so": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"if": true, "as": true, "at": true, "by": true, "in": true, "of": true,
	"on": true, "to": true, "the": true, "then": true, "that": true, "this": true,
	"it": true, "its": true, "with": true, "from": true, "for": true,
}

// startsWithVowelSound reports whether word begins with a vowel sound, which decides between
// "a" and "an". It handles the common exceptions: silent-h words take "an", "you"-sound and
// "one"-sound words take "a" despite a leading vowel, and an all-caps acronym opening on a
// letter whose name starts with a vowel (A, E, F, H, I, L, M, N, O, R, S, X) takes "an".
func startsWithVowelSound(word string) bool {
	lw := strings.ToLower(strings.Trim(word, "'"))
	if lw == "" {
		return false
	}
	if word == strings.ToUpper(word) && word != lw && len(word) > 1 {
		return strings.ContainsRune("AEFHILMNORSX", rune(word[0]))
	}
	for _, p := range silentH {
		if strings.HasPrefix(lw, p) {
			return true
		}
	}
	for _, p := range consonantVowel {
		if strings.HasPrefix(lw, p) {
			return false
		}
	}
	switch lw[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
