package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/dcadolph/slop-chop/sanitize"
)

// The pre-2022 false-positive measurement answers a question the locked corpus cannot
// answer yet and the development corpus can never answer: does the engine fire on
// ordinary professional prose it has never seen? Ground truth here is temporal rather
// than human. A repository with no push after 2021 has a README written before a general
// writing model existed, so every sample is human by construction, no rater required.
// What the run reports is not a verdict on the engine but a list of which rules cost it
// precision on real writing, which is the only form of that evidence available today.

// pre2022MinWords is the least prose a README must carry to be scored. Below it the file
// is badges and code fences, where a score reads nothing and would only add noise.
const pre2022MinWords = 100

// pre2022MinASCIILetters is the share of letters that must be ASCII for a README to count
// as English prose. The rules are English, so scoring a Chinese or Japanese README
// measures the tokenizer rather than the writing.
const pre2022MinASCIILetters = 0.9

// pre2022Threshold is the score at or above which a sample counts as a false positive.
// It is the "reads clean" boundary the CLI and the web app both draw.
const pre2022Threshold = 25

// Readme is one collected README with the provenance that makes it re-fetchable.
type Readme struct {
	// Repo is the owner and name, like "boltdb/bolt".
	Repo string `json:"repo"`
	// SHA is the commit the README was read at, so the exact bytes can be fetched again.
	SHA string `json:"sha"`
	// PushedAt is the repository's last push, the date that carries the human guarantee.
	PushedAt string `json:"pushedAt"`
	// Stars is the star count at collection time, kept only to describe the sample.
	Stars int `json:"stars"`
	// Language is the ecosystem the sample was drawn from, so the spread across
	// ecosystems is a fact in the data rather than a claim about how it was collected.
	Language string `json:"language"`
	// Text is the README body.
	Text string `json:"text"`
}

// readmeResult is one scored README along with the rules that fired on it.
type readmeResult struct {
	// repo is the owner and name.
	repo string
	// score is the slop score the engine gave it.
	score int
	// words is the prose word count the score was measured against.
	words int
	// rules are the names of the tell rules that fired, one entry per finding.
	rules []string
	// language is the ecosystem the sample came from.
	language string
}

// ruleCost counts how much one rule costs the engine on human prose.
type ruleCost struct {
	// rule is the rule name.
	rule string
	// hits is the total number of findings the rule produced.
	hits int
	// samples is the number of distinct READMEs it fired on.
	samples int
}

// eligible reports whether a README carries enough English prose to be worth scoring,
// along with the prose word count the decision was made on.
func eligible(s *sanitize.Sanitizer, text string) (words int, ok bool) {
	score := s.Score(text)
	if score.Words < pre2022MinWords {
		return score.Words, false
	}
	return score.Words, asciiShare(text) >= pre2022MinASCIILetters
}

// asciiShare returns the share of letters in text that are ASCII, or 1 when text holds no
// letters at all.
func asciiShare(text string) float64 {
	var letters, ascii int
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if r < unicode.MaxASCII {
			ascii++
		}
	}
	if letters == 0 {
		return 1
	}
	return float64(ascii) / float64(letters)
}

// scoreReadmes scores every eligible README and returns the results in input order. A
// README that fails the prose or language filter is skipped rather than scored, and the
// count of those skips comes back so the report can say how many were set aside.
func scoreReadmes(s *sanitize.Sanitizer, readmes []Readme) (results []readmeResult, skipped int) {
	for _, r := range readmes {
		words, ok := eligible(s, r.Text)
		if !ok {
			skipped++
			continue
		}
		var rules []string
		for _, f := range s.Check(r.Text) {
			if sanitize.TidyRule(f.Rule) {
				continue
			}
			rules = append(rules, f.Rule)
		}
		results = append(results, readmeResult{
			repo:     r.Repo,
			score:    s.Score(r.Text).Value,
			words:    words,
			rules:    rules,
			language: r.Language,
		})
	}
	return results, skipped
}

