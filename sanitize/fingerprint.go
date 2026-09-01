package sanitize

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// A fingerprint measures how a writer writes rather than what they write: sentence
// rhythm, punctuation habits, and register, as numbers. The keep, prefer, and avoid lists
// hold your words, which a model can echo back at you the moment it reads one. These
// numbers it cannot echo, because nobody quotes a comma rate. Measuring text against them
// says where a draft stops sounding like you.
//
// Drift is not slop. A drifted paragraph can be good writing that belongs to somebody
// else, which is exactly what a model hands you, so the report names the difference and
// leaves the verdict to the writer.

// fingerprintMinWords and fingerprintMinSentences are the least text that can carry a
// fingerprint. Under it the rates swing on single sentences and the numbers describe the
// sample rather than the writer.
const (
	fingerprintMinWords     = 300
	fingerprintMinSentences = 15
)

// compareMinWords and compareMinSentences are the least text worth comparing against a
// fingerprint. A one-line note is not a style sample.
const (
	compareMinWords     = 60
	compareMinSentences = 4
)

// spreadMinWords is the smallest sample that gets a vote in a metric's observed spread.
// A short file's rates are noise, and letting them widen the bands would excuse any
// drift.
const spreadMinWords = 150

// ErrSample means there was not enough writing to measure a voice from.
var ErrSample = fmt.Errorf("not enough text")

// Fingerprint is the measured shape of a writer's prose: one value per trait, each with
// the band it may move inside before the difference means anything.
type Fingerprint struct {
	// Samples is how many separate texts were measured.
	Samples int `json:"samples"`
	// Words is the total word count behind the numbers, code excluded.
	Words int `json:"words"`
	// Sentences is the total sentence count behind the numbers.
	Sentences int `json:"sentences"`
	// Metrics holds each trait by name. Use [MetricList] for report order.
	Metrics map[string]Metric `json:"metrics"`
}

// Metric is one measured trait of a writer's prose.
type Metric struct {
	// Value is the measured rate, in the unit named by the metric's description.
	Value float64 `json:"value"`
	// Band is how far a text may sit from Value before it reads as drift. It starts at
	// a floor wide enough to absorb ordinary variation and widens to the spread the
	// writer's own samples showed.
	Band float64 `json:"band"`
}

// Drift is one trait of a text that sits outside the fingerprint's band for it.
type Drift struct {
	// Metric is the trait's name.
	Metric string `json:"metric"`
	// Unit describes what the numbers count, for example "words per sentence".
	Unit string `json:"unit"`
	// Note names what the text did, such as "longer sentences". It carries no pronoun, so a
	// caller can address the writer or speak about them without rewriting it.
	Note string `json:"note"`
	// Want is the fingerprint's value for the trait.
	Want float64 `json:"want"`
	// Got is the measured value for the text.
	Got float64 `json:"got"`
	// Band is the band the value had to stay inside.
	Band float64 `json:"band"`
	// Off is how many bands away the text landed, so a caller can rank the report.
	Off float64 `json:"off"`
}

// metricDef defines one measurable trait: how to read it off a counted sample, how wide
// its band starts, and how to say which way a text moved.
type metricDef struct {
	// name is the trait's key in a fingerprint's metric map.
	name string
	// unit describes what the number counts.
	unit string
	// floor is the narrowest band the trait can have.
	floor float64
	// high and low name what a value above or below the band means, in plain words and
	// without a pronoun.
	high, low string
	// read returns the trait's value for a counted sample.
	read func(c counts) float64
}

