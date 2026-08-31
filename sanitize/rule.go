package sanitize

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// Rule is one compiled check applied to text. A rule either rewrites its matches or
// only flags them.
type Rule struct {
	// Name identifies the rule in findings.
	Name string
	// re is the compiled pattern the rule matches. It is nil when matchFunc is set.
	re *regexp.Regexp
	// matchFunc scans text directly and returns match ranges in FindAll shape. It
	// replaces re for rules whose match set is too large for one regex to scan cheaply,
	// like the block-word list.
	matchFunc func(text string) [][]int
	// repl is the static replacement string when replFunc is nil.
	repl string
	// replFunc computes a replacement from the text and the submatch index slice of the
	// match, as returned by FindAllStringSubmatchIndex: loc[0:2] is the whole match and
	// loc[2:] holds each capture group, so a rewrite can expand a backreference against the
	// original text with its context intact. It takes priority over repl.
	replFunc func(text string, loc []int) string
	// keep decides whether the match at [start, end) counts. A nil keep accepts every
	// match. It lets a rule skip matches by context, like a semicolon that separates
	// list items instead of joining two clauses.
	keep func(text string, start, end int) bool
	// allow is the shared set of matched texts to skip, lower-cased. A nil set skips
	// nothing. It lets a profile exempt a word every rule would otherwise act on.
	allow map[string]bool
	// nameByMatch reports that a finding's rule name is Name plus the matched text, so one
	// combined rule can flag many words while each finding still names the word it caught.
	nameByMatch bool
	// rewrite reports whether the rule changes text. When false the rule only flags.
	rewrite bool
	// unwrap runs the rule a second time over a copy of the text whose soft line wraps
	// have been joined, so a multi-word tell a hard wrap split in two is still caught. Only
	// flag rules set it: the phrase and word rules already widen their own spaces, and a
	// rewrite must key off the text exactly as written.
	unwrap bool
	// tidy marks a punctuation or spacing cleanup rule. Tidy rules run in a fixpoint loop
	// after the content swaps, since they interact with each other: a semicolon split can
	// leave a space that space-before-punct then trims. Content swaps run once instead, so
	// a swap whose replacement contains its own trigger cannot feed on its own output.
	tidy bool
}

// matches returns the byte ranges of every match the rule keeps, dropping any that
// touch a protected range, like markdown code.
//
// An unwrap rule also scans unwrapped, the same text with its soft line wraps joined. That
// copy is the same length as text, so the offsets it reports index the original directly.
// Callers with no unwrapped copy pass text itself, which costs one extra scan of a string
// that cannot match anything the first scan missed.
func (r Rule) matches(text, unwrapped string, protected [][2]int) [][]int {
	var locs [][]int
	if r.matchFunc != nil {
		locs = r.matchFunc(text)
	} else {
		locs = r.re.FindAllStringSubmatchIndex(text, -1)
		if r.unwrap && unwrapped != text {
			locs = mergeLocs(locs, r.re.FindAllStringSubmatchIndex(unwrapped, -1))
		}
	}
	kept := locs[:0]
	for _, loc := range locs {
		if overlapsAny(protected, loc[0], loc[1]) {
			continue
		}
		if r.keep != nil && !r.keep(text, loc[0], loc[1]) {
			continue
		}
		if r.allow != nil && r.allow[strings.ToLower(text[loc[0]:loc[1]])] {
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

// findingName returns the name to record for a match. A plain rule uses Name. A rule that
// sets nameByMatch appends the matched text, whitespace collapsed and lower-cased, so a
// combined block-word rule reports "word:<the word>" rather than a bare "word".
func (r Rule) findingName(match string) string {
	if !r.nameByMatch {
		return r.Name
	}
	return r.Name + ":" + strings.ToLower(strings.Join(strings.Fields(match), " "))
}

// apply rewrites every kept match in text and returns the result. It honors keep and
// the protected ranges, so a rule rewrites exactly the matches it also reports.
func (r Rule) apply(text string, protected [][2]int) string {
	locs := r.matches(text, text, protected)
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

// mergeLocs folds the matches found on the unwrapped copy into those found on the original,
// keeping the list sorted by start offset. A match that overlaps one already held is
// dropped, so a tell a wrap split reports once rather than once per copy it was found in.
// The originals win every overlap, since they mark the text exactly as it is written. Both
// inputs arrive sorted and internally non-overlapping, the shape FindAll produces, so each
// extra needs one binary search against base rather than a scan of everything held.
func mergeLocs(base, extra [][]int) [][]int {
	out := base
	for _, loc := range extra {
		if !locOverlaps(base, loc) {
			out = append(out, loc)
		}
	}
	slices.SortFunc(out, func(a, b []int) int { return cmp.Compare(a[0], b[0]) })
	return out
}

// locOverlaps reports whether loc shares any byte with a match in locs, which must be
// sorted by start offset and non-overlapping. Only the last range starting at or before
// loc's end can overlap it, so a binary search finds the one candidate.
func locOverlaps(locs [][]int, loc []int) bool {
	i, _ := slices.BinarySearchFunc(locs, loc, func(a, b []int) int { return cmp.Compare(a[0], b[1]) })
	// locs[i-1] is the last range with start < loc[1]; every later range starts at or
	// past loc's end and cannot overlap.
	return i > 0 && locs[i-1][1] > loc[0]
}
