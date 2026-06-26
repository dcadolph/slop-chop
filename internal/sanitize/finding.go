package sanitize

import "fmt"

// Finding records one place where a rule matched the text.
type Finding struct {
	// Rule is the name of the rule that matched.
	Rule string
	// Match is the exact substring that matched.
	Match string
	// Replacement is what the match was or would be replaced with. Empty means the
	// rule only flags and does not rewrite.
	Replacement string
	// Offset is the byte offset of the match in the text it was found in.
	Offset int
	// Line is the one-based line number of the match.
	Line int
	// Col is the one-based column (rune count) within the line.
	Col int
}

// String renders the finding as a single CI-friendly line.
func (f Finding) String() string {
	if f.Replacement == "" {
		return fmt.Sprintf("%d:%d %s: %q", f.Line, f.Col, f.Rule, f.Match)
	}
	return fmt.Sprintf("%d:%d %s: %q -> %q", f.Line, f.Col, f.Rule, f.Match, f.Replacement)
}