// metricDefs are the measured traits, in report order. The floors are set so that two
// things one person wrote sit inside each other's bands while a model's default register,
// which runs longer, flatter, and more formal than most people write, sits outside.
//
//nolint:gochecknoglobals // Immutable table.
var metricDefs = []metricDef{{
	name: "sentence-length", unit: "words per sentence", floor: 4,
	high: "longer sentences", low: "shorter sentences",
	read: func(c counts) float64 { return ratio(c.words, c.sentences) },
}, {
	name: "sentence-variation", unit: "spread of sentence length", floor: 0.15,
	high: "more varied sentence lengths", low: "a flatter sentence rhythm",
	read: func(c counts) float64 { return variation(c.lengths) },
}, {
	name: "paragraph-length", unit: "sentences per paragraph", floor: 1.2,
	high: "longer paragraphs", low: "shorter paragraphs",
	read: func(c counts) float64 { return ratio(c.sentences, c.paragraphs) },
}, {
	name: "commas", unit: "commas per sentence", floor: 0.5,
	high: "more clauses per sentence", low: "fewer clauses per sentence",
	read: func(c counts) float64 { return ratio(c.commas, c.sentences) },
}, {
	name: "semicolons", unit: "semicolons per 100 sentences", floor: 8,
	high: "more semicolons", low: "fewer semicolons",
	read: func(c counts) float64 { return per100(c.semicolons, c.sentences) },
}, {
	name: "dashes", unit: "dashes per 100 sentences", floor: 10,
	high: "more dashes", low: "fewer dashes",
	read: func(c counts) float64 { return per100(c.dashes, c.sentences) },
}, {
	name: "questions", unit: "percent of sentences", floor: 8,
	high: "more questions asked", low: "fewer questions asked",
	read: func(c counts) float64 { return per100(c.questions, c.sentences) },
}, {
	name: "contractions", unit: "contractions per 100 words", floor: 1.5,
	high: "a more contracted register", low: "a more formal register",
	read: func(c counts) float64 { return per100(c.contractions, c.words) },
}, {
	name: "first-person", unit: "first-person words per 100 words", floor: 2,
	high: "more first-person", low: "less first-person",
	read: func(c counts) float64 { return per100(c.firstPerson, c.words) },
}, {
	name: "long-words", unit: "percent of words over eight letters", floor: 4,
	high: "a heavier vocabulary", low: "a plainer vocabulary",
	read: func(c counts) float64 { return per100(c.longWords, c.words) },
}, {
	name: "lowercase-starts", unit: "percent of sentences", floor: 10,
	high: "more lowercase sentence openings", low: "fewer lowercase sentence openings",
	read: func(c counts) float64 { return per100(c.lowerStarts, c.sentences) },
}}

// MetricInfo names one measurable trait and says what its numbers count, so a caller can
// print a fingerprint without knowing the table behind it.
type MetricInfo struct {
	// Name is the trait's key in a fingerprint's metric map.
	Name string `json:"name"`
	// Unit describes what the number counts, for example "words per sentence".
	Unit string `json:"unit"`
}

// MetricList returns the measurable traits in report order.
func MetricList() []MetricInfo {
	out := make([]MetricInfo, 0, len(metricDefs))
	for _, d := range metricDefs {
		out = append(out, MetricInfo{Name: d.name, Unit: d.unit})
	}
	return out
}

// counts holds the raw tallies one or more samples contributed, which the metrics are
// read off. Pooling counts rather than averaging rates keeps a long sample from carrying
// the same weight as a paragraph.
type counts struct {
	// words, sentences, and paragraphs are the structural tallies.
	words, sentences, paragraphs int
	// lengths holds every sentence's word count, for the variation metric.
	lengths []int
	// commas, semicolons, and dashes are punctuation tallies.
	commas, semicolons, dashes int
	// questions is the number of sentences that ask something.
	questions int
	// contractions, firstPerson, and longWords are register tallies.
	contractions, firstPerson, longWords int
	// lowerStarts is the number of sentences that open with a lowercase letter.
	lowerStarts int
}

// add folds another sample's tallies into c.
func (c *counts) add(o counts) {
	c.words += o.words
	c.sentences += o.sentences
	c.paragraphs += o.paragraphs
	c.lengths = append(c.lengths, o.lengths...)
	c.commas += o.commas
	c.semicolons += o.semicolons
	c.dashes += o.dashes
	c.questions += o.questions
	c.contractions += o.contractions
	c.firstPerson += o.firstPerson
	c.longWords += o.longWords
	c.lowerStarts += o.lowerStarts
}

// NewFingerprint measures the samples and returns the writer's fingerprint. Every sample
// is pooled into one set of numbers, and each sample long enough to have a stable rate
// also widens the bands to the spread the writer actually showed, so a habit that varies
// from piece to piece is not read as drift later. It returns [ErrSample] when the samples
// together hold too little writing to measure.
func NewFingerprint(samples ...string) (Fingerprint, error) {
	var pooled counts
	var per []counts
	for _, s := range samples {
		c := measure(s)
		if c.words == 0 {
			continue
		}
		pooled.add(c)
		per = append(per, c)
	}
	if pooled.words < fingerprintMinWords || pooled.sentences < fingerprintMinSentences {
		return Fingerprint{}, fmt.Errorf("%w: %s in %s, want %s in %s", ErrSample,
			plural(pooled.words, "word"), plural(pooled.sentences, "sentence"),
			plural(fingerprintMinWords, "word"), plural(fingerprintMinSentences, "sentence"))
	}
	f := Fingerprint{
		Samples:   len(per),
		Words:     pooled.words,
		Sentences: pooled.sentences,
		Metrics:   make(map[string]Metric, len(metricDefs)),
	}
	for _, d := range metricDefs {
		f.Metrics[d.name] = Metric{
			Value: round(d.read(pooled)),
			Band:  round(math.Max(d.floor, spread(d, per))),
		}
	}
	return f, nil
}

