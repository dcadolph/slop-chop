package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestIgnoreDirectives checks that an inline directive silences its own line, the next-line
// directive silences the line after it, and other lines still report.
func TestIgnoreDirectives(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In        string
		WantLines []int
	}{{ // Test 0: A same-line directive silences that line.
		In:        "a robust plan <!-- slop-chop-ignore -->\nanother robust plan",
		WantLines: []int{2},
	}, { // Test 1: A next-line directive silences the line after it, not itself.
		In:        "<!-- slop-chop-ignore-next-line -->\nrobust skipped\nrobust flagged",
		WantLines: []int{3},
	}, { // Test 2: With no directive every match reports.
		In:        "robust one\nrobust two",
		WantLines: []int{1, 2},
	}, { // Test 3: A next-line directive with content on its own line still flags that line.
		In:        "robust here <!-- slop-chop-ignore-next-line -->\nrobust skipped",
		WantLines: []int{1},
	}}

	s := mustSanitizer(t)
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var got []int
			for _, f := range s.Check(test.In) {
				got = append(got, f.Line)
			}
			if diff := cmp.Diff(test.WantLines, got); diff != "" {
				t.Errorf("flagged lines mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestLineRanges checks the byte ranges returned for each line.
func TestLineRanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want [][2]int
	}{{ // Test 0: A single line spans the whole text.
		In: "abc", Want: [][2]int{{0, 3}},
	}, { // Test 1: Two lines split on the newline, which is excluded.
		In: "ab\ncd", Want: [][2]int{{0, 2}, {3, 5}},
	}, { // Test 2: A trailing newline yields a final empty range.
		In: "ab\n", Want: [][2]int{{0, 2}, {3, 3}},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, lineRanges(test.In)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
