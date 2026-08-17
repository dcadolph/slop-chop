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
		{Name: "fragment reveal", In: "The best part? It runs offline.", WantRule: "structural:fragment-reveal", WantHit: true},
		{Name: "here are n", In: "Here are five ways to speed up your build.", WantRule: "structural:here-are-n", WantHit: true},
		{Name: "whether youre", In: "Whether you're a beginner or an expert, this fits.", WantRule: "structural:whether-youre", WantHit: true},
		{Name: "think of it as", In: "Think of it as a compiler for prose.", WantRule: "structural:think-of-it-as", WantHit: true},
		{Name: "not just split", In: "You're not just buying a tool. You're joining a movement.", WantRule: "structural:not-just-sentence-split", WantHit: true},
		{Name: "spaced hyphen", In: "The build is quick - a relief after the rewrite.", WantRule: "structural:spaced-hyphen", WantHit: true},
		{Name: "signoff mid line", In: "Check the docs for more detail. I hope this helps!", WantRule: "structural:assistant-signoff", WantHit: true},
		{Name: "in an era", In: "In an era where attention is scarce, speed sells.", WantRule: "structural:in-an-era", WantHit: true},
		{Name: "bold bullet run", In: "- **Speed:** fast builds\n- **Cost:** free forever\n- **Care:** none needed", WantRule: "structural:bold-bullet-run", WantHit: true},
		{Name: "not only inversion", In: "Not only does it compile faster, it uses less memory.", WantRule: "structural:not-only-inversion", WantHit: true},
		{Name: "plays a role", In: "Caching plays a crucial role in performance.", WantRule: "structural:plays-a-role", WantHit: true},
		{Name: "conclusion heading", In: "## Conclusion\nThat covers the basics.", WantRule: "structural:conclusion-heading", WantHit: true},
		{Name: "emoji bullet", In: "- 🚀 Ship faster with zero config", WantRule: "structural:emoji-decoration", WantHit: true},
		{Name: "emoji bold heading", In: "## 💡 **Pro tips**", WantRule: "structural:emoji-decoration", WantHit: true},
		{Name: "plain prose", In: "The report is due on Friday afternoon.", WantHit: false},
		{Name: "plain heading", In: "## Setup\nInstall the binary first.", WantRule: "structural:conclusion-heading", WantHit: false},
		{Name: "emoji mid sentence", In: "The launch went well 🎉 and everyone relaxed.", WantRule: "structural:emoji-decoration", WantHit: false},
		{Name: "plain bullets", In: "- eggs\n- milk\n- bread", WantRule: "structural:bold-bullet-run", WantHit: false},
		{Name: "two bold bullets", In: "- **Speed:** fast builds\n- **Cost:** free forever", WantRule: "structural:bold-bullet-run", WantHit: false},
		{Name: "whether or not", In: "Whether you're coming or not, save me a seat.", WantRule: "structural:whether-youre", WantHit: false},
		{Name: "hyphen bullet line", In: "Bring these:\n- a folding chair\n- b vitamins", WantRule: "structural:spaced-hyphen", WantHit: false},
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

// TestExpandedRecall checks that the tells added to the default profile are caught: the
// stock connectors, the spelled-out "this is not X, it's Y" form, the "let's take a look"
// invitation, and a chatbot reply opener.
func TestExpandedRecall(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for testNum, in := range []string{
		"Furthermore, the results held.",             // Test 0: stock connector.
		"It is important to note that latency wins.", // Test 1: hedging opener.
		"That being said, we shipped it.",            // Test 2: pivot filler.
		"This is not just fast, it's reliable.",      // Test 3: spelled-out negative parallelism.
		"Let's take a closer look at the data.",      // Test 4: the look invitation.
		"Certainly! Here is the plan.",               // Test 5: chatbot opener.
	} {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := s.Check(in); len(got) == 0 {
				t.Errorf("Check(%q) found no tell, want at least one", in)
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