// costliestRules ranks the rules by how many distinct human READMEs they fired on. This
// is the actionable half of the run: a rule near the top of this list is either a real
// tell that human writers use anyway or a rule that needs a guard, and either way it is
// the rule to look at first.
func costliestRules(results []readmeResult) []ruleCost {
	hits := map[string]int{}
	samples := map[string]int{}
	for _, r := range results {
		seen := map[string]bool{}
		for _, rule := range r.rules {
			hits[rule]++
			if !seen[rule] {
				samples[rule]++
				seen[rule] = true
			}
		}
	}
	out := make([]ruleCost, 0, len(hits))
	for rule, n := range hits {
		out = append(out, ruleCost{rule: rule, hits: n, samples: samples[rule]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].samples != out[j].samples {
			return out[i].samples > out[j].samples
		}
		return out[i].rule < out[j].rule
	})
	return out
}

// percentile returns the value at the given percentile of a sorted-ascending slice, or 0
// when it is empty. It picks the nearest rank rather than interpolating, so every number
// it reports is a score some real sample actually got.
func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	i := int(p / 100 * float64(len(sorted)))
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

// falsePositives returns the results scoring at or above the threshold, worst first.
func falsePositives(results []readmeResult) []readmeResult {
	var out []readmeResult
	for _, r := range results {
		if r.score >= pre2022Threshold {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// displayRule renders a rule name for the report. A character rule names the character it
// matches, and the invisible ones print as nothing at all, so a zero-width space and a
// non-breaking space arrive as the same blank line. Those are spelled out instead.
func displayRule(rule string) string {
	char, ok := strings.CutPrefix(rule, "char:")
	if !ok {
		return rule
	}
	var b strings.Builder
	b.WriteString("char:")
	for _, r := range char {
		if unicode.IsGraphic(r) && !unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "U+%04X", r)
	}
	return b.String()
}

// pre2022TopRules is how many rules the report lists. Past it the tail is rules that
// fired on one or two samples, which says nothing.
const pre2022TopRules = 15

// provenanceOf describes how the sample was drawn, read from the samples rather than
// assumed, so a report never claims a guarantee the data behind it does not carry.
func provenanceOf(readmes []Readme) string {
	pinned := 0
	for _, r := range readmes {
		if strings.HasPrefix(r.PushedAt, "pinned") {
			pinned++
		}
	}
	switch {
	case len(readmes) == 0:
		return "No samples."
	case pinned == len(readmes):
		return "Every sample is a README read at the last commit a maintained repository made\nbefore 2022, so the projects are not selected for being abandoned."
	case pinned == 0:
		return "Every sample is a README from a repository with no push after 2021, which means\nthe projects are all abandoned ones."
	default:
		return fmt.Sprintf("Mixed sample: %d read at a pre-2022 commit, %d from repositories with no push\nafter 2021.", pinned, len(readmes)-pinned)
	}
}

// languageSpread summarizes how many of the scored samples came from each ecosystem,
// most first, so a claim about breadth can be read off the report instead of taken on
// trust. It counts what was scored rather than what was collected, since the skipped
// samples contributed nothing to the result.
func languageSpread(results []readmeResult) string {
	counts := map[string]int{}
	for _, r := range results {
		if r.language != "" {
			counts[r.language]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	type pair struct {
		lang string
		n    int
	}
	pairs := make([]pair, 0, len(counts))
	for lang, n := range counts {
		pairs = append(pairs, pair{lang, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].n != pairs[j].n {
			return pairs[i].n > pairs[j].n
		}
		return pairs[i].lang < pairs[j].lang
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%s %d", p.lang, p.n))
	}
	return strings.Join(parts, ", ")
}

// Manifest is one scored sample without its text: enough to re-fetch the exact bytes and
// check the number, and nothing that redistributes somebody else's README.
type Manifest struct {
	// Repo is the owner and name.
	Repo string `json:"repo"`
	// SHA is the commit the README was read at.
	SHA string `json:"sha"`
	// Language is the ecosystem.
	Language string `json:"language"`
	// Words is the prose word count the score was measured against.
	Words int `json:"words"`
	// Score is the slop score.
	Score int `json:"score"`
	// Rules are the tell rules that fired, deduplicated and sorted.
	Rules []string `json:"rules,omitempty"`
}

// manifestOf pairs each scored result with the sample it came from, so a published number
// can be checked against the exact commits behind it.
func manifestOf(readmes []Readme, results []readmeResult) []Manifest {
	shas := make(map[string]string, len(readmes))
	for _, r := range readmes {
		shas[r.Repo] = r.SHA
	}
	out := make([]Manifest, 0, len(results))
	for _, r := range results {
		seen := map[string]bool{}
		var rules []string
		for _, rule := range r.rules {
			if !seen[rule] {
				seen[rule] = true
				rules = append(rules, rule)
			}
		}
		sort.Strings(rules)
		out = append(out, Manifest{
			Repo: r.repo, SHA: shas[r.repo], Language: r.language,
			Words: r.words, Score: r.score, Rules: rules,
		})
	}
	return out
}

// pre2022Report writes the measurement: how the sample was drawn, how the scores are
// distributed, how many cross the threshold, and which rules cost the most.
func pre2022Report(w *strings.Builder, results []readmeResult, skipped int, provenance, spread string) {
	if len(results) == 0 {
		w.WriteString("no eligible samples\n")
		return
	}
	scores := make([]int, 0, len(results))
	words := make([]int, 0, len(results))
	for _, r := range results {
		scores = append(scores, r.score)
		words = append(words, r.words)
	}
	sort.Ints(scores)
	sort.Ints(words)

	fmt.Fprintf(w, "pre-2022 false positive measurement\n\n")
	fmt.Fprintf(w, "%s\n", provenance)
	fmt.Fprintf(w, "Either way the prose predates general writing models, so it is human without\n")
	fmt.Fprintf(w, "needing a rater. This measures quiet, not detection: there are no machine\n")
	fmt.Fprintf(w, "samples here, so it says nothing about recall.\n\n")
	fmt.Fprintf(w, "samples scored:   %d\n", len(results))
	if spread != "" {
		fmt.Fprintf(w, "ecosystems:       %s\n", spread)
	}
	fmt.Fprintf(w, "samples skipped:  %d (under %d prose words, or not English)\n", skipped, pre2022MinWords)
	fmt.Fprintf(w, "prose per sample: median %d words\n\n", percentile(words, 50))

	fmt.Fprintf(w, "score distribution\n")
	for _, p := range []struct {
		label string
		value float64
	}{{"median", 50}, {"p75", 75}, {"p90", 90}, {"p99", 99}} {
		fmt.Fprintf(w, "  %-8s %d\n", p.label, percentile(scores, p.value))
	}
	fmt.Fprintf(w, "  %-8s %d\n\n", "max", scores[len(scores)-1])

	fp := falsePositives(results)
	rate := float64(len(fp)) / float64(len(results)) * 100
	fmt.Fprintf(w, "at the reads-clean threshold of %d\n", pre2022Threshold)
	fmt.Fprintf(w, "  false positives: %d of %d (%.1f%%)\n\n", len(fp), len(results), rate)

	if len(fp) > 0 {
		fmt.Fprintf(w, "worst samples\n")
		for i, r := range fp {
			if i >= 10 {
				fmt.Fprintf(w, "  and %d more\n", len(fp)-i)
				break
			}
			fmt.Fprintf(w, "  %3d  %s (%d words)\n", r.score, r.repo, r.words)
		}
		w.WriteString("\n")
	}

	fmt.Fprintf(w, "rules firing most on human prose\n")
	ranked := costliestRules(results)
	for i, c := range ranked {
		if i >= pre2022TopRules {
			break
		}
		fmt.Fprintf(w, "  %-34s %4d hits in %4d samples (%.0f%%)\n",
			displayRule(c.rule), c.hits, c.samples, float64(c.samples)/float64(len(results))*100)
	}
}
