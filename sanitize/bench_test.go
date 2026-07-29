package sanitize

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
)

// corpusData is the embedded labeled benchmark corpus, one JSON object per line.
//
//go:embed testdata/corpus.jsonl
var corpusData string

// benchPassage is one labeled passage in the benchmark corpus.
type benchPassage struct {
	// Label is ai, human, or technical.
	Label string `json:"label"`
	// Text is the passage.
	Text string `json:"text"`
	// Note records which tell or trap the passage exercises.
	Note string `json:"note"`
}

// loadCorpus parses the embedded JSONL corpus.
func loadCorpus(t *testing.T) []benchPassage {
	t.Helper()
	var out []benchPassage
	for i, line := range strings.Split(strings.TrimSpace(corpusData), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var p benchPassage
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Fatalf("corpus line %d: %v", i+1, err)
		}
		out = append(out, p)
	}
	return out
}

// isTell reports whether a finding is an AI-content tell, a buzzword, a stock phrase, a
// structural pattern, or a character tell, rather than a whitespace or punctuation cleanup.
func isTell(rule string) bool {
	for _, p := range []string{"word:", "phrase:", "structural:", "char:"} {
		if strings.HasPrefix(rule, p) {
			return true
		}
	}
	return false
}

// tellCount counts the AI-content tells among findings.
func tellCount(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if isTell(f.Rule) {
			n++
		}
	}
	return n
}

// TestBenchmark measures the engine against the labeled corpus and fails on a regression
// below the recorded floors. It reports tell recall on AI passages, precision on technical
// prose, and how well the score separates AI from human writing. Growing the corpus and
// raising the floors is how detection is proven to improve rather than guessed at.
func TestBenchmark(t *testing.T) {
	t.Parallel()
	corpus := loadCorpus(t)
	def, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const scoreThreshold = 25 // the "reads clean under 25" boundary

	var aiCount, aiTellHits, aiScored, aiScoreSum int
	var humanCount, humanFlagged, humanScoreSum int
	var techCount, techClean int
	for _, p := range corpus {
		findings := def.Check(p.Text)
		score := def.Score(p.Text).Value
		switch p.Label {
		case "ai":
			aiCount++
			aiScoreSum += score
			if tellCount(findings) > 0 {
				aiTellHits++
			}
			if score >= scoreThreshold {
				aiScored++
			}
		case "human":
			humanCount++
			humanScoreSum += score
			if score >= scoreThreshold {
				humanFlagged++
			}
		case "technical":
			techCount++
			if tellCount(findings) == 0 {
				techClean++
			}
		default:
			t.Fatalf("unknown label %q", p.Label)
		}
	}

	tellRecall := ratio(aiTellHits, aiCount)
	techPrecision := ratio(techClean, techCount)
	scoreRecall := ratio(aiScored, aiCount)
	scorePrecision := ratio(aiScored, aiScored+humanFlagged)
	scoreF1 := 0.0
	if scorePrecision+scoreRecall > 0 {
		scoreF1 = 2 * scorePrecision * scoreRecall / (scorePrecision + scoreRecall)
	}

	t.Logf("corpus: %d ai, %d human, %d technical", aiCount, humanCount, techCount)
	t.Logf("tell recall (ai with a tell):        %.2f (%d/%d)", tellRecall, aiTellHits, aiCount)
	t.Logf("technical precision (no false tell): %.2f (%d/%d)", techPrecision, techClean, techCount)
	t.Logf("score>=%d recall (ai):                %.2f (%d/%d)", scoreThreshold, scoreRecall, aiScored, aiCount)
	t.Logf("score>=%d human false-positives:      %d/%d", scoreThreshold, humanFlagged, humanCount)
	t.Logf("score>=%d precision / f1:             %.2f / %.2f", scoreThreshold, scorePrecision, scoreF1)

	meanAI, meanHuman := ratio(aiScoreSum, aiCount), ratio(humanScoreSum, humanCount)
	t.Logf("mean score ai / human / margin:      %.1f / %.1f / %.1f", meanAI, meanHuman, meanAI-meanHuman)

	// Floors are set below the current numbers, so an ordinary change passes while a real
	// regression fails. Raise them as the engine and the corpus improve.
	assertFloor(t, "tell recall", tellRecall, 0.90)
	assertFloor(t, "technical precision", techPrecision, 0.95)
	assertFloor(t, "score recall", scoreRecall, 0.90)
	assertFloor(t, "score precision", scorePrecision, 0.95)
	assertFloor(t, "score margin", meanAI-meanHuman, 40)
}

// ratio returns n/d as a float, or 0 when d is zero.
func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// assertFloor fails the test when got is below floor, naming the metric.
func assertFloor(t *testing.T, name string, got, floor float64) {
	t.Helper()
	if got < floor {
		t.Errorf("%s regressed: %.2f is below the floor of %.2f", name, got, floor)
	}
}
