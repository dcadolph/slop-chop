package sanitize

import (
	"math"
	"regexp"
	"strings"
)

// The clause habit covers prose in which nearly every sentence carries a qualifier or an
// aside joined on by a comma: "The tool watches the directory, uploading each new file as
// it lands, and logs any failure to a file in your home directory." One of those is
// writing. A page with no plain sentence in it is the model register, which qualifies
// everything and states nothing flat. No single sentence is the tell, so nothing is
// flagged in the text: the habit is the share, and it is scored the way hedging and
// cadence are.
//
// On the long-form development half, machine passages carry a comma in 75 percent of their
// prose sentences at the median against 50 percent for human passages, a Cohen's d of
// 1.74, and 2.26 on the holdout half. The substitution attack removes none of it, since a
// lookup swaps words and this reads punctuation.

// clauseThreshold is the share of comma-bearing sentences above which the habit counts,
// and clauseMaxPoints is the most a page with a comma in every sentence can add. The
// threshold sits at the ninetieth percentile of human long-form prose on the development
// half, 0.68, so one human passage in ten takes some penalty, and the cap is held low
// enough that a human passage at the ninety-ninth percentile, 0.84, gains under four
// points.
const (
	clauseThreshold = 0.70
	clauseMaxPoints = 8
	// clauseMinSentences is how many prose sentences the signal needs before it acts. A
	// share read over three sentences is noise.
	clauseMinSentences = 6
	// clauseMinWords is the shortest sentence that counts. Anything shorter is a list
	// stub, a heading, or a fragment, which carries no clause either way.
	clauseMinWords = 5
)

// clauseNumberComma matches the thousands separator inside a number, which is not a
// clause boundary.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var clauseNumberComma = regexp.MustCompile(`\d,\d`)

// clauseShare returns the share of prose sentences in text that carry a comma, and how
// many prose sentences there are. A prose sentence sits outside code, tables, headings,
// and block quotes, and runs at least clauseMinWords words. A list item that is a sentence
// counts as one; the word floor is what drops the stubs.
func clauseShare(text string) (share float64, sentences int) {
	prose := maskCode(text)
	with := 0
	for _, sp := range sentenceSpans(prose) {
		if inTableRow(prose, sp.start) {
			continue
		}
		s := strings.TrimSpace(prose[sp.start:sp.end])
		if s == "" || markupOpener(s[0]) || len(strings.Fields(s)) < clauseMinWords {
			continue
		}
		sentences++
		if strings.Contains(clauseNumberComma.ReplaceAllString(s, "00"), ",") {
			with++
		}
	}
	if sentences == 0 {
		return 0, 0
	}
	return float64(with) / float64(sentences), sentences
}

// markupOpener reports whether a sentence opens on a heading, table, or block quote
// marker, where the comma count says nothing about the prose.
func markupOpener(c byte) bool {
	return c == '#' || c == '|' || c == '>'
}

// clausePoints returns what the clause habit adds to the score: nothing under the sentence
// floor or the threshold, then a gradient up to the cap at a comma in every sentence.
func clausePoints(text string) float64 {
	share, sentences := clauseShare(text)
	if sentences < clauseMinSentences || share <= clauseThreshold {
		return 0
	}
	return math.Min(clauseMaxPoints, clauseMaxPoints*(share-clauseThreshold)/(1-clauseThreshold))
}
