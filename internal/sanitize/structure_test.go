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

// TestMarkdownExtensionsNotFlagged checks that attr_list blocks, icon shortcodes, and inline
// image markers are not mistaken for prose punctuation, while real space-before-punctuation
// is still caught.
func TestMarkdownExtensionsNotFlagged(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for testNum, in := range []string{
		"Logo ![x](a.png){ .hero-logo }",
		"[Go](q.md){ .md-button .md-button--primary }",
		"Card :material-flash:{ .lg .middle } Title",
		"See this ![diagram](d.png) here.",
	} {
		t.Run(fmt.Sprintf("clean %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := s.Check(in); len(got) != 0 {
				t.Errorf("Check(%q) flagged markdown structure: %v", in, got)
			}
		})
	}
	if got := s.Check("Wow . And here , too"); len(got) == 0 {
		t.Errorf("real space-before-punct should still be caught")
	}
}

// TestTechnicalCollocationAllowed checks that a term of art keeps its flagged word through
// both the check and the cleaver swap, while the bare word is still caught.
func TestTechnicalCollocationAllowed(t *testing.T) {
	t.Parallel()
	def, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := def.Check("a robust regression model"); len(got) != 0 {
		t.Errorf("robust regression flagged, want protected: %v", got)
	}
	if got := def.Check("a robust plan"); len(got) == 0 {
		t.Errorf("bare robust should still be flagged")
	}
	p, err := ApplyPresets(DefaultProfile(), "cleaver")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, _ := s.Fix("optimal substructure beats a robust framework")
	want := "optimal substructure beats a solid framework"
	if got != want {
		t.Errorf("Fix = %q, want %q", got, want)
	}
}

// TestProtectQuotes checks that a profile with ProtectQuotes leaves quoted text unedited
// while still cleaning the prose around it, and that the default reaches inside quotes.
func TestProtectQuotes(t *testing.T) {
	t.Parallel()
	s, err := New(Profile{WordReplace: map[string]string{"leverage": "use"}, ProtectQuotes: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, _ := s.Fix(`We leverage it. She said "we leverage it" once.`)
	if want := `We use it. She said "we leverage it" once.`; got != want {
		t.Errorf("protected Fix = %q, want %q", got, want)
	}
	s2, err := New(Profile{WordReplace: map[string]string{"leverage": "use"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got2, _ := s2.Fix(`She said "we leverage it".`)
	if want := `She said "we use it".`; got2 != want {
		t.Errorf("unprotected Fix = %q, want %q", got2, want)
	}
}

// TestCleaverRecallPhrases checks the marketing collocations added to the cleaver preset are
// rewritten to plain phrasing.
func TestCleaverRecallPhrases(t *testing.T) {
	t.Parallel()
	p, err := ApplyPresets(DefaultProfile(), "cleaver")
	if err != nil {
		t.Fatalf("ApplyPresets: %v", err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, _ := s.Fix("This underscores the need; it resonates with users. Elevate your brand.")
	want := "This shows the need. It connects with users. Improve your brand."
	if got != want {
		t.Errorf("Fix = %q, want %q", got, want)
	}
}
