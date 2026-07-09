package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestFlagPatterns checks that the built-in structural patterns flag their tells without
// rewriting them, and that ordinary prose is left alone.
func TestFlagPatterns(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		Name     string
		In       string
		WantRule string
		WantHit  bool
	}{
		{Name: "not just but", In: "It's not just fast, it's smart.", WantRule: "structural:its-not-x-its-y", WantHit: true},
		{Name: "not only but also", In: "This is not only fast but also cheap.", WantRule: "structural:not-just-but-also", WantHit: true},
		{Name: "dive in", In: "Let's dive into the topic.", WantRule: "structural:lets-dive-in", WantHit: true},
		{Name: "heres the thing", In: "Here's the thing: it works.", WantRule: "structural:heres-the-thing", WantHit: true},
		{Name: "comes in", In: "That's where caching comes in.", WantRule: "structural:thats-where-comes-in", WantHit: true},
		{Name: "plain prose", In: "The report is due on Friday afternoon.", WantHit: false},
	}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			findings := s.Check(test.In)
			hit := false
			for _, f := range findings {
				if !strings.HasPrefix(f.Rule, "structural:") {
					continue
				}
				if f.Replacement != nil {
					t.Errorf("structural finding %q has a replacement, want flag-only", f.Rule)
				}
				if f.Rule == test.WantRule {
					hit = true
				}
			}
			if hit != test.WantHit {
				t.Errorf("rule %q hit = %v, want %v (findings %v)", test.WantRule, hit, test.WantHit, findings)
			}
		})
	}
}

// TestSemicolonInParens checks that a semicolon inside parentheses is treated as a list
// separator and left alone, not split into a new sentence.
func TestSemicolonInParens(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := "The results held (red; green; blue) across every run."
	got, _ := s.Fix(in)
	if got != in {
		t.Errorf("Fix split a parenthetical list:\n got %q\nwant %q", got, in)
	}
}
