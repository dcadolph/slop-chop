package badge

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/sanitize"
)

// TestSVG checks that each band renders its own color, that the geometry grows with
// the number of digits, and that the markup is a well-formed single SVG element.
func TestSVG(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantColor string
		WantWidth string
		In        sanitize.Score
	}{{ // Test 0: A clean score renders lime.
		In: sanitize.Score{Value: 2}, WantColor: colorLow, WantWidth: `width="93"`,
	}, { // Test 1: A mixed score renders amber.
		In: sanitize.Score{Value: 40}, WantColor: colorMid, WantWidth: `width="100"`,
	}, { // Test 2: A dense score renders salmon.
		In: sanitize.Score{Value: 85}, WantColor: colorHigh, WantWidth: `width="100"`,
	}, { // Test 3: Three digits widen the value box.
		In: sanitize.Score{Value: 100}, WantColor: colorHigh, WantWidth: `width="107"`,
	}, { // Test 4: Zero is still a low score, not an unknown one.
		In: sanitize.Score{Value: 0}, WantColor: colorLow, WantWidth: `width="93"`,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := SVG(test.In)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, test.WantColor) {
				t.Errorf("want color %s in:\n%s", test.WantColor, got)
			}
			if !strings.Contains(got, test.WantWidth) {
				t.Errorf("want %s in:\n%s", test.WantWidth, got)
			}
			if !strings.HasPrefix(got, "<svg") || !strings.HasSuffix(strings.TrimSpace(got), "</svg>") {
				t.Errorf("not a bare svg element:\n%s", got)
			}
			if !strings.Contains(got, fmt.Sprintf(">%d</text>", test.In.Value)) {
				t.Errorf("want the value as text in:\n%s", got)
			}
			if !strings.Contains(got, fmt.Sprintf(`aria-label="slop score: %d of 100"`, test.In.Value)) {
				t.Errorf("want an accessible name in:\n%s", got)
			}
		})
	}
}

// TestUnknown checks the degraded badge renders gray with a readable placeholder.
func TestUnknown(t *testing.T) {
	t.Parallel()
	got, err := Unknown()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, colorUnknown) {
		t.Errorf("want the unknown color in:\n%s", got)
	}
	if !strings.Contains(got, ">n/a</text>") {
		t.Errorf("want the placeholder in:\n%s", got)
	}
	if !strings.Contains(got, `aria-label="slop score: unavailable"`) {
		t.Errorf("want an accessible name in:\n%s", got)
	}
}

// TestLabelIsFixed checks the label never varies, since a caller-supplied label would
// let a hosted endpoint put arbitrary text on the project's domain.
func TestLabelIsFixed(t *testing.T) {
	t.Parallel()
	scored, err := SVG(sanitize.Score{Value: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	unknown, err := Unknown()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, got := range []string{scored, unknown} {
		if !strings.Contains(got, ">"+label+"</text>") {
			t.Errorf("want the fixed label in:\n%s", got)
		}
	}
}
