package sanitize

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// legacyBlockRe builds the alternation regex the block rule compiled to before the
// scanner replaced it, so the two can be compared on the same inputs.
func legacyBlockRe(t *testing.T, words []string) *regexp.Regexp {
	t.Helper()
	alts := slices.Clone(words)
	slices.SortFunc(alts, func(a, b string) int {
		if d := len(b) - len(a); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	parts := make([]string, len(alts))
	for i, w := range alts {
		parts[i] = `\b` + flexSpaces(regexp.QuoteMeta(w)) + `\b`
	}
	re, err := regexp.Compile(`(?i)(?:` + strings.Join(parts, "|") + `)`)
	if err != nil {
		t.Fatalf("legacy compile: %v", err)
	}
	return re
}

// TestBlockScanMatchesLegacyRegex runs the scanner and the old alternation over the
// benchmark corpus and a set of boundary cases and requires identical match ranges.
func TestBlockScanMatchesLegacyRegex(t *testing.T) {
	t.Parallel()
	words := DefaultProfile().BlockWords
	idx := newBlockIndex(words)
	re := legacyBlockRe(t, words)

	inputs := []string{
		"a robust plan",
		"Robust plans and ROBUST plans and robustness alike.",
		"the cutting-edge tool has a cutting edge",
		"we went on a marathon, not a sprint, together",
		"harness the\npower of nothing",
		"harness the\r\npower of nothing",
		"harness the\n\npower of nothing",
		"a treasure\ttrove of examples",
		"unrobust and robuster stay whole; robust does not",
		"state-of-the-art at the start. At the end, state-of-the-art.",
		"deep dive and a deep diver",
		"we've got you covered, so you're in good hands",
		"look no furthermore",
		"",
	}
	for _, p := range loadCorpus(t) {
		inputs = append(inputs, p.Text)
	}

	for testNum, in := range inputs {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var wantLocs, gotLocs [][2]int
			for _, loc := range re.FindAllStringIndex(in, -1) {
				wantLocs = append(wantLocs, [2]int{loc[0], loc[1]})
			}
			for _, loc := range idx.scan(in) {
				gotLocs = append(gotLocs, [2]int{loc[0], loc[1]})
			}
			if diff := cmp.Diff(wantLocs, gotLocs); diff != "" {
				t.Errorf("scanner disagrees with the legacy regex on %.60q (-want +got):\n%s", in, diff)
			}
		})
	}
}
