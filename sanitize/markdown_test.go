package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestCodeRanges checks which byte ranges count as markdown code.
func TestCodeRanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want [][2]int
	}{{ // Test 0: Plain text has no code.
		In: "no code here", Want: nil,
	}, { // Test 1: A backtick fence covers its lines.
		In: "before\n```\nx — y\n```\nafter", Want: [][2]int{{7, 22}},
	}, { // Test 2: A tilde fence works the same way.
		In: "~~~\ncode\n~~~\n", Want: [][2]int{{0, 12}},
	}, { // Test 3: An unclosed fence runs to the end of the text.
		In: "```\ncode", Want: [][2]int{{0, 8}},
	}, { // Test 4: A fence opener can be indented up to three spaces.
		In: "  ```\ncode\n  ```", Want: [][2]int{{0, 16}},
	}, { // Test 5: The closing fence may be longer than the opener.
		In: "```\ncode\n`````", Want: [][2]int{{0, 14}},
	}, { // Test 6: An inline span covers its backticks.
		In: "run `x; y` now", Want: [][2]int{{4, 10}},
	}, { // Test 7: A double-backtick span can hold a single backtick.
		In: "a ``b ` c`` d", Want: [][2]int{{2, 11}},
	}, { // Test 8: A lone backtick with no partner is plain text.
		In: "a ` b", Want: nil,
	}, { // Test 9: A span does not reach past a blank line.
		In: "a ` b\n\nc ` d", Want: nil,
	}, { // Test 10: A span may wrap across a single line break.
		In: "a `b\nc` d", Want: [][2]int{{2, 7}},
	}, { // Test 11: Fences and inline spans mix.
		In: "use `x`\n```\ny\n```", Want: [][2]int{{4, 7}, {8, 17}},
	}, { // Test 12: Backticks inside a fence do not open a span outside it.
		In: "```\na ` b\n```\nplain", Want: [][2]int{{0, 13}},
	}, { // Test 13: An indented block set off by a blank line is code.
		In: "a\n\n    code\n\nb", Want: [][2]int{{3, 11}},
	}, { // Test 14: An indented block at the start of the text is code.
		In: "    code\nplain", Want: [][2]int{{0, 8}},
	}, { // Test 15: An indented line that continues a paragraph is not code.
		In: "text\n    more text\n", Want: nil,
	}, { // Test 16: A tab-indented block counts as code.
		In: "a\n\n\tcode\n\nb", Want: [][2]int{{3, 8}},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := codeRanges(test.In)
			if diff := cmp.Diff(test.Want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCodeSegments checks that the code substrings come back in order.
func TestCodeSegments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In   string
		Want []string
	}{{ // Test 0: Plain text has no segments.
		In: "no code", Want: nil,
	}, { // Test 1: A fence and an inline span come back in order.
		In: "use `x`\n```\ny\n```", Want: []string{"`x`", "```\ny\n```"},
	}, { // Test 2: An indented block is a segment.
		In: "a\n\n    code\n\nb", Want: []string{"    code"},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := CodeSegments(test.In)
			if diff := cmp.Diff(test.Want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
