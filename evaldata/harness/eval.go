package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"

	"github.com/dcadolph/slop-chop/sanitize"
)

// Sample is one evaluation text with its ground truth and provenance.
type Sample struct {
	// ID is the sample's stable id, prefixed a for machine samples and h for human.
	ID string `json:"id"`
	// Source is the ground truth label, ai or human. Raters never see it.
	Source string `json:"source"`
	// Rules names the release tag whose rules were frozen before collection.
	Rules string `json:"rules"`
	// Meta holds provenance: model and prompt for ai, origin and genre for human.
	Meta map[string]string `json:"meta"`
	// Text is the sample itself.
	Text string `json:"text"`
}

// Rating is one rater's blind answer for one sample.
type Rating struct {
	// Sample is the id of the rated sample.
	Sample string `json:"sample"`
	// Rater is the rater's opaque id.
	Rater string `json:"rater"`
	// Machine is the 1 to 7 answer: how machine-written the text reads.
	Machine int `json:"machine"`
}

// readLines parses a JSONL file into out, one object per non-blank line. A missing file
// is an empty corpus, not an error, so the scaffolding works before any data exists.
func readLines[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

// normalize collapses whitespace and case so the disjointness check catches a passage
// that was reflowed or re-cased on its way between corpora.
func normalize(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// checkCorpus validates the eval samples and enforces the lock against the development
// corpus. It returns every violation rather than stopping at the first.
func checkCorpus(samples []Sample, devTexts []string) []string {
	var problems []string
	dev := make(map[string]bool, len(devTexts))
	for _, t := range devTexts {
		dev[normalize(t)] = true
	}
	seen := map[string]bool{}
	for i, s := range samples {
		at := fmt.Sprintf("sample %d (%s)", i+1, s.ID)
		switch {
		case s.ID == "":
			problems = append(problems, fmt.Sprintf("sample %d has no id", i+1))
		case seen[s.ID]:
			problems = append(problems, at+": duplicate id")
		}
		seen[s.ID] = true
		if s.Source != "ai" && s.Source != "human" {
			problems = append(problems, at+`: source must be "ai" or "human"`)
		}
		if s.Rules == "" {
			problems = append(problems, at+": no rules tag; samples are collected against a frozen release")
		}
		words := len(strings.Fields(s.Text))
		if words < 60 || words > 400 {
			problems = append(problems, fmt.Sprintf("%s: %d words, want 60 to 400", at, words))
		}
		if dev[normalize(s.Text)] {
			problems = append(problems, at+": text appears in the development corpus, which breaks the lock")
		}
		if leak := markupLeak(s.Text); leak != "" {
			problems = append(problems, at+": carries "+leak+", which tells a rater the label before they read it")
		}
	}
	return problems
}

// markupLeak names the first trace of markup or layout in a sample, or returns empty. The
// human half is collected as one flat line of prose, so any of these marks a sample as
// machine-written on sight, and the rating would measure the formatting instead.
func markupLeak(text string) string {
	switch {
	case strings.Contains(text, "\n"):
		return "a paragraph break"
	case strings.Contains(text, "`"):
		return "a code span"
	case strings.Contains(text, "**") || strings.Contains(text, "__"):
		return "emphasis markup"
	case strings.Contains(text, "]("):
		return "link syntax"
	case strings.ContainsAny(text, "|<>"):
		return "table or HTML markup"
	case strings.Contains(text, "  "):
		return "a doubled space"
	}
	return ""
}

// checkRatings validates the ratings against the samples.
func checkRatings(ratings []Rating, samples []Sample) []string {
	ids := make(map[string]bool, len(samples))
	for _, s := range samples {
		ids[s.ID] = true
	}
	var problems []string
	seen := map[[2]string]bool{}
	for i, r := range ratings {
		at := fmt.Sprintf("rating %d (%s by %s)", i+1, r.Sample, r.Rater)
		key := [2]string{r.Rater, r.Sample}
		if seen[key] {
			problems = append(problems, at+": the rater already answered this sample, so it would count twice")
		}
		seen[key] = true
		if !ids[r.Sample] {
			problems = append(problems, at+": names a sample that does not exist")
		}
		if r.Rater == "" {
			problems = append(problems, at+": no rater id")
		}
		if r.Machine < 1 || r.Machine > 7 {
			problems = append(problems, fmt.Sprintf("%s: machine is %d, want 1 to 7", at, r.Machine))
		}
	}
	return problems
}

// ranks returns the average ranks of the values, so tied values share a rank and the
// correlation below is the tie-corrected Spearman.
func ranks(values []float64) []float64 {
	idx := make([]int, len(values))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return values[idx[a]] < values[idx[b]] })
	out := make([]float64, len(values))
	for i := 0; i < len(idx); {
		j := i
		for j < len(idx) && values[idx[j]] == values[idx[i]] {
			j++
		}
		avg := float64(i+j+1) / 2
		for k := i; k < j; k++ {
			out[idx[k]] = avg
		}
		i = j
	}
	return out
}

