package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/slop-chop/sanitize"
)

// words returns n filler words, for building samples that clear the length floor.
func words(n int) string {
	return strings.TrimSpace(strings.Repeat("plain note text here ", (n+3)/4))
}

// TestSpearman checks the rank correlation, including ties and the degenerate cases.
func TestSpearman(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		A, B []float64
		Want float64
	}{{ // Test 0: A monotone pair correlates perfectly whatever the spacing.
		Name: "monotone", A: []float64{1, 5, 90}, B: []float64{2, 3, 4}, Want: 1,
	}, { // Test 1: A reversed pair anticorrelates perfectly.
		Name: "reversed", A: []float64{1, 2, 3}, B: []float64{9, 5, 1}, Want: -1,
	}, { // Test 2: Ties share an average rank rather than breaking by position.
		Name: "ties", A: []float64{1, 2, 2, 3}, B: []float64{1, 2, 2, 3}, Want: 1,
	}, { // Test 3: Too few points is no correlation at all.
		Name: "too few", A: []float64{1, 2}, B: []float64{1, 2}, Want: math.NaN(),
	}, { // Test 4: A constant list has no variance to correlate.
		Name: "constant", A: []float64{4, 4, 4}, B: []float64{1, 2, 3}, Want: math.NaN(),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := spearman(test.A, test.B)
			if math.IsNaN(test.Want) != math.IsNaN(got) || (!math.IsNaN(test.Want) && math.Abs(got-test.Want) > 1e-9) {
				t.Errorf("spearman = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestSeparation checks the probability that a machine sample outscores a human one.
func TestSeparation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name      string
		AI, Human []float64
		Want      float64
	}{{ // Test 0: Full separation.
		Name: "perfect", AI: []float64{80, 90}, Human: []float64{2, 5}, Want: 1,
	}, { // Test 1: Fully inverted.
		Name: "inverted", AI: []float64{1, 2}, Human: []float64{50, 60}, Want: 0,
	}, { // Test 2: Ties count half.
		Name: "tie", AI: []float64{10}, Human: []float64{10}, Want: 0.5,
	}, { // Test 3: An empty side cannot separate.
		Name: "empty", AI: nil, Human: []float64{1}, Want: math.NaN(),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := separation(test.AI, test.Human)
			if math.IsNaN(test.Want) != math.IsNaN(got) || (!math.IsNaN(test.Want) && math.Abs(got-test.Want) > 1e-9) {
				t.Errorf("separation = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestCheckCorpus checks every way a sample can break the lock or the format.
func TestCheckCorpus(t *testing.T) {
	t.Parallel()
	good := Sample{ID: "a001", Source: "ai", Rules: "v0.36.0", Text: words(100)}
	tests := []struct {
		Name      string
		Samples   []Sample
		Dev       []string
		WantCount int
	}{{ // Test 0: A clean corpus has no problems.
		Name: "clean", Samples: []Sample{good}, WantCount: 0,
	}, { // Test 1: Every field violation is reported, not just the first.
		Name: "all wrong",
		Samples: []Sample{{
			ID: "", Source: "robot", Rules: "", Text: "short",
		}},
		WantCount: 4,
	}, { // Test 2: A duplicate id is caught.
		Name: "duplicate", Samples: []Sample{good, good}, WantCount: 1,
	}, { // Test 3: Text shared with the development corpus breaks the lock, even
		// reflowed and re-cased.
		Name:      "lock",
		Samples:   []Sample{good},
		Dev:       []string{"  " + strings.ToUpper(words(100)) + "\n"},
		WantCount: 1,
	}, { // Test 4: A paragraph break marks a sample on sight, since no human one has it.
		Name:      "layout",
		Samples:   []Sample{{ID: "a001", Source: "ai", Rules: "v0.36.0", Text: words(50) + "\n\n" + words(50)}},
		WantCount: 1,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := checkCorpus(test.Samples, test.Dev)
			if len(got) != test.WantCount {
				t.Errorf("problems = %d, want %d: %v", len(got), test.WantCount, got)
			}
		})
	}
}

// TestCheckRatings checks the rating validations.
func TestCheckRatings(t *testing.T) {
	t.Parallel()
	samples := []Sample{{ID: "a001"}}
	tests := []struct {
		Name      string
		Ratings   []Rating
		WantCount int
	}{{ // Test 0: A clean rating.
		Name: "clean", Ratings: []Rating{{Sample: "a001", Rater: "r01", Machine: 4}}, WantCount: 0,
	}, { // Test 1: An unknown sample, a missing rater, and an out-of-range answer.
		Name: "all wrong", Ratings: []Rating{{Sample: "zzz", Rater: "", Machine: 9}}, WantCount: 3,
	}, { // Test 2: One rater answering one sample twice would count as two raters.
		Name: "duplicate",
		Ratings: []Rating{
			{Sample: "a001", Rater: "r01", Machine: 4}, {Sample: "a001", Rater: "r01", Machine: 6},
		},
		WantCount: 1,
	}, { // Test 3: Two raters on one sample is the normal case, not a duplicate.
		Name: "two raters",
		Ratings: []Rating{
			{Sample: "a001", Rater: "r01", Machine: 4}, {Sample: "a001", Rater: "r02", Machine: 6},
		},
		WantCount: 0,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := checkRatings(test.Ratings, samples)
			if len(got) != test.WantCount {
				t.Errorf("problems = %d, want %d: %v", len(got), test.WantCount, got)
			}
		})
	}
}

// TestMeanRatings checks the per-sample aggregation.
func TestMeanRatings(t *testing.T) {
	t.Parallel()
	got := meanRatings([]Rating{
		{Sample: "a", Rater: "r1", Machine: 2},
		{Sample: "a", Rater: "r2", Machine: 4},
		{Sample: "b", Rater: "r1", Machine: 7},
	})
	if m := got["a"]; m.Mean != 3 || m.Count != 2 {
		t.Errorf("a = %+v, want mean 3 count 2", m)
	}
	if m := got["b"]; m.Mean != 7 || m.Count != 1 {
		t.Errorf("b = %+v, want mean 7 count 1", m)
	}
}

// TestRaterConsistency checks the pairwise agreement measure and its floor.
func TestRaterConsistency(t *testing.T) {
	t.Parallel()
	// Two raters agreeing on six shared samples correlate perfectly.
	var agree []Rating
	for i := 1; i <= 6; i++ {
		agree = append(agree,
			Rating{Sample: fmt.Sprintf("s%d", i), Rater: "r1", Machine: i},
			Rating{Sample: fmt.Sprintf("s%d", i), Rater: "r2", Machine: i},
		)
	}
	if got := raterConsistency(agree); math.Abs(got-1) > 1e-9 {
		t.Errorf("consistency = %v, want 1", got)
	}
	// Under five shared samples no pair qualifies.
	if got := raterConsistency(agree[:8]); !math.IsNaN(got) {
		t.Errorf("consistency = %v, want NaN under the floor", got)
	}
}

// TestReport checks the full report against a small rated corpus: the headline lines,
// both disagreement directions, and the thin-rating warning.
func TestReport(t *testing.T) {
	t.Parallel()
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	slop := strings.Repeat("In summary, we leverage robust synergy to seamlessly deliver. ", 12)
	plain := strings.Repeat("The mail came late and nobody minded much at all that day. ", 10)
	samples := []Sample{
		{ID: "a001", Source: "ai", Rules: "v0.36.0", Text: slop},
		{ID: "h001", Source: "human", Rules: "v0.36.0", Text: plain},
		// A human passage people misread as machine: high rating, low score.
		{ID: "h002", Source: "human", Rules: "v0.36.0", Text: plain + "It was fine."},
	}
	ratings := []Rating{
		{Sample: "a001", Rater: "r1", Machine: 7}, {Sample: "a001", Rater: "r2", Machine: 6},
		{Sample: "a001", Rater: "r3", Machine: 7},
		{Sample: "h001", Rater: "r1", Machine: 1}, {Sample: "h001", Rater: "r2", Machine: 2},
		{Sample: "h001", Rater: "r3", Machine: 1},
		{Sample: "h002", Rater: "r1", Machine: 6}, {Sample: "h002", Rater: "r2", Machine: 5},
	}
	rows := scoreSamples(s, samples, ratings)
	if len(rows) != 3 {
		t.Fatalf("scored rows = %d, want 3", len(rows))
	}
	var b strings.Builder
	report(&b, samples, rows, ratings)
	out := b.String()
	for _, want := range []string{
		"pinned to ruleset v0.36.0",
		"rated samples: 3 (1 ai, 2 human)",
		"fewer than 3 raters",
		"score vs human rating (Spearman):",
		"95% interval",
		"within machine samples:",
		"within human samples:",
		"ai/human separation by score:",
		"score components vs human rating",
		"density",
		"drumbeat",
		"tails",
		"h002",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

// TestReadLines checks the JSONL loader: a missing file is an empty corpus, a bad line
// names its position, and good lines parse.
func TestReadLines(t *testing.T) {
	t.Parallel()
	if got, err := readLines[Sample](filepath.Join(t.TempDir(), "missing.jsonl")); err != nil || got != nil {
		t.Errorf("missing file = %v, %v, want nil, nil", got, err)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	body := `{"id":"a001","source":"ai","text":"hi"}` + "\n\n" + `{"id":"h001","source":"human","text":"yo"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	want := []Sample{{ID: "a001", Source: "ai", Text: "hi"}, {ID: "h001", Source: "human", Text: "yo"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(bad, []byte("{}\nnot json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readLines[Sample](bad); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("bad line err = %v, want it to name line 2", err)
	}
}

// writeJSONL writes one JSON object per line into a temp file and returns its path.
func writeJSONL(t *testing.T, dir, name string, rows ...any) string {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		enc, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(enc)
		b.WriteString("\n")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRun drives the command end to end over its four outcomes: an empty corpus, a
// corpus that breaks the lock, samples with no ratings yet, and a full analysis.
func TestRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	shared := words(100)
	dev := writeJSONL(t, dir, "dev.jsonl", devPassage{Text: shared})
	empty := writeJSONL(t, dir, "empty.jsonl")

	// Distinct from the shared text, so this sample clears the lock.
	sample := Sample{ID: "a001", Source: "ai", Rules: "v0.36.0", Text: words(120)}
	good := writeJSONL(t, dir, "good.jsonl", sample)
	locked := writeJSONL(t, dir, "locked.jsonl",
		Sample{ID: "a002", Source: "ai", Rules: "v0.36.0", Text: shared})
	rated := writeJSONL(t, dir, "rated.jsonl",
		Rating{Sample: "a001", Rater: "r1", Machine: 5},
		Rating{Sample: "a001", Rater: "r2", Machine: 6},
	)
	ready := writeJSONL(t, dir, "ready.jsonl",
		Rating{Sample: "a001", Rater: "r1", Machine: 5},
		Rating{Sample: "a001", Rater: "r2", Machine: 6},
		Rating{Sample: "a001", Rater: "r3", Machine: 6},
	)

	tests := []struct {
		Name       string
		Check      bool
		Score      bool
		Paths      paths
		Want       error
		WantOut    string
		WantAbsent string
	}{{ // Test 0: An empty corpus passes the lock and says so.
		Name: "empty check", Check: true,
		Paths:   paths{samples: empty, ratings: empty, dev: []string{dev}},
		WantOut: "0 sample(s) and 0 rating(s), lock holds",
	}, { // Test 1: A sample whose text is in the development corpus breaks the lock.
		Name:  "lock broken",
		Paths: paths{samples: locked, ratings: empty, dev: []string{dev}},
		Want:  errCorpus,
	}, { // Test 2: A run with no samples names the protocol rather than reporting nothing.
		Name:    "no samples",
		Paths:   paths{samples: empty, ratings: empty, dev: []string{dev}},
		WantOut: "no samples yet",
	}, { // Test 3: Without ratings a plain run reports progress and reads no score.
		Name:       "unrated",
		Paths:      paths{samples: good, ratings: empty, dev: []string{dev}},
		WantOut:    "No score has been read",
		WantAbsent: "separation",
	}, { // Test 4: A plain run on a fully rated corpus still reads no score.
		Name:       "rated plain",
		Paths:      paths{samples: good, ratings: ready, dev: []string{dev}},
		WantOut:    "samples with 3 or more raters: 1 of 1",
		WantAbsent: "score vs human rating",
	}, { // Test 5: Scoring is refused while any sample is short of raters.
		Name: "score too early", Score: true,
		Paths: paths{samples: good, ratings: rated, dev: []string{dev}},
		Want:  errNotReady,
	}, { // Test 6: A fully rated corpus scores on request.
		Name: "score ready", Score: true,
		Paths:   paths{samples: good, ratings: ready, dev: []string{dev}},
		WantOut: "rated samples: 1",
	}, { // Test 7: An unreadable samples file is an error, not an empty corpus.
		Name:  "bad json",
		Paths: paths{samples: writeBad(t, dir), ratings: empty, dev: []string{dev}},
		Want:  nil, // any error; checked below
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			var out strings.Builder
			err := run(test.Check, test.Score, test.Paths, &out)
			switch {
			case test.Name == "bad json":
				if err == nil {
					t.Fatal("bad json returned no error")
				}
			case test.Want != nil:
				if !errors.Is(err, test.Want) {
					t.Fatalf("err = %v, want %v", err, test.Want)
				}
			case err != nil:
				t.Fatalf("run: %v", err)
			}
			if test.WantOut != "" && !strings.Contains(out.String(), test.WantOut) {
				t.Errorf("output = %q, want it to hold %q", out.String(), test.WantOut)
			}
			if test.WantAbsent != "" && strings.Contains(out.String(), test.WantAbsent) {
				t.Errorf("output = %q, want it to omit %q", out.String(), test.WantAbsent)
			}
		})
	}
}

// writeBad writes a file whose second line is not JSON.
func writeBad(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "bad-run.jsonl")
	if err := os.WriteFile(path, []byte("{}\nnope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClip checks the one-line shortener used by the disagreement rows.
func TestClip(t *testing.T) {
	t.Parallel()
	if got := clip("short text", 40); got != "short text" {
		t.Errorf("clip = %q, want it untouched", got)
	}
	if got := clip("  spaced\n\ttext  ", 40); got != "spaced text" {
		t.Errorf("clip = %q, want whitespace collapsed", got)
	}
	got := clip(strings.Repeat("word ", 40), 10)
	if len([]rune(got)) != 10 || !strings.HasSuffix(got, "…") {
		t.Errorf("clip = %q, want 10 runes ending in an ellipsis", got)
	}
}

// TestCorpusNote checks the note names the pinned ruleset and never calls the corpus
// burned, since the burned samples live in their own file and are not read.
func TestCorpusNote(t *testing.T) {
	t.Parallel()
	got := corpusNote([]Sample{
		{ID: "a001", Rules: "v0.41.0"}, {ID: "h001", Rules: "v0.41.0"},
	})
	for _, want := range []string{"2 samples", "pinned to ruleset v0.41.0", "is not read here"} {
		if !strings.Contains(got, want) {
			t.Errorf("note missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "were used to tune") {
		t.Errorf("note calls a clean corpus burned:\n%s", got)
	}
}

// TestCoverageOf checks rating progress is counted per sample against the rater floor.
func TestCoverageOf(t *testing.T) {
	t.Parallel()
	samples := []Sample{{ID: "a001"}, {ID: "h001"}, {ID: "h002"}}
	tests := []struct {
		Name    string
		In      []Rating
		WantCov coverage
	}{{ // Test 0: Nothing rated.
		Name: "empty", WantCov: coverage{Total: 3},
	}, { // Test 1: One sample at the floor, one under it, one untouched.
		Name: "partial",
		In: []Rating{
			{Sample: "a001", Rater: "r1", Machine: 6}, {Sample: "a001", Rater: "r2", Machine: 5},
			{Sample: "a001", Rater: "r3", Machine: 7}, {Sample: "h001", Rater: "r1", Machine: 2},
		},
		WantCov: coverage{Raters: 3, Ratings: 4, Rated: 2, Ready: 1, Total: 3},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.WantCov, coverageOf(samples, test.In)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBootstrapSpearman checks the interval brackets the point estimate, collapses on a
// perfect relation, widens on noise, is reproducible from its seed, and refuses too
// little data.
func TestBootstrapSpearman(t *testing.T) {
	t.Parallel()
	var line, noise, rating []float64
	for i := range 40 {
		line = append(line, float64(i))
		noise = append(noise, float64((i*37)%40))
		rating = append(rating, float64(i))
	}

	lo, hi := bootstrapSpearman(line, rating, 500, 1)
	if lo < 0.999 || hi > 1.0001 {
		t.Errorf("perfect relation interval = [%v, %v], want [1, 1]", lo, hi)
	}

	lo, hi = bootstrapSpearman(noise, rating, 500, 1)
	rho := spearman(noise, rating)
	if lo > rho || hi < rho {
		t.Errorf("interval [%v, %v] does not bracket the estimate %v", lo, hi, rho)
	}
	if hi-lo < 0.2 {
		t.Errorf("noisy interval [%v, %v] is implausibly narrow", lo, hi)
	}

	lo2, hi2 := bootstrapSpearman(noise, rating, 500, 1)
	if lo != lo2 || hi != hi2 {
		t.Errorf("same seed gave [%v, %v] then [%v, %v]", lo, hi, lo2, hi2)
	}

	lo, hi = bootstrapSpearman([]float64{1, 2}, []float64{1, 2}, 500, 1)
	if !math.IsNaN(lo) || !math.IsNaN(hi) {
		t.Errorf("two points gave [%v, %v], want NaN", lo, hi)
	}
}

// TestPilotSubset checks the pilot is the requested size, split between the labels,
// identical on every call, and blind to text, so a score cannot have chosen it.
func TestPilotSubset(t *testing.T) {
	t.Parallel()
	var samples []Sample
	for i := range 34 {
		samples = append(samples, Sample{ID: fmt.Sprintf("a%03d", i), Source: "ai", Text: "x"})
	}
	for i := range 50 {
		samples = append(samples, Sample{ID: fmt.Sprintf("h%03d", i), Source: "human", Text: "y"})
	}
	ids := func(list []Sample) []string {
		var out []string
		for _, s := range list {
			out = append(out, s.ID)
		}
		return out
	}

	tests := []struct {
		Name      string
		N         int
		WantCount int
		WantAI    int
	}{{ // Test 0: An even split.
		Name: "twenty", N: 20, WantCount: 20, WantAI: 10,
	}, { // Test 1: An odd size gives the extra one to the human half.
		Name: "odd", N: 21, WantCount: 21, WantAI: 10,
	}, { // Test 2: Zero means the whole corpus.
		Name: "zero", N: 0, WantCount: 84, WantAI: 34,
	}, { // Test 3: A size past the corpus means the whole corpus.
		Name: "too big", N: 500, WantCount: 84, WantAI: 34,
	}, { // Test 4: A short machine half is topped up from the human half.
		Name: "short half", N: 80, WantCount: 80, WantAI: 34,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := pilotSubset(samples, test.N)
			ai := 0
			for _, s := range got {
				if s.Source == "ai" {
					ai++
				}
			}
			if diff := cmp.Diff(test.WantCount, len(got)); diff != "" {
				t.Errorf("count mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantAI, ai); diff != "" {
				t.Errorf("machine count mismatch (-want +got):\n%s", diff)
			}
		})
	}

	first := ids(pilotSubset(samples, 20))
	if diff := cmp.Diff(first, ids(pilotSubset(samples, 20))); diff != "" {
		t.Errorf("pilot changed between calls (-first +second):\n%s", diff)
	}

	// Changing every text must not change the choice, since a choice that could see the
	// text could see the score.
	retexted := make([]Sample, len(samples))
	for i, s := range samples {
		s.Text = strings.Repeat("In summary, we leverage robust synergy. ", i+1)
		retexted[i] = s
	}
	if diff := cmp.Diff(first, ids(pilotSubset(retexted, 20))); diff != "" {
		t.Errorf("pilot depends on text (-before +after):\n%s", diff)
	}
}
