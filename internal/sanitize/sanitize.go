package sanitize

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Sanitizer applies a compiled profile to text. Create one with New and reuse it.
type Sanitizer struct {
	// rules are the compiled rules applied in order.
	rules []Rule
	// allowPhrases matches allow-listed collocations. Its occurrences are protected from
	// every rule, so a term of art like "robust regression" keeps its word even when the
	// bare word is a tell. It is nil when the profile has no multi-word allow entries.
	allowPhrases *regexp.Regexp
	// protectQuotes leaves double-quoted spans unedited when set, so a quoted source is not
	// reworded. It mirrors Profile.ProtectQuotes.
	protectQuotes bool
}

// New compiles the profile into a Sanitizer.
func New(p Profile) (*Sanitizer, error) {
	rules, err := p.compile()
	if err != nil {
		return nil, err
	}
	phrases, err := allowPhraseRe(p.Allow)
	if err != nil {
		return nil, err
	}
	return &Sanitizer{rules: rules, allowPhrases: phrases, protectQuotes: p.ProtectQuotes}, nil
}

// Check reports every rule match in text without changing it. Findings are computed
// against the original text, so their positions are exact, and they come back in text
// order rather than rule order.
func (s *Sanitizer) Check(text string) []Finding {
	var findings []Finding
	protected := s.protectedRanges(text)
	for ri, r := range s.rules {
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
				order:       ri,
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
// happens when more than one rule matches one word. It keeps the finding Fix would act on,
// so the report never contradicts the output. Findings arrive sorted by offset, so
// duplicates sit next to each other.
func dedupeFindings(findings []Finding) []Finding {
	out := findings[:0]
	for _, f := range findings {
		if n := len(out); n > 0 && out[n-1].Offset == f.Offset && strings.EqualFold(out[n-1].Match, f.Match) {
			if preferFinding(f, out[n-1]) {
				out[n-1] = f
			}
			continue
		}
		out = append(out, f)
	}
	return out
}

// preferFinding reports whether cand should replace kept when both mark the same span. A
// rewrite beats a bare flag, since a flag leaves the span for the rewrite to change, so the
// report should show the fix. Between two rewrites the earlier rule wins, because Fix runs
// rules in order and the first to change a span is the change the output keeps: reporting a
// later rule's replacement would name a swap that never happened.
func preferFinding(cand, kept Finding) bool {
	if candRewrite, keptRewrite := cand.Replacement != nil, kept.Replacement != nil; candRewrite != keptRewrite {
		return candRewrite
	} else if candRewrite {
		return cand.order < kept.order
	}
	return false
}

// Fix returns the cleaned text along with the findings from the original. Rules that
// only flag are reported but leave the text unchanged. Findings carry positions against
// the original text, so they are gathered before the rewriting starts.
//
// The order is tidy, then content swaps once, then tidy again. The leading tidy normalizes
// spacing first, so a swap keyed on a single space still fires when the input had a run of
// them. The content swaps then apply exactly once, which is what keeps a self-referential
// swap like "use" to "make use of" from feeding on its own output. The trailing tidy cleans
// up after the swaps, which is also where article agreement is repaired once a swap has
// flipped a leading sound. Because the swaps run once between two idempotent tidy passes,
// Fix is idempotent on any profile whose swaps do not rewrite their own output.
func (s *Sanitizer) Fix(text string) (string, []Finding) {
	findings := s.Check(text)
	out := s.tidyFixpoint(text)
	out = s.applyContentSwaps(out)
	out = s.tidyFixpoint(out)
	return out, findings
}

// applyContentSwaps runs every content rewrite once, in order: character and phrase
// swaps, spelling, word replacement, deletions, and regex swaps. One pass is deliberate.
// Re-running these would let a swap whose replacement contains its own trigger, such as
// "use" to "make use of", match its own output and grow without bound, and would let a
// chain like "a" to "b" and "b" to "c" carry "a" all the way to "c". Each match is swapped
// exactly once. Offsets shift as text changes, so protected ranges are recomputed after
// any rule that edited the text.
func (s *Sanitizer) applyContentSwaps(text string) string {
	out := text
	protected := s.protectedRanges(out)
	for _, r := range s.rules {
		if !r.rewrite || r.tidy {
			continue
		}
		next := r.apply(out, protected)
		if next != out {
			out = next
			protected = s.protectedRanges(out)
		}
	}
	return out
}

// tidyFixpoint runs the punctuation and spacing cleanup rules until the text stops
// changing, so a later rule that alters an earlier rule's input, like space-before-punct
// trimming a space the semicolon split had left, cannot leave the output half done. Each
// tidy rule is idempotent on its own, so the loop settles quickly; it is capped anyway so
// it always terminates.
func (s *Sanitizer) tidyFixpoint(text string) string {
	const maxPasses = 10
	out := text
	for range maxPasses {
		next := s.applyTidy(out)
		if next == out {
			break
		}
		out = next
	}
	return out
}

// applyTidy runs every tidy rule once, in order. Each rewrite shifts offsets, so the
// protected ranges are recomputed after any rule that changed the text.
func (s *Sanitizer) applyTidy(text string) string {
	out := text
	protected := s.protectedRanges(out)
	for _, r := range s.rules {
		if !r.rewrite || !r.tidy {
			continue
		}
		next := r.apply(out, protected)
		if next != out {
			out = next
			protected = s.protectedRanges(out)
		}
	}
	return out
}

// protectedRanges returns every byte range the rules must not touch: code, Markdown
// structure, ignore lines, and any allow-listed collocation, merged so overlapsAny can rely
// on the order.
func (s *Sanitizer) protectedRanges(text string) [][2]int {
	ranges := skipRanges(text)
	extra := false
	if s.allowPhrases != nil {
		for _, loc := range s.allowPhrases.FindAllStringIndex(text, -1) {
			ranges = append(ranges, [2]int{loc[0], loc[1]})
		}
		extra = true
	}
	if s.protectQuotes {
		ranges = append(ranges, quoteRanges(text)...)
		extra = true
	}
	if extra {
		ranges = mergeRanges(ranges)
	}
	return ranges
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
