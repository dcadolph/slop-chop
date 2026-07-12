package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestStructuralProtection checks that link and image destinations, autolinks, reference
// definitions, bare URLs, and front matter are never rewritten, while the visible link text
// and the surrounding prose still are.
func TestStructuralProtection(t *testing.T) {
	t.Parallel()
	p, err := ApplyPresets(DefaultProfile(), "cleaver")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name string
		In   string
		Want string
	}{
		{
			Name: "inline link target protected, text deslopped",
			In:   "Read [a robust guide](https://example.com/leverage-utilize) now.",
			Want: "Read [a solid guide](https://example.com/leverage-utilize) now.",
		},
		{
			Name: "image path protected, alt deslopped",
			In:   "![a robust diagram](img/robust-arch.png)",
			Want: "![a solid diagram](img/robust-arch.png)",
		},
		{
			Name: "autolink protected",
			In:   "See <https://example.com/leverage> here.",
			Want: "See <https://example.com/leverage> here.",
		},
		{
			Name: "reference definition protected",
			In:   "[home]: https://example.com/utilize-guide",
			Want: "[home]: https://example.com/utilize-guide",
		},
		{
			Name: "bare url protected, prose deslopped",
			In:   "This robust tool: https://example.com/leverage-utilize is here.",
			Want: "This solid tool: https://example.com/leverage-utilize is here.",
		},
		{
			Name: "front matter protected, body deslopped",
			In:   "---\ntitle: Leverage Robust Systems\n---\n\nThis is a robust body.",
			Want: "---\ntitle: Leverage Robust Systems\n---\n\nThis is a solid body.",
		},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("Fix mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBareURLNotFlagged checks that the default profile does not flag a buzzword that only
// appears inside a bare URL, so a link cannot fail a check gate on its own.
func TestBareURLNotFlagged(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.Check("Read https://example.com/leverage-guide now."); len(got) != 0 {
		t.Errorf("bare URL flagged, want none: %v", got)
	}
}
