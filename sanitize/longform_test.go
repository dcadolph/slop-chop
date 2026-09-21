package sanitize

import (
	_ "embed"
	"encoding/json"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"testing"
)

// The rule exemplar corpus is two sentences long at the median, which is the right size
// for pinning what a rule matches and the wrong size for anything that needs a paragraph.
// Cadence is the first signal here that reads a whole passage, and on the exemplar corpus
// only ten of a hundred and thirteen passages were even long enough to trigger it.
//
// This corpus is the instrument for signals at that scale. It is development data, so it
// is allowed to inform design. The held-out number still belongs to the locked evaluation
// corpus, which is what keeps a signal measured here from being a signal tuned here.

// longformData is the embedded long-form development corpus, one JSON object per line.
//
//go:embed testdata/longform.jsonl
var longformData string

// loadLongform parses the embedded long-form corpus.
func loadLongform(t *testing.T) []benchPassage {
	t.Helper()
	var out []benchPassage
	for i, line := range strings.Split(strings.TrimSpace(longformData), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var p benchPassage
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Fatalf("longform corpus line %d: %v", i+1, err)
		}
		out = append(out, p)
	}
	return out
}

// longformByLabel splits the corpus into its two halves.
func longformByLabel(t *testing.T) (ai, human []benchPassage) {
	t.Helper()
	for _, p := range loadLongform(t) {
		switch p.Label {
		case "ai":
			ai = append(ai, p)
		case "human":
			human = append(human, p)
		}
	}
	return ai, human
}

// longformFold assigns a passage to the development half or the holdout half, keyed off
// the text so the assignment is stable across runs and needs no field in the file. The
// evaluation corpus was burned by being developed against, and a development corpus is
// burned the same way the first time a threshold is chosen on it. Splitting it means the
// choosing happens on one half and the reporting on the other, so a number survives the
// work that produced it.
func longformFold(p benchPassage) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(p.Text))
	if h.Sum32()%2 == 0 {
		return "dev"
	}
	return "holdout"
}

// longformSplit returns the corpus divided by fold and label.
func longformSplit(t *testing.T, fold string) (ai, human []benchPassage) {
	t.Helper()
	for _, p := range loadLongform(t) {
		if longformFold(p) != fold {
			continue
		}
		switch p.Label {
		case "ai":
			ai = append(ai, p)
		case "human":
			human = append(human, p)
		}
	}
	return ai, human
}

// TestLongformSplitIsBalanced checks that the two folds are close enough in size and
// composition to stand in for each other. A holdout that happens to hold most of one
// model or one label reports a number about that accident rather than about the engine.
func TestLongformSplitIsBalanced(t *testing.T) {
	t.Parallel()
	devAI, devHu := longformSplit(t, "dev")
	holdAI, holdHu := longformSplit(t, "holdout")
	t.Logf("development half: %d ai, %d human", len(devAI), len(devHu))
	t.Logf("holdout half:     %d ai, %d human", len(holdAI), len(holdHu))
	for _, c := range []struct {
		Name string
		A, B int
	}{
		{"ai", len(devAI), len(holdAI)},
		{"human", len(devHu), len(holdHu)},
	} {
		small, large := min(c.A, c.B), max(c.A, c.B)
		if small < 20 || large > small*2 {
			t.Errorf("%s split is %d and %d, too small or too lopsided to hold out against",
				c.Name, c.A, c.B)
		}
	}
}

// TestLongformCorpusIsFitForPurpose checks the properties that make this corpus able to
// answer the question it exists for. A passage below the rhythm floor cannot be read by
// the signals under test, and a corpus that drifts back toward short passages would go on
// reporting numbers that mean nothing, which is the failure it was built to end.
func TestLongformCorpusIsFitForPurpose(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ai, human := longformByLabel(t)
	if len(ai) < 40 || len(human) < 40 {
		t.Fatalf("corpus holds %d ai and %d human passages, want at least 40 of each so a "+
			"rate means something", len(ai), len(human))
	}
	for _, p := range loadLongform(t) {
		if n := s.Score(p.Text).Sentences; n < cadenceMinSentences {
			t.Errorf("passage (%s) holds %d sentences, below the %d the rhythm signals need",
				p.Note, n, cadenceMinSentences)
		}
	}
}

