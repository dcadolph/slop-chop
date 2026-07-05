package sanitize

import (
	"regexp"
	"strings"
)

// Rule is one compiled check applied to text. A rule either rewrites its matches or
// only flags them.
type Rule struct {
	// Name identifies the rule in findings.
	Name string
	// re is the compiled pattern the rule matches.
	re *regexp.Regexp
	// repl is the static replacement string when replFunc is nil.
	repl string
	// replFunc computes a replacement from the text and the byte range of the match, so
	// a rewrite can depend on where the match sits. It takes priority over repl.
	replFunc func(text string, loc []int) string
	// keep decides whether the match at [start, end) counts. A nil keep accepts every
	// match. It lets a rule skip matches by context, like a semicolon that separates
	// list items instead of joining two clauses.
	keep func(text string, start, end int) bool
	// rewrite reports whether the rule changes text. When false the rule only flags.
	rewrite bool
}

// matches returns the byte ranges of every match the rule keeps, dropping any that
// touch a protected range, like markdown code.
func (r Rule) matches(text string, protected [][2]int) [][]int {
	locs := r.re.FindAllStringIndex(text, -1)
	kept := locs[:0]
	for _, loc := range locs {
		if overlapsAny(protected, loc[0], loc[1]) {
			continue
		}
		if r.keep != nil && !r.keep(text, loc[0], loc[1]) {
			continue
		}
		kept = append(kept, loc)
	}
	return kept
}

// replacement returns the rewrite for the match at loc in text.
func (r Rule) replacement(text string, loc []int) string {
	if r.replFunc != nil {
		return r.replFunc(text, loc)
	}
	return r.repl
}

// apply rewrites every kept match in text and returns the result. It honors keep and
// the protected ranges, so a rule rewrites exactly the matches it also reports.
func (r Rule) apply(text string, protected [][2]int) string {
	locs := r.matches(text, protected)
	if len(locs) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(text[last:loc[0]])
		b.WriteString(r.replacement(text, loc))
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}
