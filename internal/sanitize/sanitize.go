package sanitize

import (
	"strings"
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
	for _, r := range s.rules {
		for _, loc := range r.matches(text) {
			match := text[loc[0]:loc[1]]
			repl := ""
			if r.rewrite {
				repl = r.replacement(match)
			}
			line, col := lineCol(text, loc[0])
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
		if r.replFunc != nil {
			out = r.re.ReplaceAllStringFunc(out, r.replFunc)
			continue
		}
		out = r.re.ReplaceAllLiteralString(out, r.repl)
	}
	return out, findings
}

// lineCol converts a byte offset into a one-based line and rune column.
func lineCol(text string, offset int) (line, col int) {
	line = 1 + strings.Count(text[:offset], "\n")
	lineStart := strings.LastIndexByte(text[:offset], '\n') + 1
	col = 1 + utf8.RuneCountInString(text[lineStart:offset])
	return line, col
}
