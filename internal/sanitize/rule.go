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
	// replFunc computes a replacement from a match. It takes priority over repl.
	replFunc func(match string) string
	// keep decides whether a match at the given start offset counts. A nil keep accepts
	// every match. It lets a rule skip matches by context, like a semicolon that
	// separates list items instead of joining two clauses.
	keep func(text string, start int) bool
	// rewrite reports whether the rule changes text. When false the rule only flags.
	rewrite bool
}

// matches returns the byte ranges of every match the rule keeps.
func (r Rule) matches(text string) [][]int {
	locs := r.re.FindAllStringIndex(text, -1)
	if r.keep == nil {
		return locs
	}
	kept := locs[:0]
	for _, loc := range locs {
		if r.keep(text, loc[0]) {
			kept = append(kept, loc)
		}
	}
	return kept
}

// replacement returns the rewrite for a matched substring.
func (r Rule) replacement(match string) string {
	if r.replFunc != nil {
		return r.replFunc(match)
	}
	return r.repl
}

// apply rewrites every kept match in text and returns the result. It honors keep, so a
// rule rewrites exactly the matches it also reports.
func (r Rule) apply(text string) string {
	locs := r.matches(text)
	if len(locs) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(text[last:loc[0]])
		b.WriteString(r.replacement(text[loc[0]:loc[1]]))
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}
