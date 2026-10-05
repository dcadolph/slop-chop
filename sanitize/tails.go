package sanitize

import (
	"math"
	"regexp"
)

// Participle tails cover the habit of ending a sentence on a comma and an -ing clause that
// adds a consequence nobody asked for: "The Allies flew 14,000 sorties, completely
// neutralizing the Luftwaffe." One of those is ordinary writing, and a third of human
// long-form passages carry one. A page that closes sentence after sentence that way is the
// model register, so like hedging the tell is the rate, never the instance, and nothing is
// flagged in the text.
//
// On the long-form development corpus, 85 percent of machine passages carry a tail against
// 36 percent of human ones, and machine passages average about eight times the human rate.

// tailPattern matches a comma, an optional -ly adverb, and an -ing word that runs straight
// on into a clause. The clause has to open without a comma, which is what separates a tail,
// "neutralizing the defense", from a list of activities, "hiking, swimming, and running".
//
//nolint:gochecknoglobals // Compiled once, never modified.
var tailPattern = regexp.MustCompile(`,\s+(?:[a-z]+ly\s+)?([a-z]+ing)\s+[^,.!?;:\n]{3,}`)

// tailNotParticiple are -ing words that open an ordinary phrase after a comma rather than a
// tail: prepositions such as "including" and "during", and nouns that happen to end in -ing.
//
//nolint:gochecknoglobals // Immutable lookup.
var tailNotParticiple = map[string]bool{
	"including": true, "during": true, "regarding": true, "following": true,
	"according": true, "concerning": true, "considering": true, "excluding": true,
	"something": true, "nothing": true, "anything": true, "everything": true,
	"morning": true, "evening": true, "building": true, "ceiling": true, "king": true,
	"ring": true, "string": true, "spring": true, "wing": true, "sibling": true,
	"thing": true, "being": true,
}

// Participle tail scoring. The first tail is free, since one is a writer's ordinary move.
// Each tail past it adds tailPointsPerRate times its share of the sentences, capped at
// tailMaxPoints so the signal can sharpen a verdict and cannot carry one alone. On the
// long-form development corpus these values give 10 of 80 human passages any points at all,
// at a mean of 0.28, against a machine mean of 4.35.
const (
	tailMaxPoints     = 10
	tailPointsPerRate = 40
	// tailMinSentences is how many sentences the signal needs before it will act, so a
	// two-sentence reply with two tails cannot read as a habit.
	tailMinSentences = 4
)

// tailCount returns how many sentences in text end on a participle tail and how many
// sentences there are. Each sentence counts once however many tails it carries, and table
// rows are skipped, since a cell is not prose.
func tailCount(text string) (tails, sentences int) {
	for _, sp := range sentenceSpans(text) {
		if inTableRow(text, sp.start) {
			continue
		}
		sentences++
		for _, m := range tailPattern.FindAllStringSubmatch(text[sp.start:sp.end], -1) {
			if !tailNotParticiple[m[1]] {
				tails++
				break
			}
		}
	}
	return tails, sentences
}

// tailPoints returns what the participle tail habit adds to the score, read from prose
// only so a fenced block cannot dilute it.
func tailPoints(text string) float64 {
	tails, sentences := tailCount(maskCode(text))
	if sentences < tailMinSentences || tails < 2 {
		return 0
	}
	rate := float64(tails-1) / float64(sentences)
	return math.Min(tailMaxPoints, rate*tailPointsPerRate)
}