// spread returns half the distance between the highest and lowest reading of the metric
// across the samples big enough to have a stable one, or zero when fewer than two do.
func spread(d metricDef, per []counts) float64 {
	low, high := math.Inf(1), math.Inf(-1)
	n := 0
	for _, c := range per {
		if c.words < spreadMinWords || c.sentences == 0 {
			continue
		}
		v := d.read(c)
		low = math.Min(low, v)
		high = math.Max(high, v)
		n++
	}
	if n < 2 {
		return 0
	}
	return (high - low) / 2
}

// Empty reports whether the fingerprint holds no measurements, so callers can tell a
// voice file that was never fingerprinted from one that was.
func (f Fingerprint) Empty() bool {
	return len(f.Metrics) == 0
}

// Compare measures text against the fingerprint and returns every trait that landed
// outside its band, the furthest off first. An empty result means the text reads like the
// writer on every trait measured. It returns [ErrSample] when the text is too short to
// carry a style reading, and an error when the fingerprint itself is empty.
func (f Fingerprint) Compare(text string) ([]Drift, error) {
	if f.Empty() {
		return nil, fmt.Errorf("%w: no fingerprint to compare against", ErrSample)
	}
	c := measure(text)
	if c.words < compareMinWords || c.sentences < compareMinSentences {
		return nil, fmt.Errorf("%w: %s in %s, want %s in %s", ErrSample,
			plural(c.words, "word"), plural(c.sentences, "sentence"),
			plural(compareMinWords, "word"), plural(compareMinSentences, "sentence"))
	}
	var out []Drift
	for _, d := range metricDefs {
		m, ok := f.Metrics[d.name]
		if !ok || m.Band <= 0 {
			continue
		}
		got := d.read(c)
		off := math.Abs(got-m.Value) / m.Band
		if off <= 1 {
			continue
		}
		note := d.low
		if got > m.Value {
			note = d.high
		}
		out = append(out, Drift{
			Metric: d.name,
			Unit:   d.unit,
			Note:   note,
			Want:   m.Value,
			Got:    round(got),
			Band:   m.Band,
			Off:    round(off),
		})
	}
	// Sort by distance so the report opens with the trait that reads least like the
	// writer, and by name on a tie so the same text always reports the same order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Off != out[j].Off {
			return out[i].Off > out[j].Off
		}
		return out[i].Metric < out[j].Metric
	})
	return out, nil
}

// listMarker matches the bullet, number, or quote a line opens a list or quotation with.
// The marker is furniture rather than writing, and leaving it in would read as a word.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var listMarker = regexp.MustCompile(`^\s*(?:[-*+>]\s+|\d+[.)]\s+)`)

