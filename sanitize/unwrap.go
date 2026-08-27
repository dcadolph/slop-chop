package sanitize

import "strings"

// unwrapProse returns text with every soft line wrap turned into a space, so a tell split
// across a hard-wrapped line still matches. Each newline it joins becomes a single space in
// place, which leaves the result byte for byte as long as the input, so an offset reported
// against the unwrapped copy points at the same place in the original.
//
// Only a wrap inside one block is joined. A blank line, a heading, a list marker, a table
// row, a block quote, and a fence all start a new block, and joining across one would let a
// rule match a tell that spans two blocks that were never one sentence.
func unwrapProse(text string) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	b := []byte(text)
	for i := range b {
		if b[i] == '\n' && softWrap(text, i) {
			b[i] = ' '
		}
	}
	return string(b)
}

// softWrap reports whether the newline at i is a wrap inside a block rather than a break
// between two blocks. Index i must point at a newline.
func softWrap(text string, i int) bool {
	if blankFrom(text, i+1) {
		return false
	}
	if opensBlock(text[i+1:]) {
		return false
	}
	return !closesBlock(text[lineStart(text, i):i])
}

// lineStart returns the offset of the first byte of the line holding i.
func lineStart(text string, i int) int {
	if n := strings.LastIndexByte(text[:i], '\n'); n >= 0 {
		return n + 1
	}
	return 0
}

// opensBlock reports whether the line at the start of rest begins a new Markdown block, so
// the wrap before it is a block boundary and not a continuation of the line above.
func opensBlock(rest string) bool {
	line := rest
	if n := strings.IndexByte(line, '\n'); n >= 0 {
		line = line[:n]
	}
	line = strings.TrimLeft(line, " \t")
	if line == "" {
		return true
	}
	switch line[0] {
	case '#', '>', '|', '`', '~', '=':
		return true
	case '-', '*', '+':
		// A bullet marker is a space or the end of the line after the mark. A word
		// starting with one of these, like "*emphasis*", is prose and joins.
		return len(line) == 1 || line[1] == ' ' || line[1] == '\t'
	}
	return orderedMarker(line)
}

// orderedMarker reports whether line opens with an ordered list marker, digits followed by
// a dot or a parenthesis and then a space.
func orderedMarker(line string) bool {
	n := 0
	for n < len(line) && line[n] >= '0' && line[n] <= '9' {
		n++
	}
	if n == 0 || n+1 >= len(line) {
		return false
	}
	if line[n] != '.' && line[n] != ')' {
		return false
	}
	return line[n+1] == ' ' || line[n+1] == '\t'
}

// closesBlock reports whether line is a block that never continues onto the next line, so
// its text is not joined with what follows.
func closesBlock(line string) bool {
	line = strings.TrimLeft(line, " \t")
	if line == "" {
		return true
	}
	switch line[0] {
	case '#', '|', '`', '~':
		return true
	}
	return false
}
