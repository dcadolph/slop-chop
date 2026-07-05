package sanitize

import (
	"sort"
	"unicode/utf8"
)

// Sanitizer applies a compiled profile to text. Create one with New and reuse it.
type Sanitizer struct {
	// rules are the compiled rules applied in order.
	rules []Rule
}

// New compiles the profile into a Sanitizer.
func New(p Profile) (*Sanitizer, error) {
	rules, err := p.compile()
	if err != nil {
		return nil, err
	}
	return &Sanitizer{rules: rules}, nil
}

// Check reports every rule match in text without changing it. Findings are computed
// against the original text, so their positions are exact.
func (s *Sanitizer) Check(text string) []Finding {
	var findings []Finding
	newlines := newlineOffsets(text)
	for _, r := range s.rules {
		for _, loc := range r.matches(text) {
			match := text[loc[0]:loc[1]]
			var repl *string
			if r.rewrite {
				v := r.replacement(match)
				repl = &v
			}
			line, col := lineColAt(text, newlines, loc[0])
			findings = append(findings, Finding{
				Rule:        r.Name,
				Match:       match,
				Replacement: repl,
				Offset:      loc[0],
				Line:        line,
				Col:         col,
			})
		}
	}
	return findings
}

// Fix returns the cleaned text along with the findings from the original. Rules that
// only flag are reported but leave the text unchanged.
func (s *Sanitizer) Fix(text string) (string, []Finding) {
	findings := s.Check(text)
	out := text
	for _, r := range s.rules {
		if !r.rewrite {
			continue
		}
		out = r.apply(out)
	}
	return out, findings
}

// newlineOffsets returns the byte offset of every newline in text, in order. Computing
// this once lets lineColAt find a match's line without rescanning from the start.
func newlineOffsets(text string) []int {
	var offs []int
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			offs = append(offs, i)
		}
	}
	return offs
}

// lineColAt converts a byte offset into a one-based line and rune column, using a
// precomputed list of newline offsets.
func lineColAt(text string, newlines []int, offset int) (line, col int) {
	n := sort.SearchInts(newlines, offset)
	line = n + 1
	lineStart := 0
	if n > 0 {
		lineStart = newlines[n-1] + 1
	}
	col = 1 + utf8.RuneCountInString(text[lineStart:offset])
	return line, col
}
