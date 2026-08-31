package sanitize

import "strings"

// The block-word scanner replaces the one-regex-per-list alternation the block rule used
// to compile to. A case-insensitive alternation of nearly two hundred branches keeps the
// regexp NFA saturated on every byte of input; indexing the terms by first word and
// probing only at word starts does the same matching in linear time.

// blockIndex holds the prepared block list: terms grouped by their lower-cased first
// word run, longest term first so the longest match wins at a position.
type blockIndex map[string][]string

// newBlockIndex prepares words for the scanner: each term lower-cased with its spacing
// collapsed, grouped by first word, longest first within a group.
func newBlockIndex(words []string) blockIndex {
	idx := make(blockIndex)
	for _, w := range words {
		term := strings.ToLower(strings.Join(strings.Fields(w), " "))
		if term == "" {
			continue
		}
		key := firstWordRun(term)
		idx[key] = append(idx[key], term)
	}
	for key, terms := range idx {
		terms := terms
		for i := 1; i < len(terms); i++ {
			for j := i; j > 0 && len(terms[j]) > len(terms[j-1]); j-- {
				terms[j], terms[j-1] = terms[j-1], terms[j]
			}
		}
		idx[key] = terms
	}
	return idx
}

// firstWordRun returns the leading run of word characters in term.
func firstWordRun(term string) string {
	i := 0
	for i < len(term) && isWordByte(term[i]) {
		i++
	}
	return term[:i]
}

// scan returns the byte ranges of every block term in text, non-overlapping and in
// order, with the same boundary and wrap behavior the regex form had: matches start and
// end on word boundaries, and a space in a term crosses runs of spaces and tabs and at
// most one line break.
func (idx blockIndex) scan(text string) [][]int {
	var locs [][]int
	var lower [64]byte
	for i := 0; i < len(text); {
		if !isWordByte(text[i]) || (i > 0 && isWordByte(text[i-1])) {
			i++
			continue
		}
		j := i
		for j < len(text) && isWordByte(text[j]) && j-i < len(lower) {
			c := text[j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			lower[j-i] = c
			j++
		}
		end := -1
		for _, term := range idx[string(lower[:j-i])] {
			if e := matchTermAt(text, i, term); e >= 0 {
				end = e
				break
			}
		}
		if end >= 0 {
			locs = append(locs, []int{i, end})
			i = end
			continue
		}
		i = j
	}
	return locs
}

// matchTermAt reports where the lower-cased term ends when matched at start in text, or
// -1 when it does not match there. Letters compare case-insensitively in ASCII, the set
// the block list uses; a space in the term matches a wsGap-shaped run of whitespace; any
// other character matches itself. A term ending in a word character requires a word
// boundary after it.
func matchTermAt(text string, start int, term string) int {
	i := start
	for j := 0; j < len(term); j++ {
		if term[j] == ' ' {
			g := skipGap(text, i)
			if g == i {
				return -1
			}
			i = g
			continue
		}
		if i >= len(text) {
			return -1
		}
		c := text[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != term[j] {
			return -1
		}
		i++
	}
	if endsWithWordChar(term) && i < len(text) && isWordByte(text[i]) {
		return -1
	}
	return i
}

// skipGap consumes the whitespace a term's space may cross: spaces and tabs with at most
// one line break, the same shape as wsGap. It returns the offset after the run, which is
// the given offset when no whitespace is there.
func skipGap(text string, i int) int {
	brokeLine := false
	for i < len(text) {
		switch text[i] {
		case ' ', '\t':
			i++
		case '\r':
			if brokeLine || i+1 >= len(text) || text[i+1] != '\n' {
				return i
			}
			brokeLine = true
			i += 2
		case '\n':
			if brokeLine {
				return i
			}
			brokeLine = true
			i++
		default:
			return i
		}
	}
	return i
}
