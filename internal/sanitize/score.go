package sanitize

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Score is a read on how much a text reads like AI wrote it, from 0 for clean to 100 for
// heavy slop. It sums named signals: the density of rule tells, how flat the sentence
// cadence is, and how hedge-heavy the register is, so a robotic rhythm or a noncommittal
// voice still counts where a word list finds nothing.
type Score struct {
	// Value is the 0 to 100 score, the capped sum of the signals below.
	Value int `json:"value"`
	// Tells is the number of rule findings in the text.
	Tells int `json:"tells"`
	// Words is the word count the densities are measured against.
	Words int `json:"words"`
	// TellsPer100 is tells per hundred words, the density the score leans on.
	TellsPer100 float64 `json:"tellsPer100"`
	// CadenceCV is the coefficient of variation of sentence length. A low value means a
	// flat, even rhythm, which reads as machine written.
	CadenceCV float64 `json:"cadenceCv"`
	// Density is the points tell density added to Value.
	Density int `json:"density"`
	// Cadence is the points a flat sentence rhythm added to Value.
	Cadence int `json:"cadence"`
	// Hedging is the points a hedge-heavy register added to Value.
	Hedging int `json:"hedging"`
}

// sentenceSplit breaks text on sentence-ending punctuation to measure cadence.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var sentenceSplit = regexp.MustCompile(`[.!?]+`)

// hedges are the noncommittal qualifiers whose density marks the AI register.
//
//nolint:gochecknoglobals // Immutable set.
var hedges = map[string]bool{
	"may": true, "might": true, "could": true, "possibly": true, "perhaps": true,
	"arguably": true, "generally": true, "potentially": true, "somewhat": true,
	"seemingly": true, "presumably": true, "conceivably": true, "relatively": true,
	"likely": true, "roughly": true, "fairly": true,
}

// Score rates text from 0 to 100 by tell density, cadence flatness, and hedge density.
func (s *Sanitizer) Score(text string) Score {
	tells := len(s.Check(text))
	// Measure the densities against prose only. Code is blanked so a large fenced block
	// cannot dilute the word count the signals are weighed against.
	prose := maskCode(text)
	words := len(strings.Fields(prose))

	// Tell density is the main signal. Each tell per hundred words adds eight points, capped
	// so a dense page saturates near eighty.
	density := math.Min(80, per100(tells, words)*8)

	// A flat, even rhythm reads as machine written. A coefficient of variation under 0.5
	// penalizes, cv == 0 the most. A negative cv means too few sentences to judge.
	cv := cadenceCV(prose)
	cadence := 0.0
	if cv >= 0 {
		cadence = math.Max(0, math.Min(1, (0.5-cv)/0.5)) * 20
	}

	// A hedge-heavy register, the noncommittal "may generally" voice, is a structural tell a
	// word list misses. Hedge density adds up to ten points.
	hedging := math.Min(10, per100(countHedges(prose), words)*2.5)

	value := int(math.Round(math.Min(100, density+cadence+hedging)))
	return Score{
		Value:       value,
		Tells:       tells,
		Words:       words,
		TellsPer100: math.Round(per100(tells, words)*100) / 100,
		CadenceCV:   math.Round(math.Max(0, cv)*1000) / 1000,
		Density:     int(math.Round(density)),
		Cadence:     int(math.Round(cadence)),
		Hedging:     int(math.Round(hedging)),
	}
}

// per100 returns n per hundred of d, or 0 when d is zero.
func per100(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

// countHedges counts hedge words in text, matched case-insensitively on word boundaries.
func countHedges(text string) int {
	n := 0
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if hedges[strings.ToLower(w)] {
			n++
		}
	}
	return n
}

// cadenceCV returns the coefficient of variation of sentence length in words, or -1 when
// there are too few sentences to judge a rhythm. A returned 0 is a real reading: every
// sentence is the same length, the flattest cadence there is, distinct from the -1 sentinel.
func cadenceCV(text string) float64 {
	var lengths []float64
	for _, sentence := range sentenceSplit.Split(text, -1) {
		if n := len(strings.Fields(sentence)); n > 0 {
			lengths = append(lengths, float64(n))
		}
	}
	if len(lengths) < 3 {
		return -1
	}
	var sum float64
	for _, n := range lengths {
		sum += n
	}
	mean := sum / float64(len(lengths))
	if mean == 0 {
		return -1
	}
	var variance float64
	for _, n := range lengths {
		variance += (n - mean) * (n - mean)
	}
	variance /= float64(len(lengths))
	return math.Sqrt(variance) / mean
}