// meanOf returns the mean of xs, or zero when empty.
func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// stdevOf returns the sample standard deviation of xs, or zero when it holds fewer than
// two values.
func stdevOf(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := meanOf(xs)
	sum := 0.0
	for _, x := range xs {
		sum += (x - m) * (x - m)
	}
	return math.Sqrt(sum / float64(len(xs)-1))
}

// cohensD returns the standardized difference between two groups, which is how far apart
// they sit in units of their own spread. It is the number that says whether a gap is worth
// anything, since a large difference between two very scattered groups separates nothing.
func cohensD(a, b []float64) float64 {
	na, nb := float64(len(a)), float64(len(b))
	if na < 2 || nb < 2 {
		return 0
	}
	sa, sb := stdevOf(a), stdevOf(b)
	pooled := math.Sqrt(((na-1)*sa*sa + (nb-1)*sb*sb) / (na + nb - 2))
	if pooled == 0 {
		return 0
	}
	return (meanOf(a) - meanOf(b)) / pooled
}

// medianOf returns the median of xs, or zero when empty. It copies first, since a caller
// reusing the slice afterward should not find it reordered.
func medianOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	return c[len(c)/2]
}

// TestLongformCadenceSeparation reports what the rhythm signals do on full-length prose,
// which is the measurement the exemplar corpus is too short to make. It gates the
// direction only. The size of the gap is a development number, and the held-out claim
// waits on a locked corpus collected after the ruleset was frozen.
func TestLongformCadenceSeparation(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ai, human := longformByLabel(t)
	collect := func(ps []benchPassage) (cv, points, score []float64, penalized int) {
		for _, p := range ps {
			sc := s.Score(p.Text)
			if sc.CadenceCV >= 0 {
				cv = append(cv, sc.CadenceCV)
			}
			points = append(points, float64(sc.Cadence))
			score = append(score, float64(sc.Value))
			if sc.Cadence > 0 {
				penalized++
			}
		}
		return cv, points, score, penalized
	}
	aiCV, aiPts, aiScore, aiHit := collect(ai)
	huCV, huPts, huScore, huHit := collect(human)

	t.Logf("passages:            ai %d, human %d", len(ai), len(human))
	t.Logf("cadence cv mean:     ai %.3f, human %.3f (lower is flatter)", meanOf(aiCV), meanOf(huCV))
	t.Logf("cadence cv median:   ai %.3f, human %.3f", medianOf(aiCV), medianOf(huCV))
	t.Logf("cohen's d on cv:     %.2f (negative means machine prose is flatter)", cohensD(aiCV, huCV))
	t.Logf("penalty fires on:    ai %d/%d (%.0f%%), human %d/%d (%.0f%%)",
		aiHit, len(ai), 100*float64(aiHit)/float64(len(ai)),
		huHit, len(human), 100*float64(huHit)/float64(len(human)))
	t.Logf("mean cadence points: ai %.2f, human %.2f", meanOf(aiPts), meanOf(huPts))
	t.Logf("mean total score:    ai %.1f, human %.1f", meanOf(aiScore), meanOf(huScore))
	t.Logf("cohen's d on score:  %.2f (positive means machine prose scores higher)", cohensD(aiScore, huScore))
	// The verdict line is what a reader acts on, so how many passages reach it matters
	// separately from how far apart the means sit. A signal that separates two groups
	// cleanly and leaves both below the threshold changes nobody's verdict.
	over := func(xs []float64) int {
		n := 0
		for _, x := range xs {
			if x >= 25 {
				n++
			}
		}
		return n
	}
	t.Logf("reach the 25 line:   ai %d/%d, human %d/%d",
		over(aiScore), len(aiScore), over(huScore), len(huScore))

	// The one thing that must hold for the penalty to be defensible at all: machine prose
	// has to be the flatter half. If this inverts, the penalty is charging human writing
	// for a habit it does not have, and it should come out rather than be re-tuned.
	if meanOf(aiCV) >= meanOf(huCV) {
		t.Errorf("machine cadence cv %.3f is not below human %.3f: the penalty is pointed "+
			"the wrong way and should be removed rather than adjusted",
			meanOf(aiCV), meanOf(huCV))
	}
}

