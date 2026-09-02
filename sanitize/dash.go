package sanitize

import "strings"

// The em-dash swap, the rule that has to read context before it fires: a dash pair around
// an aside is not the dash splicing two clauses, and neither is a dateline.

// The em-dash swap is the one character rule that reads its surroundings. A lone dash
// stands in for a comma, which is what models overuse it for. A matched pair fencing a
// phrase that opens on a coordinating conjunction is doing emphasis instead, and commas
// there produce "a comprehensive, and robust, plan", which is worse English than the
// input. Dropping that pair gives "a comprehensive and robust plan".
const (
	// emDash is the character the swap is keyed on.
	emDash = "—"
	// emDashComma is the default replacement, the one the context check applies to.
	emDashComma = ", "
)

// dashConjunctions are the words that, opening a dash-fenced phrase, mark the pair as
// emphasis rather than an aside.
//
//nolint:gochecknoglobals // Immutable lookup.
var dashConjunctions = map[string]bool{
	"and": true, "or": true, "but": true, "nor": true, "yet": true, "so": true,
}

// emDashSwap returns the replacement for one em-dash, dropping it to a space when it is
// half of a conjunction-led pair and using def everywhere else.
func emDashSwap(def string) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		if conjunctionDashPair(text, loc[0]) {
			return " "
		}
		return def
	}
}

// conjunctionDashPair reports whether the em-dash at offset i opens or closes a pair
// fencing a phrase that starts with a coordinating conjunction. Both dashes must sit on
// one line, so a dash ending a line is never read as half of a pair.
func conjunctionDashPair(text string, i int) bool {
	start, end := lineStart(text, i), lineEnd(text, i)
	// The line is scanned with its inline code spans blanked: a dash inside backticks
	// is code, and pairing a prose dash with it silently eats the comma the prose dash
	// needs.
	line := maskInlineSpans(text[start:end])
	li := i - start
	after := li + len(emDash)
	// As the opening dash: a conjunction follows it and another dash closes the phrase.
	if dashConjunctions[firstWord(line[after:])] &&
		strings.Contains(line[after:], emDash) {
		return true
	}
	// As the closing dash: an earlier dash on this line opens a phrase whose first word
	// is a conjunction.
	if open := strings.LastIndex(line[:li], emDash); open >= 0 {
		return dashConjunctions[firstWord(line[open+len(emDash):li])]
	}
	return false
}

// maskInlineSpans returns line with every backtick-delimited span blanked to spaces,
// length preserved, so a scan over the result sees prose only.
func maskInlineSpans(line string) string {
	if !strings.Contains(line, "`") {
		return line
	}
	b := []byte(line)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		n := backtickRun(line, i)
		endSpan := spanEnd(line, i+n, n)
		if endSpan < 0 {
			i += n
			continue
		}
		for j := i; j < endSpan; j++ {
			b[j] = ' '
		}
		i = endSpan
	}
	return string(b)
}

// firstWord returns the first run of letters in s, lower-cased, skipping leading spaces.
func firstWord(s string) string {
	s = strings.TrimLeft(s, " \t")
	end := 0
	for end < len(s) && (('a' <= s[end] && s[end] <= 'z') || ('A' <= s[end] && s[end] <= 'Z')) {
		end++
	}
	return strings.ToLower(s[:end])
}

// lineEnd returns the offset of the newline ending the line holding i, or the end of the
// text when the line is the last one.
func lineEnd(text string, i int) int {
	if n := strings.IndexByte(text[i:], '\n'); n >= 0 {
		return i + n
	}
	return len(text)
}

// emDashKeep reports whether an em-dash should be swapped at all. Three human
// conventions keep their dash: interrupted speech, where the dash is pressed against a
// closing quote; a wire-service dateline, the all-caps opener before the first dash of
// the line; and a transcript speaker line, which renders false starts as spaced dashes.
func emDashKeep(text string, start, end int) bool {
	if end < len(text) {
		switch text[end] {
		case '"', '\'':
			return false
		}
		if strings.HasPrefix(text[end:], "”") || strings.HasPrefix(text[end:], "’") {
			return false
		}
	}
	return !capsPrefixLine(text[lineStart(text, start):start])
}

// capsPrefixLine reports whether prefix, the text from a line start up to a dash, is a
// dateline or speaker label: an opening run of two or more capitals, joined only by
// spaces and name punctuation, optionally closed by a colon with anything after it.
func capsPrefixLine(prefix string) bool {
	caps := 0
	for i := 0; i < len(prefix); i++ {
		switch c := prefix[i]; {
		case 'A' <= c && c <= 'Z':
			caps++
		case c == ' ' || c == '.' || c == ',' || c == '\'' || c == '-':
		case c == ':':
			return caps >= 2
		default:
			return false
		}
	}
	return caps >= 2
}
