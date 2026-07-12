package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestUnicodeNormalize checks that the default profile folds non-breaking spaces to normal
// spaces and strips paste-cruft zero-width characters, while leaving a zero-width joiner in
// an emoji sequence alone.
func TestUnicodeNormalize(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name string
		In   string
		Want string
	}{
		{Name: "nbsp folds and collapses", In: "two\u00a0\u00a0spaces of air", Want: "two spaces of air"},
		{Name: "zero-width space stripped", In: "zero\u200bwidth split", Want: "zerowidth split"},
		{Name: "stray bom stripped", In: "a\ufeffb of c", Want: "ab of c"},
		{Name: "zwj emoji preserved", In: "hi \U0001F468\u200d\U0001F469 there", Want: "hi \U0001F468\u200d\U0001F469 there"},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("Fix(%q) mismatch (-want +got):\n%s", test.In, diff)
			}
		})
	}
}

// TestCheckLineColMultibyte checks that findings carry the right one-based line and rune
// column after the single forward pass that assigns positions, including across a multibyte
// character and on a second line.
func TestCheckLineColMultibyte(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// "él" is two runes but three bytes, so a tell after it must report a rune column.
	in := "él robust\nplan robust here"
	var got [][2]int
	for _, f := range s.Check(in) {
		if f.Rule == "word:robust" {
			got = append(got, [2]int{f.Line, f.Col})
		}
	}
	want := [][2]int{{1, 4}, {2, 6}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("robust line/col mismatch (-want +got):\n%s", diff)
	}
}
