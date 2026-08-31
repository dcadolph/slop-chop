package sanitize

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Score is a read on how much a text reads like AI wrote it, from 0 for clean to 100 for
// heavy slop. It sums named signals: the weighted density of rule tells and how
// hedge-heavy the register is. Typography normalization and house-style cleanups are
// reported as findings but carry no score weight, so a professionally typeset human page
// never reads as slop for its curly quotes. Repeats of one rule count with halving
// weight, so a poet's em-dashes or a writer's pet word read as a habit, while the same
// density spread across distinct tells reads as a machine.
type Score struct {
	// Value is the 0 to 100 score, the capped sum of the signals below.
	Value int `json:"value"`
	// Tells is the number of rule findings in the text.
	Tells int `json:"tells"`
	// Words is the word count the densities are measured against.
	Words int `json:"words"`
	// TellsPer100 is tells per hundred words, the density the score leans on.
	TellsPer100 float64 `json:"tellsPer100"`
	// CadenceCV is the coefficient of variation of sentence length, reported for
	// context but carrying no score weight: modern model prose varies its rhythm on
	// purpose, so a flat cadence marks old machine output while penalizing plain,
	// competent human writing. It is -1 when there are too few sentences to judge.
	CadenceCV float64 `json:"cadenceCv"`
	// Density is the points weighted tell density added to Value. A structural tell
	// counts double, since a stock sentence shape is stronger evidence than one word,
	// and typography swaps count nothing.
	Density int `json:"density"`
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

// tidyRuleNames are the cleanup rules whose findings are house style, not evidence of
// machine writing, so they carry no score weight.
//
//nolint:gochecknoglobals // Immutable set.
var tidyRuleNames = map[string]bool{
	"semicolon": true, "orphan-comma": true, "space-before-punct": true,
	"comma-before-stop": true, "comma-run": true, "double-space": true, "article": true,
}

// TidyRule reports whether name is a punctuation or spacing cleanup rule rather than an
// AI tell. Callers scanning code comments use it to drop findings that only make sense
// in prose the tool is allowed to rewrite.
func TidyRule(name string) bool {
	return tidyRuleNames[name]
}

// weightedChars are the character swaps that still count toward the score: the em-dash,
// the model tell itself, and the invisible characters that smuggle words past a word
// list. Every other character swap is typography normalization, evidence of typesetting
// rather than of a model, and counts nothing.
//
//nolint:gochecknoglobals // Immutable set.
var weightedChars = map[string]bool{
	"char:—":      true,
	"char:\u200b": true,
	"char:\u2060": true,
	"char:\ufeff": true,
	"char:\u00ad": true,
}

// Score rates text from 0 to 100 by weighted tell density and hedge density.
func (s *Sanitizer) Score(text string) Score {
	findings := s.Check(text)
	tells := len(findings)
	weighted := s.weightTells(findings)
	// Measure the densities against prose only. Code is blanked so a large fenced block
	// cannot dilute the word count the signals are weighed against.
	prose := maskCode(text)
	words := len(strings.Fields(prose))

	// Weighted tell density is the main signal. Each weighted tell per hundred words adds
	// eight points, capped so a dense page saturates near eighty.
	density := 0.0
	if words > 0 {
		density = math.Min(80, weighted/float64(words)*100*8)
	}

	// A hedge-heavy register, the noncommittal "may generally" voice, is a structural tell a
	// word list misses. Hedge density adds up to ten points.
	hedging := math.Min(10, per100(countHedges(prose), words)*2.5)

	value := int(math.Round(math.Min(100, density+hedging)))
	return Score{
		Value:       value,
		Tells:       tells,
		Words:       words,
		TellsPer100: math.Round(per100(tells, words)*100) / 100,
		CadenceCV:   cadenceReport(cadenceCV(prose)),
		Density:     int(math.Round(density)),
		Hedging:     int(math.Round(hedging)),
	}
}

// weightTells sums the score weight of every finding. Structural findings whose spans
// overlap count once, so two patterns firing on the same sentence do not double up. A
// rule firing again counts half of its previous occurrence: machine writing shows many
// different tells, so the twelfth em-dash in a poem or a writer's one pet buzzword
// repeated is weak evidence, while the same total from distinct tells keeps full weight.
func (s *Sanitizer) weightTells(findings []Finding) float64 {
	total := 0.0
	structStart, structEnd := -1, -1
	seen := make(map[string]float64, len(findings))
	for _, f := range findings {
		if strings.HasPrefix(f.Rule, "structural:") {
			start, end := f.Offset, f.Offset+len(f.Match)
			if structStart >= 0 && start < structEnd {
				if end > structEnd {
					structEnd = end
				}
				continue
			}
			structStart, structEnd = start, end
		}
		w := s.tellWeight(f.Rule)
		if prev, ok := seen[f.Rule]; ok {
			w = prev / 2
		}
		seen[f.Rule] = w
		total += w
	}
	return total
}

// tellWeight resolves the score weight for one finding by its rule name. An exact entry
// in the profile's scoreWeights wins, then the rule's class, then the built-in default:
// structural tells count double, since a stock sentence shape is stronger evidence than
// one word, typography swaps and house-style cleanups count nothing, and everything else
// counts one.
func (s *Sanitizer) tellWeight(rule string) float64 {
	if w, ok := s.weights[rule]; ok {
		return w
	}
	if class, ok := ruleClass(rule); ok {
		if w, ok := s.weights[class]; ok {
			return w
		}
	}
	return defaultTellWeight(rule)
}

// ruleClass returns the class a rule name belongs to for weight lookup: the part before
// the colon, or "tidy" for the colonless cleanup rules.
func ruleClass(rule string) (string, bool) {
	if class, _, ok := strings.Cut(rule, ":"); ok {
		return class, true
	}
	if tidyRuleNames[rule] {
		return "tidy", true
	}
	return "", false
}

// defaultTellWeight returns the built-in score weight for a rule name.
func defaultTellWeight(rule string) float64 {
	switch {
	case strings.HasPrefix(rule, "structural:"):
		return 2
	case strings.HasPrefix(rule, "char:"):
		if weightedChars[rule] {
			return 1
		}
		return 0
	case tidyRuleNames[rule]:
		return 0
	}
	return 1
}

// per100 returns n per hundred of d, or 0 when d is zero.
func per100(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

// countHedges counts hedge words in text, matched case-insensitively on word boundaries.
// An all-caps word is skipped: MAY and SHOULD in a spec are RFC 2119 normative keywords,
// the opposite of hedging.
func countHedges(text string) int {
	n := 0
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) > 1 && w == strings.ToUpper(w) {
			continue
		}
		if hedges[strings.ToLower(w)] {
			n++
		}
	}
	return n
}

// cadenceReport rounds a measured coefficient of variation for the score report, and passes
// the -1 too-few-sentences sentinel through unrounded, so a flat, measured 0 stays distinct
// from "not enough sentences to judge" instead of both surfacing as 0.
func cadenceReport(cv float64) float64 {
	if cv < 0 {
		return -1
	}
	return math.Round(cv*1000) / 1000
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
