package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/slop-chop/sanitize"
)

// prose returns a passage of n plain words, long enough to clear the eligibility floor
// without carrying a tell of its own.
func prose(n int) string {
	return strings.TrimSpace(strings.Repeat("the survey crew set a benchmark on the ridge that morning ", n/10+1))
}

// TestAsciiShare checks the language filter, since scoring a README written in another
// script measures the tokenizer rather than the writing.
func TestAsciiShare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want float64
	}{
		{Name: "plain english", In: "the quick brown fox", Want: 1},             // Test 0.
		{Name: "no letters at all", In: "1234 !!! ---", Want: 1},                // Test 1.
		{Name: "half and half", In: "abcd \u4f60\u597d\u4e16\u754c", Want: 0.5}, // Test 2.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := asciiShare(test.In); got != test.Want {
				t.Errorf("asciiShare(%q) = %v, want %v", test.In, got, test.Want)
			}
		})
	}
}

// TestEligible checks that a README is scored only when it carries enough English prose.
// A badge wall scores nothing meaningful and would only dilute the measurement.
func TestEligible(t *testing.T) {
	t.Parallel()
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name string
		In   string
		Want bool
	}{{ // Test 0: Real prose, well past the floor.
		Name: "long prose", In: prose(300), Want: true,
	}, { // Test 1: Too short to read anything from.
		Name: "stub", In: "# thing\n\nA tool.", Want: false,
	}, { // Test 2: A wall of badges is not prose.
		Name: "badges only", In: strings.Repeat("![b](https://img.shields.io/x) ", 80), Want: false,
	}, { // Test 3: Long, but not English.
		Name: "non-english", In: strings.Repeat("\u4f60\u597d\u4e16\u754c ", 300), Want: false,
	}, { // Test 4: Code fences are masked, so a file of code is not prose.
		Name: "code only", In: "```go\n" + strings.Repeat("func f() { return }\n", 200) + "```", Want: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if _, got := eligible(s, test.In); got != test.Want {
				t.Errorf("eligible = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestPercentile checks the nearest-rank pick, so every number the report prints is a
// score some real sample actually received.
func TestPercentile(t *testing.T) {
	t.Parallel()
	sorted := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	tests := []struct {
		P    float64
		Want int
	}{
		{P: 0, Want: 0},   // Test 0: The floor.
		{P: 50, Want: 5},  // Test 1: The median.
		{P: 90, Want: 9},  // Test 2: The tail.
		{P: 100, Want: 9}, // Test 3: Clamped to the last element.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := percentile(sorted, test.P); got != test.Want {
				t.Errorf("percentile(%v) = %d, want %d", test.P, got, test.Want)
			}
		})
	}
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("percentile(nil) = %d, want 0", got)
	}
}

// TestCostliestRules checks the ranking that makes the run actionable: rules are ordered
// by how many distinct samples they touched, not by raw hits, so one README repeating a
// word forty times cannot outrank a rule that fired across forty READMEs.
func TestCostliestRules(t *testing.T) {
	t.Parallel()
	results := []readmeResult{
		{repo: "a", rules: []string{"word:noisy", "word:noisy", "word:noisy", "word:broad"}},
		{repo: "b", rules: []string{"word:broad"}},
		{repo: "c", rules: []string{"word:broad"}},
	}
	got := costliestRules(results)
	want := []ruleCost{
		{rule: "word:broad", hits: 3, samples: 3},
		{rule: "word:noisy", hits: 3, samples: 1},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(ruleCost{}), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("costliestRules mismatch (-want +got):\n%s", diff)
	}
}

// TestFalsePositives checks that the threshold is inclusive and the worst sample sorts
// first, since the report leads with the ones worth reading.
func TestFalsePositives(t *testing.T) {
	t.Parallel()
	results := []readmeResult{
		{repo: "clean", score: pre2022Threshold - 1},
		{repo: "edge", score: pre2022Threshold},
		{repo: "worst", score: 90},
	}
	got := falsePositives(results)
	var names []string
	for _, r := range got {
		names = append(names, r.repo)
	}
	if diff := cmp.Diff([]string{"worst", "edge"}, names, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("falsePositives mismatch (-want +got):\n%s", diff)
	}
}

