package sanitize

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// plainSample builds a sample of short, plain, first-person sentences long enough to
// carry a fingerprint. n is the number of sentences.
func plainSample(n int) string {
	var sb strings.Builder
	for i := range n {
		fmt.Fprintf(&sb, "I wrote the note on day %d and I sent it. ", i)
		if i%3 == 2 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// TestNewFingerprintSample checks the sample-size guard on both sides of the floor.
func TestNewFingerprintSample(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name    string
		In      []string
		Want    error
		WantMin int
	}{{ // Test 0: A short note cannot carry a fingerprint.
		Name: "one sentence", In: []string{"I wrote this."}, Want: ErrSample,
	}, { // Test 1: No samples at all.
		Name: "none", In: nil, Want: ErrSample,
	}, { // Test 2: Enough sentences clears the floor.
		Name: "long enough", In: []string{plainSample(40)}, Want: nil, WantMin: fingerprintMinWords,
	}, { // Test 3: Several short samples pool into one fingerprint.
		Name: "pooled", In: []string{plainSample(20), plainSample(20)}, Want: nil,
		WantMin: fingerprintMinWords,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			f, err := NewFingerprint(test.In...)
			if !errors.Is(err, test.Want) {
				t.Fatalf("err = %v, want %v", err, test.Want)
			}
			if test.Want != nil {
				return
			}
			if f.Words < test.WantMin {
				t.Errorf("words = %d, want at least %d", f.Words, test.WantMin)
			}
			if len(f.Metrics) != len(metricDefs) {
				t.Errorf("metrics = %d, want %d", len(f.Metrics), len(metricDefs))
			}
			for _, m := range MetricList() {
				if f.Metrics[m.Name].Band <= 0 {
					t.Errorf("%s has band %v, want a positive band", m.Name, f.Metrics[m.Name].Band)
				}
			}
		})
	}
}

// TestNewFingerprintBandsWiden checks that samples which disagree widen the band, so a
// habit that varies from piece to piece is not read as drift later.
func TestNewFingerprintBandsWiden(t *testing.T) {
	t.Parallel()
	// Both samples are long enough to vote, and their comma rates sit far apart.
	dense := strings.Repeat("The note, which I wrote, was late, again, and short. ", 40)
	bare := strings.Repeat("The note was late and short so I sent another one instead. ", 40)
	steady, err := NewFingerprint(bare, bare)
	if err != nil {
		t.Fatalf("steady: %v", err)
	}
	varied, err := NewFingerprint(bare, dense)
	if err != nil {
		t.Fatalf("varied: %v", err)
	}
	if got, want := varied.Metrics["commas"].Band, steady.Metrics["commas"].Band; got <= want {
		t.Errorf("varied band = %v, want wider than the steady band of %v", got, want)
	}
	if got := steady.Metrics["commas"].Band; got != 0.5 {
		t.Errorf("steady band = %v, want the floor of 0.5", got)
	}
}

// TestCompare checks what a comparison reports: nothing for the writer's own register, the
// drifted traits for a machine's, and a sample error for text too short to read.
func TestCompare(t *testing.T) {
	t.Parallel()
	mine, err := NewFingerprint(plainSample(40))
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}

	tests := []struct {
		Name       string
		In         string
		Want       error
		WantDrift  []string
		WantSorted bool
	}{{ // Test 0: More of the same writing reads like the writer.
		Name: "same register", In: plainSample(12), Want: nil, WantDrift: nil,
	}, { // Test 1: A long, formal, latinate register drifts on several traits.
		Name: "machine register",
		In: strings.Repeat("The comprehensive implementation of organizational observability "+
			"strategies demonstrates substantial operational improvements across distributed "+
			"infrastructure environments throughout the transition period. ", 8),
		Want:       nil,
		WantDrift:  []string{"long-words", "sentence-length"},
		WantSorted: true,
	}, { // Test 2: A note is too short to carry a reading.
		Name: "too short", In: "I wrote this one.", Want: ErrSample,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got, err := mine.Compare(test.In)
			if !errors.Is(err, test.Want) {
				t.Fatalf("err = %v, want %v", err, test.Want)
			}
			if test.Want != nil {
				return
			}
			var names []string
			for _, d := range got {
				names = append(names, d.Metric)
			}
			for _, want := range test.WantDrift {
				if !contains(names, want) {
					t.Errorf("drift = %v, want it to include %s", names, want)
				}
			}
			if test.WantDrift == nil && len(got) > 0 {
				t.Errorf("drift = %v, want none", names)
			}
			if test.WantSorted {
				for i := 1; i < len(got); i++ {
					if got[i-1].Off < got[i].Off {
						t.Errorf("drift is not sorted by distance: %v", got)
					}
				}
			}
			for _, d := range got {
				if d.Note == "" || d.Unit == "" {
					t.Errorf("%s reported without words: %+v", d.Metric, d)
				}
			}
		})
	}
}

