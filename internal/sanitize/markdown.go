package sanitize

import (
	"sort"
	"strings"
)

// codeRanges returns the byte ranges of markdown code in text: fenced blocks and inline
// code spans. Rules skip matches inside these ranges, so the engine never rewrites code.
func codeRanges(text string) [][2]int {
	fences := fenceRanges(text)
	ranges := append([][2]int{}, fences...)
	ranges = append(ranges, inlineCodeRanges(text, fences)...)
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
	return ranges
}

// fenceRanges returns the byte ranges of fenced code blocks. A block runs from its
// opening fence line through its closing fence line, or to the end of the text when
// the fence never closes.
func fenceRanges(text string) [][2]int {
	var ranges [][2]int
	openStart := -1
	var openMark byte
	openLen := 0

	pos := 0
	for pos <= len(text) {
		lineEnd := len(text)
		if i := strings.IndexByte(text[pos:], '\n'); i >= 0 {
			lineEnd = pos + i
		}
		mark, n, rest := fenceMarker(text[pos:lineEnd])
		switch {
		case openStart < 0 && n > 0:
			openStart, openMark, openLen = pos, mark, n
		case openStart >= 0 && n >= openLen && mark == openMark && strings.TrimSpace(rest) == "":
			ranges = append(ranges, [2]int{openStart, lineEnd})
			openStart = -1
		}
		if lineEnd == len(text) {
			break
		}
		pos = lineEnd + 1
	}
	if openStart >= 0 {
		ranges = append(ranges, [2]int{openStart, len(text)})
	}
	return ranges
}

// fenceMarker reports the fence at the start of line: the marker byte, the run length,
// and the rest of the line after the run. A line that opens no fence returns 0, 0, "".
func fenceMarker(line string) (mark byte, n int, rest string) {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0, ""
	}
	mark = line[i]
	j := i
	for j < len(line) && line[j] == mark {
		j++
	}
	if j-i < 3 {
		return 0, 0, ""
	}
	return mark, j - i, line[j:]
}

// inlineCodeRanges returns the byte ranges of inline code spans outside the given
// fenced blocks. A span opens with a run of backticks and closes at the next run of
// the same length.
func inlineCodeRanges(text string, fences [][2]int) [][2]int {
	var ranges [][2]int
	f := 0
	i := 0
	for i < len(text) {
		for f < len(fences) && i >= fences[f][1] {
			f++
		}
		if f < len(fences) && i >= fences[f][0] {
			i = fences[f][1]
			continue
		}
		if text[i] != '`' {
			i++
			continue
		}
		n := backtickRun(text, i)
		if end := spanEnd(text, i+n, n); end >= 0 {
			ranges = append(ranges, [2]int{i, end})
			i = end
			continue
		}
		i += n
	}
	return ranges
}

// spanEnd returns the end offset of the inline span opened by a run of n backticks, or
// -1 when no closing run of exactly n backticks appears before the next blank line.
// Stopping at a blank line keeps one stray backtick from hiding the rest of the text
// from the rules.
func spanEnd(text string, from, n int) int {
	j := from
	for j < len(text) {
		switch {
		case text[j] == '\n' && blankFrom(text, j+1):
			return -1
		case text[j] == '`':
			m := backtickRun(text, j)
			if m == n {
				return j + m
			}
			j += m
		default:
			j++
		}
	}
	return -1
}

// backtickRun returns the length of the backtick run starting at i.
func backtickRun(text string, i int) int {
	j := i
	for j < len(text) && text[j] == '`' {
		j++
	}
	return j - i
}

// blankFrom reports whether the line starting at i is blank: only spaces and tabs
// before the next newline or the end of the text.
func blankFrom(text string, i int) bool {
	for ; i < len(text); i++ {
		switch text[i] {
		case ' ', '\t':
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

// overlapsAny reports whether [start, end) overlaps any range in ranges, which must be
// sorted by start offset.
func overlapsAny(ranges [][2]int, start, end int) bool {
	for _, r := range ranges {
		if r[0] >= end {
			return false
		}
		if r[1] > start {
			return true
		}
	}
	return false
}
