package sanitize

import "strings"

// ignoreToken silences the line it sits on. ignoreNextToken silences the line after it.
// Both usually sit inside an HTML comment so they read as a directive, not prose.
const (
	ignoreToken     = "slop-chop-ignore"
	ignoreNextToken = "slop-chop-ignore-next-line"
)

// skipRanges returns every byte range the rules must leave alone: protected code spans,
// Markdown structure and front matter, and lines silenced by an inline ignore directive. The
// sources are merged so the result is sorted and disjoint, which overlapsAny relies on to
// decide whether a match is protected.
func skipRanges(text string) [][2]int {
	ranges := append(codeRanges(text), ignoredRanges(text)...)
	ranges = append(ranges, structuralRanges(text)...)
	return mergeRanges(ranges)
}

// ignoredRanges returns the byte ranges of lines silenced by an ignore directive. A line
// holding the next-line directive silences the line after it, and a line holding the plain
// directive silences itself. The next-line form is checked first, since its token contains
// the plain token as a prefix.
func ignoredRanges(text string) [][2]int {
	if !strings.Contains(text, ignoreToken) {
		return nil
	}
	lines := lineRanges(text)
	var ranges [][2]int
	for i, lr := range lines {
		line := text[lr[0]:lr[1]]
		switch {
		case strings.Contains(line, ignoreNextToken):
			if i+1 < len(lines) {
				ranges = append(ranges, lines[i+1])
			}
		case strings.Contains(line, ignoreToken):
			ranges = append(ranges, lr)
		}
	}
	return ranges
}

// lineRanges returns the byte range of each line in text, the newline excluded. A trailing
// newline still yields a final empty range, which no match can overlap.
func lineRanges(text string) [][2]int {
	var ranges [][2]int
	start := 0
	for i := range len(text) {
		if text[i] == '\n' {
			ranges = append(ranges, [2]int{start, i})
			start = i + 1
		}
	}
	return append(ranges, [2]int{start, len(text)})
}
