package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
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
	}
	return problems
}

// checkRatings validates the ratings against the samples.
func checkRatings(ratings []Rating, samples []Sample) []string {
	ids := make(map[string]bool, len(samples))
	for _, s := range samples {
		ids[s.ID] = true
	}
	var problems []string
	for i, r := range ratings {
		at := fmt.Sprintf("rating %d (%s by %s)", i+1, r.Sample, r.Rater)
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
		out = append(out, scored{
			Sample: sample,
			Score:  float64(s.Score(sample.Text).Value),
			Rating: m.Mean,
			Raters: m.Count,
		})
	}
	return out
}

// report writes the full analysis. The disagreement lists are the finding whatever the
// headline number says, so they print either way.
func report(w *strings.Builder, rows []scored, ratings []Rating) {
	var scores, rats, ai, human []float64
	underRated := 0
	for _, r := range rows {
		scores = append(scores, r.Score)
		rats = append(rats, r.Rating)
		if r.Sample.Source == "ai" {
			ai = append(ai, r.Score)
		} else {
			human = append(human, r.Score)
		}
		if r.Raters < 3 {
			underRated++
		}
	}
	fmt.Fprintf(w, "rated samples: %d (%d ai, %d human)\n", len(rows), len(ai), len(human))
	if underRated > 0 {
		fmt.Fprintf(w, "warning: %d sample(s) have fewer than 3 raters\n", underRated)
	}
	fmt.Fprintf(w, "score vs human rating (Spearman): %s\n", num(spearman(scores, rats)))
	fmt.Fprintf(w, "ai/human separation by score:     %s (0.5 chance, 1.0 perfect)\n",
		num(separation(ai, human)))
	fmt.Fprintf(w, "rater consistency (mean pairwise): %s\n", num(raterConsistency(ratings)))

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