// TestPre2022Report checks that the report names the numbers a reader needs and stays
// honest about an empty run rather than printing a rate over no samples.
func TestPre2022Report(t *testing.T) {
	t.Parallel()
	var empty strings.Builder
	pre2022Report(&empty, nil, 0, provenanceOf(nil), "")
	if !strings.Contains(empty.String(), "no eligible samples") {
		t.Errorf("empty report = %q, want the no-samples line", empty.String())
	}

	var b strings.Builder
	pre2022Report(&b, []readmeResult{
		{repo: "a/a", score: 2, words: 200, rules: []string{"word:robust"}},
		{repo: "b/b", score: 40, words: 300, rules: []string{"word:robust", "char:—"}},
	}, 7, "test provenance", "Go 2")
	out := b.String()
	for _, want := range []string{
		"samples scored:   2",
		"samples skipped:  7",
		"false positives: 1 of 2 (50.0%)",
		"b/b",
		"word:robust",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q:\n%s", want, out)
		}
	}
}

// TestScoreReadmes checks the end-to-end path: ineligible samples are set aside rather
// than scored, and tidy findings never count as tells against human prose.
func TestScoreReadmes(t *testing.T) {
	t.Parallel()
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	results, skipped := scoreReadmes(s, []Readme{
		{Repo: "long/prose", Text: prose(300)},
		{Repo: "too/short", Text: "A tool."},
	})
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if len(results) != 1 || results[0].repo != "long/prose" {
		t.Fatalf("results = %+v, want the one eligible sample", results)
	}
	for _, rule := range results[0].rules {
		if sanitize.TidyRule(rule) {
			t.Errorf("tidy rule %q counted as a tell", rule)
		}
	}
}

// TestDisplayRule checks that an invisible character rule prints as something a reader
// can tell apart, since a zero-width space and a non-breaking space both render as
// nothing and would otherwise share a blank line in the report.
func TestDisplayRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want string
	}{
		{In: "word:robust", Want: "word:robust"},   // Test 0: Left alone.
		{In: "char:—", Want: "char:—"},             // Test 1: A visible character stays.
		{In: "char:\u200b", Want: "char:U+200B"},   // Test 2: Zero-width space.
		{In: "char:\u00a0", Want: "char:U+00A0"},   // Test 3: Non-breaking space.
		{In: "structural:x", Want: "structural:x"}, // Test 4: Not a character rule.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := displayRule(test.In); got != test.Want {
				t.Errorf("displayRule(%q) = %q, want %q", test.In, got, test.Want)
			}
		})
	}
}

// TestProvenanceOf checks that the report describes the sample it actually has. A pinned
// run and a push-date run carry different guarantees, and the mixed case must say so
// rather than claim either one.
func TestProvenanceOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   []Readme
		Want string
	}{
		{Name: "empty", In: nil, Want: "No samples"},                                                            // Test 0.
		{Name: "all pinned", In: []Readme{{PushedAt: "pinned<2022-01-01"}}, Want: "maintained"},                 // Test 1.
		{Name: "all abandoned", In: []Readme{{PushedAt: "2020-03-01"}}, Want: "abandoned"},                      // Test 2.
		{Name: "mixed", In: []Readme{{PushedAt: "pinned<2022-01-01"}, {PushedAt: "2020-03-01"}}, Want: "Mixed"}, // Test 3.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := provenanceOf(test.In); !strings.Contains(got, test.Want) {
				t.Errorf("provenanceOf = %q, want it to mention %q", got, test.Want)
			}
		})
	}
}

// TestLanguageSpread checks that the spread describes the scored samples. Counting what
// was collected instead would print ecosystem totals that do not add up to the sample the
// result was measured on.
func TestLanguageSpread(t *testing.T) {
	t.Parallel()
	got := languageSpread([]readmeResult{
		{language: "Go"}, {language: "Go"}, {language: "Rust"}, {language: ""},
	})
	if got != "Go 2, Rust 1" {
		t.Errorf("languageSpread = %q, want %q", got, "Go 2, Rust 1")
	}
	if got := languageSpread(nil); got != "" {
		t.Errorf("languageSpread(nil) = %q, want empty", got)
	}
}