// spearman returns the rank correlation between the two lists, or NaN when there is too
// little to correlate.
func spearman(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 3 {
		return math.NaN()
	}
	return pearson(ranks(a), ranks(b))
}

// pearson returns the linear correlation between the two lists, or NaN when either has
// no variance.
func pearson(a, b []float64) float64 {
	n := float64(len(a))
	var sa, sb float64
	for i := range a {
		sa += a[i]
		sb += b[i]
	}
	ma, mb := sa/n, sb/n
	var cov, va, vb float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	if va == 0 || vb == 0 {
		return math.NaN()
	}
	return cov / math.Sqrt(va*vb)
}

// separation returns the probability that a random ai sample outscores a random human
// one, with ties counted half. 0.5 is chance and 1.0 is perfect separation. It returns
// NaN when either side is empty.
func separation(aiScores, humanScores []float64) float64 {
	if len(aiScores) == 0 || len(humanScores) == 0 {
		return math.NaN()
	}
	wins := 0.0
	for _, a := range aiScores {
		for _, h := range humanScores {
			switch {
			case a > h:
				wins++
			case a == h:
				wins += 0.5
			}
		}
	}
	return wins / float64(len(aiScores)*len(humanScores))
}

// meanRatings returns each sample's mean rating and rater count, keyed by sample id.
func meanRatings(ratings []Rating) map[string]struct {
	Mean  float64
	Count int
} {
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, r := range ratings {
		sums[r.Sample] += float64(r.Machine)
		counts[r.Sample]++
	}
	out := make(map[string]struct {
		Mean  float64
		Count int
	}, len(sums))
	for id, sum := range sums {
		out[id] = struct {
			Mean  float64
			Count int
		}{sum / float64(counts[id]), counts[id]}
	}
	return out
}

// raterConsistency returns the mean pairwise Spearman correlation between raters over
// the samples each pair shares, skipping pairs with fewer than five shared samples. It
// returns NaN when no pair qualifies.
func raterConsistency(ratings []Rating) float64 {
	byRater := map[string]map[string]float64{}
	for _, r := range ratings {
		if byRater[r.Rater] == nil {
			byRater[r.Rater] = map[string]float64{}
		}
		byRater[r.Rater][r.Sample] = float64(r.Machine)
	}
	var raters []string
	for id := range byRater {
		raters = append(raters, id)
	}
	sort.Strings(raters)
	var sum float64
	pairs := 0
	for i := range raters {
		for j := i + 1; j < len(raters); j++ {
			var a, b []float64
			for sample, va := range byRater[raters[i]] {
				if vb, ok := byRater[raters[j]][sample]; ok {
					a = append(a, va)
					b = append(b, vb)
				}
			}
			if len(a) < 5 {
				continue
			}
			if rho := spearman(a, b); !math.IsNaN(rho) {
				sum += rho
				pairs++
			}
		}
	}
	if pairs == 0 {
		return math.NaN()
	}
	return sum / float64(pairs)
}

// scored pairs one sample with its slop score and its mean human rating.
type scored struct {
	// Sample is the sample being reported.
	Sample Sample
	// Score is the slop score of the text under the default profile.
	Score float64
	// Parts is the full score, so each component can be tested against the ratings.
	Parts sanitize.Score
	// Rating is the mean human rating, 1 to 7.
	Rating float64
	// Raters is how many raters answered.
	Raters int
}

// scoreSamples scores every rated sample with the default profile.
func scoreSamples(s *sanitize.Sanitizer, samples []Sample, ratings []Rating) []scored {
	means := meanRatings(ratings)
	var out []scored
	for _, sample := range samples {
		m, ok := means[sample.ID]
		if !ok {
			continue
		}
		parts := s.Score(sample.Text)
		out = append(out, scored{
			Sample: sample,
			Score:  float64(parts.Value),
			Parts:  parts,
			Rating: m.Mean,
			Raters: m.Count,
		})
	}
	return out
}

// component names one part of the score whose relation to the ratings is reported.
type component struct {
	// Name is the label printed in the report.
	Name string
	// Of reads the component's points from a score.
	Of func(sanitize.Score) int
}

// components are the score's parts in the order the report lists them. They are fixed
// here, before any scoring, so the list cannot be chosen after seeing which ones look good.
//
//nolint:gochecknoglobals // Immutable lookup.
var components = []component{
	{"density", func(s sanitize.Score) int { return s.Density }},
	{"hedging", func(s sanitize.Score) int { return s.Hedging }},
	{"cadence", func(s sanitize.Score) int { return s.Cadence }},
	{"evidence", func(s sanitize.Score) int { return s.Evidence }},
	{"drumbeat", func(s sanitize.Score) int { return s.Drumbeat }},
	{"tails", func(s sanitize.Score) int { return s.Tails }},
	{"clauses", func(s sanitize.Score) int { return s.Clauses }},
}