// TestLongformCadenceSurvivesAttack measures how much of each signal a substitution
// attack can strip. The claim worth making is not that cadence cannot move: an evasion
// that swaps one word for two changes a sentence length, so it can. The claim is that the
// attack which strips tells wholesale barely reaches the rhythm, because escaping a
// rhythm means rewriting sentences rather than looking words up.
func TestLongformCadenceSurvivesAttack(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ai, _ := longformByLabel(t)
	var tellsBefore, tellsAfter, cadBefore, cadAfter float64
	for _, p := range ai {
		res := s.Attack(p.Text)
		before, after := s.Score(p.Text), s.Score(res.Text)
		tellsBefore += float64(before.Tells)
		tellsAfter += float64(after.Tells)
		cadBefore += float64(before.Cadence)
		cadAfter += float64(after.Cadence)
	}
	if tellsBefore == 0 || cadBefore == 0 {
		t.Skip("the corpus carries no tells or no cadence penalty, so there is nothing to attack")
	}
	tellLoss := 100 * (tellsBefore - tellsAfter) / tellsBefore
	cadLoss := 100 * (cadBefore - cadAfter) / cadBefore
	t.Logf("tells the attack removed:          %.0f%% (%.0f of %.0f)", tellLoss, tellsBefore-tellsAfter, tellsBefore)
	t.Logf("cadence points the attack removed: %.0f%% (%.0f of %.0f)", cadLoss, cadBefore-cadAfter, cadBefore)

	// A substitution reaches a rhythm only by accident, through an evasion that happens to
	// change a word count. If it ever starts removing cadence at anything like the rate it
	// removes tells, then the rhythm is reachable by lookup and is not the durable signal
	// it is carried for.
	if cadLoss >= tellLoss/2 {
		t.Errorf("the attack removed %.0f%% of the cadence penalty against %.0f%% of the "+
			"tells: a rhythm should not be this reachable by substitution", cadLoss, tellLoss)
	}
}

// TestLongformEvidenceHoldout reports what the evidence component does to a verdict on
// full-length prose, measured on the half of the corpus it was not chosen on. The shape
// and the multiplier were read off the development half, so this is the number that means
// anything, and it is kept as a test rather than a note so a later change to the weighting
// has to face it.
func TestLongformEvidenceHoldout(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// withoutEvidence subtracts the component back out, which is what a document scored
	// before accumulated tells counted for anything.
	reach := func(ps []benchPassage, withEvidence bool) int {
		n := 0
		for _, p := range ps {
			sc := s.Score(p.Text)
			v := float64(sc.Value)
			if !withEvidence {
				v = math.Max(0, v-float64(sc.Evidence))
			}
			if v >= 25 {
				n++
			}
		}
		return n
	}
	ai, human := longformSplit(t, "holdout")
	beforeRec, afterRec := reach(ai, false), reach(ai, true)
	beforeFP, afterFP := reach(human, false), reach(human, true)
	t.Logf("holdout half: %d machine, %d human", len(ai), len(human))
	t.Logf("  without evidence: %d/%d machine reach the line, %d/%d human", beforeRec, len(ai), beforeFP, len(human))
	t.Logf("  with evidence:    %d/%d machine reach the line, %d/%d human", afterRec, len(ai), afterFP, len(human))

	if afterRec <= beforeRec {
		t.Errorf("evidence recovered no machine prose on the holdout (%d against %d): the "+
			"component is costing precision for nothing", afterRec, beforeRec)
	}
	// Precision is the claim this engine is carried on. Recovering recall is worth
	// nothing if long human documents start reaching the line, and a long document is
	// exactly where accumulated evidence could run away.
	if afterFP > len(human)/20 {
		t.Errorf("evidence flagged %d of %d human documents, past the one in twenty this "+
			"is allowed to cost", afterFP, len(human))
	}
}
