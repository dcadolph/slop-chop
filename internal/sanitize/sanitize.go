package sanitize

import (
	"cmp"
	"slices"
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
// against the original text, so their positions are exact, and they come back in text
// order rather than rule order.
func (s *Sanitizer) Check(text string) []Finding {
	var findings []Finding
	protected := skipRanges(text)
	for _, r := range s.rules {
		for _, loc := range r.matches(text, protected) {
			match := text[loc[0]:loc[1]]
			var repl *string
			if r.rewrite {
				v := r.replacement(text, loc)
				repl = &v
			}
			findings = append(findings, Finding{
				Rule:        r.findingName(match),
				Match:       match,
				Replacement: repl,
				Offset:      loc[0],
			})
		}
	}
	slices.SortFunc(findings, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.Offset, b.Offset), cmp.Compare(a.Rule, b.Rule))
	})
	assignLineCol(text, findings)
	return dedupeFindings(findings)
}

// dedupeFindings collapses findings that mark the same text at the same offset, which
// happens when a rewrite rule and a flag rule both match one word. It keeps the rewrite so
// the report shows the fix rather than a bare flag. Findings arrive sorted by offset, so
// duplicates sit next to each other.
func dedupeFindings(findings []Finding) []Finding {
	out := findings[:0]
	for _, f := range findings {
		if n := len(out); n > 0 && out[n-1].Offset == f.Offset && strings.EqualFold(out[n-1].Match, f.Match) {
			if out[n-1].Replacement == nil && f.Replacement != nil {
				out[n-1] = f
			}
			continue
		}
		out = append(out, f)
	}
	return out
}

// Fix returns the cleaned text along with the findings from the original. Rules that
// only flag are reported but leave the text unchanged. Findings carry positions against
// the original text, so they are gathered before the rewriting starts.
func (s *Sanitizer) Fix(text string) (string, []Finding) {
	findings := s.Check(text)
	return s.fixpoint(text), findings
}

// fixpoint runs the rewriting rules over the text until it stops changing, so a later
// rule that alters an earlier rule's input, like the punctuation cleanup dropping a space
// the semicolon split had read, cannot leave the output half done. It is capped so a
// profile whose swaps cycle, such as a to b and b to a, terminates instead of looping.
func (s *Sanitizer) fixpoint(text string) string {
	const maxPasses = 10
	out := text
	for range maxPasses {
		next := s.applyAll(out)
		if next == out {
			break
		}
		out = next
	}
	return out
}

// applyAll runs every rewriting rule once, in order. Each rewrite shifts offsets, so the
// protected ranges are recomputed after any rule that changed the text.
func (s *Sanitizer) applyAll(text string) string {
	out := text
	protected := skipRanges(out)
	for _, r := range s.rules {
		if !r.rewrite {
			continue
		}
		next := r.apply(out, protected)
		if next != out {
			out = next
			protected = skipRanges(out)
		}
	}
	return out
}

// assignLineCol fills the one-based Line and rune Col of each finding in a single forward
// pass over text. The findings must already be sorted by offset. Walking one cursor keeps
// the pass linear, where computing each position by rescanning from its line start turns
// quadratic on a long line packed with findings, such as a minified file.
func assignLineCol(text string, findings []Finding) {
	pos, line, col := 0, 1, 1
	for i := range findings {
		for pos < findings[i].Offset {
			if text[pos] == '\n' {
				line, col, pos = line+1, 1, pos+1
				continue
			}
			_, n := utf8.DecodeRuneInString(text[pos:])
			pos, col = pos+n, col+1
		}
		findings[i].Line, findings[i].Col = line, col
	}
}
