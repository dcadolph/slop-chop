package sanitize

import (
	"math"
	"regexp"
	"strings"
)

// Score is a read on how much a text reads like AI wrote it, from 0 for clean to 100 for
// heavy slop. It combines the density of rule tells with how flat the sentence cadence is,
// since a robotic, even rhythm is a tell no word list catches.
type Score struct {
	// Value is the 0 to 100 score, higher meaning more slop.
	Value int `json:"value"`
	// Tells is the number of rule findings in the text.
	Tells int `json:"tells"`
	// Words is the word count the density is measured against.
	Words int `json:"words"`
	// TellsPer100 is tells per hundred words, the density the score leans on.
	TellsPer100 float64 `json:"tellsPer100"`
	// CadenceCV is the coefficient of variation of sentence length. A low value means a
	// flat, even rhythm, which reads as machine written.
	CadenceCV float64 `json:"cadenceCv"`
}

// sentenceSplit breaks text on sentence-ending punctuation to measure cadence.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var sentenceSplit = regexp.MustCompile(`[.!?]+`)

// Score rates text from 0 to 100 by tell density and cadence flatness.
func (s *Sanitizer) Score(text string) Score {
	tells := len(s.Check(text))
	words := len(strings.Fields(text))
	perHundred := 0.0
	if words > 0 {
		perHundred = float64(tells) / float64(words) * 100
	}
	// Each tell per hundred words adds eight points, so a dense page of slop saturates
	// the density term near eighty and leaves room for the cadence penalty on top.
	base := math.Min(80, perHundred*8)

	cv := cadenceCV(text)
	// A coefficient of variation under 0.5 reads as flat. The flatter it is, the larger
	// the penalty, up to twenty points, and only when there are enough sentences to judge.
	cadence := 0.0
	if cv > 0 {
		cadence = math.Max(0, math.Min(1, (0.5-cv)/0.5)) * 20
	}

	value := int(math.Round(math.Min(100, base+cadence)))
	return Score{
		Value:       value,
		Tells:       tells,
		Words:       words,
		TellsPer100: math.Round(perHundred*100) / 100,
		CadenceCV:   math.Round(cv*1000) / 1000,
	}
}

// cadenceCV returns the coefficient of variation of sentence length in words, or zero when
// there are too few sentences to judge a rhythm.
func cadenceCV(text string) float64 {
	var lengths []float64
	for _, sentence := range sentenceSplit.Split(text, -1) {
		if n := len(strings.Fields(sentence)); n > 0 {
			lengths = append(lengths, float64(n))
		}
	}
	if len(lengths) < 3 {
		return 0
	}
	var sum float64
	for _, n := range lengths {
		sum += n
	}
	mean := sum / float64(len(lengths))
	if mean == 0 {
		return 0
	}
	var variance float64
	for _, n := range lengths {
		variance += (n - mean) * (n - mean)
	}
	variance /= float64(len(lengths))
	return math.Sqrt(variance) / mean
}