// contains reports whether s holds v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestCompareDirection checks that the note names which way the text moved, since a
// report that only says "different" is no help to a writer.
func TestCompareDirection(t *testing.T) {
	t.Parallel()
	terse, err := NewFingerprint(plainSample(40))
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	long := strings.Repeat("I wrote the note on the day it was due and I sent it along "+
		"to the team before the meeting started, which is what I always do when the week "+
		"runs late and nobody has read the draft yet. ", 6)
	got, err := terse.Compare(long)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	found := false
	for _, d := range got {
		if d.Metric != "sentence-length" {
			continue
		}
		found = true
		if d.Got <= d.Want {
			t.Errorf("got %v, want a value above the fingerprint's %v", d.Got, d.Want)
		}
		if !strings.Contains(d.Note, "longer") {
			t.Errorf("note = %q, want it to say the sentences ran longer", d.Note)
		}
	}
	if !found {
		t.Errorf("sentence-length did not drift on much longer sentences: %+v", got)
	}
}

// TestCompareEmpty checks that comparing against a fingerprint that was never measured
// fails rather than reporting everything as drift.
func TestCompareEmpty(t *testing.T) {
	t.Parallel()
	var f Fingerprint
	if !f.Empty() {
		t.Errorf("Empty() = false on a zero fingerprint")
	}
	if _, err := f.Compare(plainSample(20)); !errors.Is(err, ErrSample) {
		t.Errorf("err = %v, want ErrSample", err)
	}
}

