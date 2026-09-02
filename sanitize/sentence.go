package sanitize

import (
	"strings"
	"unicode"
)

// Finding sentence boundaries, which several rules need before they can decide whether a
// match sits at the start of a sentence, inside one, or across two.

// sentenceStart reports whether offset sits at the start of a sentence: at the start of
// the text, or after sentence-ending punctuation or a line break, with any spaces in
// between ignored. A period that closes an abbreviation or an ellipsis does not end a
// sentence, so "e.g., a hammer" is never read as a sentence opening on a comma.
func sentenceStart(text string, offset int) bool {
	i := offset - 1
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i < 0 {
		return true
	}
	switch text[i] {
	case '\n', '\r', '!', '?':
		return true
	case '.':
		return !abbreviationEnd(text, i)
	}
	return false
}

// abbreviations are dotted shortenings whose period ends the word rather than the
// sentence, lower-cased without their final dot. Dotted forms like "e.g." and "i.e."
// are recognized by their internal dot and need no entry.
//
//nolint:gochecknoglobals // Immutable lookup.
var abbreviations = map[string]bool{
	"etc": true, "vs": true, "cf": true, "ca": true, "al": true, "st": true,
	"no": true, "nos": true, "dr": true, "mr": true, "mrs": true, "ms": true,
	"prof": true, "jr": true, "sr": true, "rev": true, "hon": true, "gen": true,
	"sgt": true, "capt": true, "lt": true, "col": true, "inc": true, "ltd": true,
	"co": true, "corp": true, "dept": true, "est": true, "fig": true, "figs": true,
	"vol": true, "vols": true, "pp": true, "ed": true, "eds": true, "misc": true,
	"approx": true, "appt": true, "apt": true, "ave": true, "blvd": true, "rd": true,
	"ft": true, "oz": true, "lb": true, "lbs": true, "hr": true, "hrs": true,
	"sec": true, "min": true, "yr": true, "yrs": true, "mo": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true,
	"aug": true, "sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
	"mon": true, "tue": true, "tues": true, "wed": true, "thu": true, "thurs": true,
	"fri": true, "sat": true, "sun": true,
}

// abbreviationEnd reports whether the period at i closes an abbreviation, an initial, or
// an ellipsis rather than a sentence. The destructive cleanups key off sentence starts, so
// an ambiguous period is read as an abbreviation: leaving a comma in place is recoverable,
// while capitalizing mid-sentence is corruption.
func abbreviationEnd(text string, i int) bool {
	if i > 0 && text[i-1] == '.' {
		return true
	}
	// A digit on both sides marks a decimal, a version, or an address, not a sentence
	// end, so "3.14" never splits a sentence in two.
	if i > 0 && i+1 < len(text) &&
		text[i-1] >= '0' && text[i-1] <= '9' && text[i+1] >= '0' && text[i+1] <= '9' {
		return true
	}
	j := i - 1
	for j >= 0 {
		c := text[j]
		if c == '.' || c == '_' || ('0' <= c && c <= '9') ||
			('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') {
			j--
			continue
		}
		break
	}
	token := text[j+1 : i]
	if token == "" {
		return false
	}
	if strings.Contains(token, ".") {
		return true
	}
	if len(token) == 1 && unicode.IsLetter(rune(token[0])) {
		return true
	}
	return abbreviations[strings.ToLower(token)]
}

// sentenceBounds returns the byte range of the sentence around offset, bounded by
// sentence-ending punctuation or a block break. A newline that is only a soft line wrap
// does not end the sentence, so a hard-wrapped sentence keeps its full extent and the
// semicolon list guard sees every semicolon it holds, not just the ones on one line. A
// period that closes an abbreviation does not end the sentence either.
func sentenceBounds(text string, offset int) (start, end int) {
	for i := offset - 1; i >= 0; i-- {
		if sentenceBoundary(text, i) {
			start = i + 1
			break
		}
	}
	end = len(text)
	for i := offset + 1; i < len(text); i++ {
		if sentenceBoundary(text, i) {
			end = i
			break
		}
	}
	return start, end
}

// sentenceBoundary reports whether the byte at i ends a sentence: sentence punctuation
// that is not an abbreviation, or a newline that breaks a block rather than wrapping a
// line.
func sentenceBoundary(text string, i int) bool {
	switch text[i] {
	case '!', '?':
		return true
	case '.':
		return !abbreviationEnd(text, i)
	case '\n':
		return !softWrap(text, i)
	}
	return false
}
