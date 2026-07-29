package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestAnchors checks which load-bearing tokens are pulled from prose. The order of the
// result is not contractual, so the comparison sorts both sides.
func TestAnchors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want []string
	}{{ // Test 0: Plain prose has no anchors.
		In: "just some words here", Want: nil,
	}, { // Test 1: A percentage and a bare number are anchors.
		In: "we hit 99.9% uptime and 3 nines", Want: []string{"99.9%", "3"},
	}, { // Test 2: URLs and emails are anchors.
		In:   "see https://example.com/x and mail a.b@c.co",
		Want: []string{"https://example.com/x", "a.b@c.co"},
	}, { // Test 3: Money keeps its magnitude, so a changed suffix shows up.
		In: "grew from $4.2M to $4.2B", Want: []string{"$4.2M", "$4.2B"},
	}, { // Test 4: Versions and dates come through as single tokens.
		In: "v1.2.3 shipped 2026-07-07", Want: []string{"1.2.3", "2026-07-07"},
	}, { // Test 5: All-caps acronyms are anchors; Title case is not.
		In: "The API speaks HTTP, not Http", Want: []string{"API", "HTTP"},
	}, { // Test 6: A trailing sentence period is not part of a number.
		In: "there are 3.", Want: []string{"3"},
	}, { // Test 7: Numbers inside inline code are masked out.
		In: "run `port 8080` then 42", Want: []string{"42"},
	}, { // Test 8: Numbers inside a fenced block are masked out.
		In: "```\nlisten 8080\n```\nreal 42", Want: []string{"42"},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := Anchors(test.In)
			less := func(a, b string) bool { return a < b }
			if diff := cmp.Diff(test.Want, got, cmpopts.SortSlices(less), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