// minRaters is the fewest raters a sample needs before the corpus may be scored. The
// scored report runs once over the whole corpus, so a sample short of this blocks it
// rather than being reported thin.
const minRaters = 3

// The bootstrap behind the confidence interval on the headline correlation. The seed is
// fixed so the interval is reproducible from the published ratings.
const (
	bootstrapRounds = 2000
	bootstrapSeed   = 20261004
)

// corpusNote states which corpus a report covers and which ruleset its samples were pinned
// to. A caveat that lives only in a document is a caveat somebody quotes a number without,
// so it rides on the number instead. The corpus burned on 2026-09-21 lives in its own file
// and is never read here, so the note says that rather than calling these samples burned.
func corpusNote(samples []Sample) string {
	tags := map[string]bool{}
	for _, s := range samples {
		tags[s.Rules] = true
	}
	var pinned []string
	for t := range tags {
		pinned = append(pinned, t)
	}
	sort.Strings(pinned)
	return fmt.Sprintf("corpus: %d samples pinned to ruleset %s. The corpus burned on 2026-09-21\n"+
		"is not read here. See evaldata/README.md and evaldata/ANALYSIS.md.\n\n",
		len(samples), strings.Join(pinned, ", "))
}

// coverage is how far rating has progressed, read from the ratings alone.
type coverage struct {
	// Raters is how many distinct raters have answered.
	Raters int
	// Ratings is how many answers there are in total.
	Ratings int
	// Rated is how many samples have at least one answer.
	Rated int
	// Ready is how many samples have at least minRaters answers.
	Ready int
	// Total is how many samples the corpus holds.
	Total int
}

// coverageOf counts rating progress without reading a single score.
func coverageOf(samples []Sample, ratings []Rating) coverage {
	means := meanRatings(ratings)
	raters := map[string]bool{}
	for _, r := range ratings {
		raters[r.Rater] = true
	}
	c := coverage{Raters: len(raters), Ratings: len(ratings), Total: len(samples)}
	for _, s := range samples {
		m, ok := means[s.ID]
		if !ok {
			continue
		}
		c.Rated++
		if m.Count >= minRaters {
			c.Ready++
		}
	}
	return c
}

// coverageReport writes what can be said before the corpus is scored: how far rating has
// come and whether the raters agree with each other. It never loads the engine, which is
// what lets a pilot measure rater agreement without spending the corpus.
func coverageReport(w *strings.Builder, samples []Sample, ratings []Rating) {
	w.WriteString(corpusNote(samples))
	c := coverageOf(samples, ratings)
	fmt.Fprintf(w, "raters: %d, ratings: %d\n", c.Raters, c.Ratings)
	fmt.Fprintf(w, "samples rated at least once: %d of %d\n", c.Rated, c.Total)
	fmt.Fprintf(w, "samples with %d or more raters: %d of %d\n", minRaters, c.Ready, c.Total)
	fmt.Fprintf(w, "rater agreement (mean pairwise Spearman): %s\n", num(raterConsistency(ratings)))
	w.WriteString("\nNo score has been read. Scoring is one run over the whole corpus, made with\n")
	w.WriteString("-score once every sample has enough raters. See evaldata/ANALYSIS.md.\n")
}

// bootstrapSpearman returns a 95 percent percentile interval for the Spearman correlation
// between a and b, resampling pairs with replacement. Resamples with no variance are
// skipped. It returns NaN bounds when there is too little data to resample.
func bootstrapSpearman(a, b []float64, rounds int, seed uint64) (lo, hi float64) {
	n := len(a)
	if n != len(b) || n < 3 {
		return math.NaN(), math.NaN()
	}
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // Reproducible resampling, not security.
	rhos := make([]float64, 0, rounds)
	ra, rb := make([]float64, n), make([]float64, n)
	for range rounds {
		for i := range n {
			k := rng.IntN(n)
			ra[i], rb[i] = a[k], b[k]
		}
		if rho := spearman(ra, rb); !math.IsNaN(rho) {
			rhos = append(rhos, rho)
		}
	}
	if len(rhos) == 0 {
		return math.NaN(), math.NaN()
	}
	sort.Float64s(rhos)
	return rhos[int(0.025*float64(len(rhos)))], rhos[int(0.975*float64(len(rhos)-1))]
}