// proseOnly strips everything in text that is not a sentence somebody wrote: code, front
// matter, headings, table rows, horizontal rules, and list markers. What is left is the
// prose the numbers are supposed to describe. Structural lines become blank ones, which
// is what they already are to a reader: a break between paragraphs.
func proseOnly(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	front := false
	for i, line := range strings.Split(maskCode(text), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case i == 0 && trimmed == "---":
			front = true
		case front:
			if trimmed == "---" {
				front = false
			}
		case trimmed == "", strings.HasPrefix(trimmed, "#"), strings.HasPrefix(trimmed, "|"),
			ruleLine(trimmed):
		default:
			b.WriteString(listMarker.ReplaceAllString(line, ""))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// ruleLine reports whether a line is a horizontal rule or a setext heading underline,
// both of which are runs of one punctuation mark rather than writing.
func ruleLine(s string) bool {
	if len(s) < 3 {
		return false
	}
	return strings.Trim(s, "-") == "" || strings.Trim(s, "=") == "" || strings.Trim(s, "_") == ""
}

// measure counts one sample, reading only the prose in it, so a fenced block's
// punctuation or a table's pipes never reach the numbers that describe how somebody
// writes.
func measure(text string) counts {
	prose := proseOnly(text)
	c := counts{}
	for _, p := range paragraphSpans(prose) {
		if p.words == 0 {
			continue
		}
		c.paragraphs++
	}
	for _, sp := range sentenceSpans(prose) {
		body := prose[sp.start:sp.end]
		words := strings.Fields(body)
		if len(words) == 0 {
			continue
		}
		c.sentences++
		c.words += len(words)
		c.lengths = append(c.lengths, len(words))
		if strings.HasSuffix(strings.TrimRight(body, " \t\n\r\"'”’)"), "?") {
			c.questions++
		}
		if r := firstLetter(body); r != 0 && unicode.IsLower(r) {
			c.lowerStarts++
		}
		c.commas += strings.Count(body, ",")
		c.semicolons += strings.Count(body, ";")
		c.dashes += countDashes(body)
		for _, w := range words {
			c.contractions += contractionCount(w)
			lower := strings.ToLower(strings.Trim(w, ".,;:!?\"'()[]“”‘’"))
			if firstPersonWords[lower] {
				c.firstPerson++
			}
			if letterCount(lower) > 8 {
				c.longWords++
			}
		}
	}
	return c
}

// firstPersonWords are the words that point back at the writer. Their density separates
// a person talking from a report about a subject.
//
//nolint:gochecknoglobals // Immutable set.
var firstPersonWords = map[string]bool{
	"i": true, "me": true, "my": true, "mine": true, "myself": true,
	"we": true, "us": true, "our": true, "ours": true, "ourselves": true,
	"i'm": true, "i've": true, "i'll": true, "i'd": true,
	"we're": true, "we've": true, "we'll": true, "we'd": true,
}

// contractionSubjects are the words whose trailing "'s" is a contraction of is or has
// rather than a possessive. "it's" is one, "the writer's" is not, and telling them apart
// is the difference between measuring register and counting apostrophes.
//
//nolint:gochecknoglobals // Immutable set.
var contractionSubjects = map[string]bool{
	"it": true, "that": true, "there": true, "here": true, "what": true, "who": true,
	"he": true, "she": true, "let": true, "this": true, "where": true, "how": true,
	"everything": true, "something": true, "nothing": true, "everyone": true,
	"someone": true, "nobody": true,
}

// contractionSuffixes are the endings that mark a contraction outright, whatever word
// carries them.
//
//nolint:gochecknoglobals // Immutable set.
var contractionSuffixes = map[string]bool{
	"t": true, "re": true, "ve": true, "ll": true, "d": true, "m": true,
}

// contractionCount reports whether word is a contraction, as one or zero.
func contractionCount(word string) int {
	w := strings.ToLower(strings.Trim(word, ".,;:!?\"()[]“”"))
	w = strings.ReplaceAll(w, "’", "'")
	head, tail, ok := strings.Cut(w, "'")
	if !ok || head == "" || tail == "" {
		return 0
	}
	if contractionSuffixes[tail] {
		return 1
	}
	if tail == "s" && contractionSubjects[head] {
		return 1
	}
	return 0
}

// countDashes counts the dashes a writer chooses: the em-dash and en-dash wherever they
// appear, and the hyphen only when it is spaced out as a dash, so compound words are not
// mistaken for punctuation habits.
func countDashes(s string) int {
	n := strings.Count(s, "—") + strings.Count(s, "–")
	for i := 1; i+1 < len(s); i++ {
		if s[i] != '-' || s[i-1] != ' ' {
			continue
		}
		j := i
		for j < len(s) && s[j] == '-' {
			j++
		}
		if j < len(s) && s[j] == ' ' {
			n++
		}
		i = j
	}
	return n
}

// firstLetter returns the first letter in s, or zero when it holds none.
func firstLetter(s string) rune {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return r
		}
	}
	return 0
}

// letterCount returns how many letters s holds, so punctuation and digits do not inflate
// a word's length.
func letterCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// ratio returns n over d, or zero when d is zero.
func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// variation returns the coefficient of variation of the lengths, the spread of sentence
// length relative to its own average, or zero when there is too little to judge.
func variation(lengths []int) float64 {
	if len(lengths) < 3 {
		return 0
	}
	sum := 0.0
	for _, n := range lengths {
		sum += float64(n)
	}
	mean := sum / float64(len(lengths))
	if mean == 0 {
		return 0
	}
	v := 0.0
	for _, n := range lengths {
		v += (float64(n) - mean) * (float64(n) - mean)
	}
	return math.Sqrt(v/float64(len(lengths))) / mean
}

// plural returns n with word, adding an s unless n is one.
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// round trims a measurement to two decimals, so a fingerprint file stays readable and two
// runs over the same text compare equal.
func round(v float64) float64 {
	return math.Round(v*100) / 100
}
