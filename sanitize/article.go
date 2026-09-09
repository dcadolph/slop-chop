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
	// Some acronyms are read both ways by careful writers, so neither article is
	// an error and the rule has no business picking one.
	if contestedAcronyms[letterRun(m[3])] {
		return false
	}
	return startsWithVowelSound(m[3]) != (len(m[1]) == 2)
}

// contestedAcronyms are all-caps words that careful writers pronounce two ways, so both
// articles are defensible and the rule stays quiet on either. SQL is "sequel" to one
// house and "ess-cue-ell" to the next, which is why Oracle writes "a SQL statement" and
// PostgreSQL writes "an SQL statement". A rule cannot know which a writer hears, and
// correcting someone's house style is worse than saying nothing. FAQ is deliberately
// absent: this project has already picked "an FAQ" and pins it in a test.
//
//nolint:gochecknoglobals // Immutable lookup.
var contestedAcronyms = map[string]bool{
	"SQL": true,
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

// spokenAcronyms are all-caps words a reader says rather than spells. The letter-name rule
// below is right only for an initialism read one letter at a time, where the R of RFC is
// said "ar" and takes "an". These are read as words, so the sound that matters is the
// word's own: README is "read-me" and takes "a", however its letters are named. Only the
// ones whose first letter name starts with a vowel are listed, since those are the only
// ones the letter-name rule gets wrong.
//
//nolint:gochecknoglobals // Immutable lookup.
var spokenAcronyms = map[string]bool{
	"README": true, "REST": true, "REPL": true, "RAID": true, "RAM": true, "ROM": true,
	"RADAR": true, "LASER": true, "LAN": true, "LIFO": true, "MAC": true, "MIME": true,
	"MODEM": true, "NASA": true, "NAT": true, "NUMA": true, "SAML": true, "SCUBA": true,
	"SIM": true, "SONAR": true, "SUDO": true, "FIFO": true, "FUSE": true, "HUD": true,
	"NAN": true, "MIDI": true, "SAAS": true, "SASS": true, "LILO": true,
}

// letterRun returns the leading run of letters in word, so a lookup is not defeated by the
// punctuation the word pattern admits: README, README's, and README. all answer to README.
func letterRun(word string) string {
	for i := 0; i < len(word); i++ {
		if word[i] < 'A' || word[i] > 'Z' {
			return word[:i]
		}
	}
	return word
}

// startsWithVowelSound reports whether word begins with a vowel sound, which decides between
// "a" and "an". It handles the common exceptions: silent-h words take "an", "you"-sound and
// "one"-sound words take "a" despite a leading vowel, and an all-caps initialism opening on a
// letter whose name starts with a vowel (A, E, F, H, I, L, M, N, O, R, S, X) takes "an".
// An all-caps word a reader pronounces rather than spells is judged by its sound instead.
func startsWithVowelSound(word string) bool {
	lw := strings.ToLower(strings.Trim(word, "'"))
	if lw == "" {
		return false
	}
	if word == strings.ToUpper(word) && word != lw && len(word) > 1 &&
		!spokenAcronyms[letterRun(word)] {
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
