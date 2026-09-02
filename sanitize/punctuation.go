package sanitize

import (
	"strings"
	"unicode"
)

// The punctuation cleanups: the semicolon split and the comma and space tidying that
// clears the debris the content rewrites leave behind.

// trimLeadingSpace returns the match without its leading spaces and tabs, leaving just
// the punctuation.
func trimLeadingSpace(text string, loc []int) string {
	return strings.TrimLeft(text[loc[0]:loc[1]], " \t")
}

// keepFinalByte rewrites a match to its final byte. It drops a comma run pressed
// against closing punctuation, the debris left when a word between them is cut.
func keepFinalByte(text string, loc []int) string {
	return text[loc[1]-1 : loc[1]]
}

// commaOpensSentence reports whether the comma at start begins a sentence, which marks
// it as debris left when an opening word was cut. A comma anywhere else is ordinary.
func commaOpensSentence(text string, start, _ int) bool {
	return sentenceStart(text, start)
}

// stripOrphanComma drops a sentence-opening comma and the spaces after it, keeping the
// next letter as a capital, so cutting an opener like "Seamlessly," leaves a clean start.
func stripOrphanComma(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	return string(unicode.ToUpper(r[len(r)-1]))
}

// notLineStart reports whether the match at start has text before it on the same line.
// It keeps indentation, like a markdown code block leading into a dot, out of reach of
// the punctuation cleanup.
func notLineStart(text string, start, _ int) bool {
	return start > 0 && text[start-1] != '\n' && text[start-1] != '\r'
}

// spaceBeforePunctKeep reports whether a space-before-punctuation match is real cleanup and
// not Markdown structure. It keeps indentation out of reach like notLineStart, and skips the
// "!" that opens an inline image, where the space belongs before the image.
func spaceBeforePunctKeep(text string, start, end int) bool {
	if !notLineStart(text, start, end) {
		return false
	}
	return text[end-1] != '!' || end >= len(text) || text[end] != '['
}

// collapsibleRun reports whether a run of spaces should collapse. A run at the start of
// a line is indentation, a run that reaches the end of a line can be a markdown hard
// break, and a run on a table row is alignment padding, so all three stay.
func collapsibleRun(text string, start, end int) bool {
	if !notLineStart(text, start, end) || inTableRow(text, start) {
		return false
	}
	return end < len(text) && text[end] != '\n' && text[end] != '\r'
}

// inTableRow reports whether offset sits on a line whose first character is a pipe,
// which marks a markdown table row.
func inTableRow(text string, offset int) bool {
	i := offset
	for i > 0 && text[i-1] != '\n' {
		i--
	}
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return i < len(text) && text[i] == '|'
}

// splitSemicolon rewrites a "; x" match into ". X", ending the clause and capitalizing
// the next word. When the clause already ends in sentence punctuation, the semicolon is
// dropped without adding a second period, so "2.; the" does not become "2.. The".
func splitSemicolon(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	last := string(unicode.ToUpper(r[len(r)-1]))
	if loc[0] > 0 {
		switch text[loc[0]-1] {
		case '.', '!', '?':
			return " " + last
		}
	}
	return ". " + last
}

// semicolonConjunctions are the words that, right after a semicolon, mark it as a list
// separator rather than a clause join.
var semicolonConjunctions = []string{"and ", "or ", "but ", "nor ", "yet ", "so "}

// semicolonJoinsClauses reports whether the semicolon at offset semi joins two clauses,
// which is safe to split, rather than separating list items, which is not. It treats a
// semicolon as a list separator when its sentence holds more than one semicolon, or when
// a coordinating conjunction follows it, since both usually mean a deliberate list.
func semicolonJoinsClauses(text string, semi int) bool {
	start, end := sentenceBounds(text, semi)
	if strings.Count(text[start:end], ";") > 1 {
		return false
	}
	if inTableRow(text, semi) || inParens(text[start:semi]) {
		return false
	}
	rest := strings.ToLower(strings.TrimLeft(text[semi+1:end], " \t"))
	for _, conj := range semicolonConjunctions {
		if strings.HasPrefix(rest, conj) {
			return false
		}
	}
	return true
}

// inParens reports whether prefix, the text from the sentence start up to a semicolon,
// leaves a parenthesis open, which means the semicolon sits inside a parenthetical and is
// almost always a list separator rather than a clause join.
func inParens(prefix string) bool {
	return strings.Count(prefix, "(") > strings.Count(prefix, ")")
}
