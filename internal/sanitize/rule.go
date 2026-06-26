package sanitize

import "regexp"

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
	// rewrite reports whether the rule changes text. When false the rule only flags.
	rewrite bool
}

// matches returns the byte ranges of every match in text.
func (r Rule) matches(text string) [][]int {
	return r.re.FindAllStringIndex(text, -1)
}

// replacement returns the rewrite for a matched substring.
func (r Rule) replacement(match string) string {
	if r.replFunc != nil {
		return r.replFunc(match)
	}
	return r.repl
}