// TestMeasure checks the raw tallies each trait is read off, since every number above
// rests on them.
func TestMeasure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want counts
	}{{ // Test 0: One plain sentence.
		Name: "plain", In: "The mail came late today.",
		Want: counts{words: 5, sentences: 1, paragraphs: 1, lengths: []int{5}},
	}, { // Test 1: Contractions count, possessives do not.
		Name: "contractions", In: "It's late and the writer's note isn't here. I'd know.",
		Want: counts{words: 10, sentences: 2, paragraphs: 1, lengths: []int{8, 2},
			contractions: 3, firstPerson: 1},
	}, { // Test 2: Spaced hyphens are dashes, compound words are not.
		Name: "dashes", In: "The well-known fix - the only one - broke. It was em—dashed too.",
		Want: counts{words: 13, sentences: 2, paragraphs: 1, lengths: []int{9, 4}, dashes: 3,
			longWords: 1},
	}, { // Test 3: Questions and lowercase openings are counted per sentence.
		Name: "questions", In: "Why did it fail? because nobody looked at the logs.",
		Want: counts{words: 10, sentences: 2, paragraphs: 1, lengths: []int{4, 6},
			questions: 1, lowerStarts: 1},
	}, { // Test 4: Long words are measured in letters, so punctuation does not inflate them.
		Name: "long words", In: "The implementation, unfortunately, regressed.",
		Want: counts{words: 4, sentences: 1, paragraphs: 1, lengths: []int{4}, commas: 2,
			longWords: 3},
	}, { // Test 5: Code, headings, tables, and list markers are not prose.
		Name: "markdown",
		In: "# Heading\n\nThe fix landed.\n\n| a | b |\n| - | - |\n\n- one item here\n\n" +
			"```\nfmt.Println(\"x; y, z\")\n```\n",
		// The list item keeps its sentence and loses its bullet, so it opens lowercase.
		Want: counts{words: 6, sentences: 2, paragraphs: 2, lengths: []int{3, 3}, lowerStarts: 1},
	}, { // Test 6: Front matter is metadata rather than writing.
		Name: "front matter", In: "---\ntitle: Notes; draft\n---\n\nThe fix landed.\n",
		Want: counts{words: 3, sentences: 1, paragraphs: 1, lengths: []int{3}},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := measure(test.In)
			if diff := cmp.Diff(test.Want, got, cmp.AllowUnexported(counts{})); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestVariation checks the spread measurement, including the too-little-to-judge cases
// that would otherwise divide by zero.
func TestVariation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   []int
		Want float64
	}{{ // Test 0: Too few sentences to read a rhythm.
		Name: "too few", In: []int{4, 9}, Want: 0,
	}, { // Test 1: Every sentence the same length is the flattest rhythm there is.
		Name: "flat", In: []int{7, 7, 7, 7}, Want: 0,
	}, { // Test 2: Empty sentences cannot divide.
		Name: "zero mean", In: []int{0, 0, 0}, Want: 0,
	}, { // Test 3: A real spread, which is the standard deviation over the mean.
		Name: "spread", In: []int{5, 10, 15}, Want: math.Sqrt(50.0/3) / 10,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := variation(test.In); math.Abs(got-test.Want) > 1e-9 {
				t.Errorf("variation = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestProseOnly checks that the structure a writer did not compose is removed before
// anything is counted.
func TestProseOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want string
	}{{ // Test 0: A heading is a label, not a sentence.
		Name: "heading", In: "## Notes\nThe fix landed.", Want: "\nThe fix landed.\n",
	}, { // Test 1: A bullet is furniture, its sentence is not.
		Name: "bullet", In: "- the fix landed\n", Want: "the fix landed\n\n",
	}, { // Test 2: A numbered item keeps its text.
		Name: "numbered", In: "1. the fix landed\n", Want: "the fix landed\n\n",
	}, { // Test 3: A table is data.
		Name: "table", In: "| a | b |\n", Want: "\n\n",
	}, { // Test 4: A rule is punctuation.
		Name: "rule", In: "text\n\n---\n\nmore\n", Want: "text\n\n\n\nmore\n\n",
	}, { // Test 5: Front matter is metadata.
		Name: "front matter", In: "---\na: b\n---\ntext\n", Want: "\n\n\ntext\n\n",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, proseOnly(test.In)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestContractionCount checks the possessive guard, which is the difference between
// measuring register and counting apostrophes.
func TestContractionCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want int
	}{
		{"don't", 1}, {"isn't", 1}, {"we're", 1}, {"I've", 1}, {"they'll", 1},
		{"I'd", 1}, {"I'm", 1}, {"it's", 1}, {"It's", 1}, {"that's", 1},
		{"writer's", 0}, {"dog's", 0}, {"note", 0}, {"'quoted'", 0}, {"o'clock", 0},
		{"it’s", 1},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.In), func(t *testing.T) {
			t.Parallel()
			if got := contractionCount(test.In); got != test.Want {
				t.Errorf("contractionCount(%q) = %d, want %d", test.In, got, test.Want)
			}
		})
	}
}

// TestCountDashes checks that only the dashes a writer chose are counted.
func TestCountDashes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want int
	}{
		{"a — b", 1}, {"a – b", 1}, {"a - b", 1}, {"a -- b", 1},
		{"well-known", 0}, {"a-b-c", 0}, {"trailing -", 0}, {"", 0},
		{"a - b - c", 2},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.In), func(t *testing.T) {
			t.Parallel()
			if got := countDashes(test.In); got != test.Want {
				t.Errorf("countDashes(%q) = %d, want %d", test.In, got, test.Want)
			}
		})
	}
}

// TestMetricList checks that every measured trait is named and described, since the CLI
// prints a fingerprint straight off this list.
func TestMetricList(t *testing.T) {
	t.Parallel()
	list := MetricList()
	if len(list) != len(metricDefs) {
		t.Fatalf("MetricList = %d entries, want %d", len(list), len(metricDefs))
	}
	seen := map[string]bool{}
	for _, m := range list {
		if m.Name == "" || m.Unit == "" {
			t.Errorf("metric described as %+v", m)
		}
		if seen[m.Name] {
			t.Errorf("%s listed twice", m.Name)
		}
		seen[m.Name] = true
	}
	for _, d := range metricDefs {
		if d.high == "" || d.low == "" || d.floor <= 0 {
			t.Errorf("%s is not fully defined: %+v", d.name, d)
		}
	}
}