// pilotSubset returns n samples for a pilot, half machine and half human, chosen by a
// hash of each id so the choice is fixed, reproducible, and blind to every score. Every
// pilot rater gets the same subset, since agreement can only be measured on samples
// raters share. When a half is short, the other half fills the gap.
func pilotSubset(samples []Sample, n int) []Sample {
	if n <= 0 || n >= len(samples) {
		return samples
	}
	var ai, human []Sample
	for _, s := range samples {
		if s.Source == "ai" {
			ai = append(ai, s)
		} else {
			human = append(human, s)
		}
	}
	byHash := func(list []Sample) {
		sort.Slice(list, func(i, j int) bool {
			return sheetKey("pilot:"+list[i].ID) < sheetKey("pilot:"+list[j].ID)
		})
	}
	byHash(ai)
	byHash(human)
	takeAI := min(n/2, len(ai))
	takeHuman := min(n-takeAI, len(human))
	takeAI = min(n-takeHuman, len(ai))
	out := append(append([]Sample{}, ai[:takeAI]...), human[:takeHuman]...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// report writes the full analysis. The disagreement lists are the finding whatever the
// headline number says, so they print either way.
func report(w *strings.Builder, samples []Sample, rows []scored, ratings []Rating) {
	var scores, rats, ai, human []float64
	var aiScores, aiRats, humanScores, humanRats []float64
	underRated := 0
	for _, r := range rows {
		scores = append(scores, r.Score)
		rats = append(rats, r.Rating)
		if r.Sample.Source == "ai" {
			ai = append(ai, r.Score)
			aiScores, aiRats = append(aiScores, r.Score), append(aiRats, r.Rating)
		} else {
			human = append(human, r.Score)
			humanScores, humanRats = append(humanScores, r.Score), append(humanRats, r.Rating)
		}
		if r.Raters < minRaters {
			underRated++
		}
	}
	w.WriteString(corpusNote(samples))
	fmt.Fprintf(w, "rated samples: %d (%d ai, %d human)\n", len(rows), len(ai), len(human))
	if underRated > 0 {
		fmt.Fprintf(w, "warning: %d sample(s) have fewer than %d raters\n", underRated, minRaters)
	}
	lo, hi := bootstrapSpearman(scores, rats, bootstrapRounds, bootstrapSeed)
	fmt.Fprintf(w, "score vs human rating (Spearman): %s, 95%% interval %s to %s\n",
		num(spearman(scores, rats)), num(lo), num(hi))
	// Within each label the true answer is held constant, so a correlation here is the
	// score tracking perception rather than the score telling two populations apart.
	fmt.Fprintf(w, "  within machine samples:          %s\n", num(spearman(aiScores, aiRats)))
	fmt.Fprintf(w, "  within human samples:            %s\n", num(spearman(humanScores, humanRats)))
	fmt.Fprintf(w, "ai/human separation by score:     %s (0.5 chance, 1.0 perfect)\n",
		num(separation(ai, human)))
	fmt.Fprintf(w, "rater consistency (mean pairwise): %s\n", num(raterConsistency(ratings)))

	w.WriteString("\nscore components vs human rating (Spearman, exploratory):\n")
	for _, c := range components {
		var vals []float64
		for _, r := range rows {
			vals = append(vals, float64(c.Of(r.Parts)))
		}
		fmt.Fprintf(w, "  %-9s %s\n", c.Name, num(spearman(vals, rats)))
	}

	w.WriteString("\nscore says slop, people read it as human:\n")
	printDisagreements(w, rows, func(r scored) bool { return r.Score >= 55 && r.Rating <= 3 })
	w.WriteString("\npeople read it as machine, score waves it through:\n")
	printDisagreements(w, rows, func(r scored) bool { return r.Score < 25 && r.Rating >= 5 })
}

// printDisagreements lists the rows the predicate selects, worst score-to-rating gap
// first, or says there are none.
func printDisagreements(w *strings.Builder, rows []scored, pick func(scored) bool) {
	var hits []scored
	for _, r := range rows {
		if pick(r) {
			hits = append(hits, r)
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		gi := math.Abs(hits[i].Score/100*6 + 1 - hits[i].Rating)
		gj := math.Abs(hits[j].Score/100*6 + 1 - hits[j].Rating)
		return gi > gj
	})
	if len(hits) == 0 {
		w.WriteString("  none\n")
		return
	}
	for _, r := range hits {
		fmt.Fprintf(w, "  %-6s %-6s score %3.0f  rating %.1f  %s\n",
			r.Sample.ID, r.Sample.Source, r.Score, r.Rating, clip(r.Sample.Text, 60))
	}
}

// num formats a statistic, naming the not-enough-data case instead of printing NaN.
func num(v float64) string {
	if math.IsNaN(v) {
		return "n/a (not enough data)"
	}
	return fmt.Sprintf("%.3f", v)
}

// clip shortens s to at most n runes for one-line report rows.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
