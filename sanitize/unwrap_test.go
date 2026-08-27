package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestUnwrapProse checks which line breaks join into a space and which stay, and that a
// joined copy is always the same byte length as its input so match offsets still line up.
func TestUnwrapProse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want string
	}{{ // Test 0: A wrap inside a paragraph joins.
		Name: "soft wrap", In: "Think\nof it as a tool.", Want: "Think of it as a tool.",
	}, { // Test 1: A blank line is a paragraph break and never joins.
		Name: "paragraph break", In: "One thing.\n\nAnother thing.", Want: "One thing.\n\nAnother thing.",
	}, { // Test 2: A heading below prose starts its own block.
		Name: "heading below", In: "Some prose\n# Heading", Want: "Some prose\n# Heading",
	}, { // Test 3: A heading above prose never runs into it.
		Name: "heading above", In: "# Heading\nSome prose", Want: "# Heading\nSome prose",
	}, { // Test 4: Two bullets stay two bullets.
		Name: "bullets", In: "- we may want this\n- it might work", Want: "- we may want this\n- it might work",
	}, { // Test 5: A bullet's own wrapped continuation joins.
		Name: "bullet continuation", In: "- a long point that\n  keeps going", Want: "- a long point that   keeps going",
	}, { // Test 6: An ordered list marker starts a block.
		Name: "ordered list", In: "first line\n2. second item", Want: "first line\n2. second item",
	}, { // Test 7: A number that is not a marker is prose and joins.
		Name: "bare number", In: "we shipped\n2 releases", Want: "we shipped 2 releases",
	}, { // Test 8: Emphasis at a line start is prose, not a bullet.
		Name: "emphasis not bullet", In: "the word\n*matters* here", Want: "the word *matters* here",
	}, { // Test 9: A fence never joins with the prose around it.
		Name: "fence", In: "run this\n```go", Want: "run this\n```go",
	}, { // Test 10: A table row stands alone.
		Name: "table row", In: "see below\n| a | b |", Want: "see below\n| a | b |",
	}, { // Test 11: A block quote starts its own block.
		Name: "block quote", In: "he said\n> a quote", Want: "he said\n> a quote",
	}, { // Test 12: Text with no break is returned as it is.
		Name: "no newline", In: "one line only", Want: "one line only",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := unwrapProse(test.In)
			if got != test.Want {
				t.Errorf("unwrapProse(%q) = %q, want %q", test.In, got, test.Want)
			}
			if len(got) != len(test.In) {
				t.Errorf("length changed: got %d, want %d; offsets would no longer line up", len(got), len(test.In))
			}
		})
	}
}

// TestStructuralRulesSurviveLineWrap checks that a structural tell is flagged whether it
// sits on one line or a hard wrap splits it, since prose in Markdown is usually wrapped.
func TestStructuralRulesSurviveLineWrap(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name     string
		OneLine  string
		Wrapped  string
		WantRule string
	}{{ // Test 0: The dive-in invitation, split after the verb.
		Name: "dive in", OneLine: "Let's dive into how indexes work.",
		Wrapped: "Let's dive\ninto how indexes work.", WantRule: "structural:lets-dive-in",
	}, { // Test 1: The analogy opener, split mid phrase.
		Name: "think of it as", OneLine: "Think of it as the index in a textbook.",
		Wrapped: "Think\nof it as the index in a textbook.", WantRule: "structural:think-of-it-as",
	}, { // Test 2: The setup and reveal, split before the payoff.
		Name: "comes in", OneLine: "That's where the INCLUDE clause comes in here.",
		Wrapped: "That's\nwhere the INCLUDE clause comes in here.", WantRule: "structural:thats-where-comes-in",
	}, { // Test 3: The enumeration opener.
		Name: "here are n", OneLine: "Here are three things to keep in mind.",
		Wrapped: "Here are three\nthings to keep in mind.", WantRule: "structural:here-are-n",
	}, { // Test 4: The audience-flattering frame, split across the or.
		Name: "whether youre", OneLine: "Whether you're a founder or an engineer, this applies.",
		Wrapped: "Whether you're a founder\nor an engineer, this applies.", WantRule: "structural:whether-youre",
	}, { // Test 5: The formal inversion.
		Name: "not only inversion", OneLine: "Not only does it compile faster, it uses less memory.",
		Wrapped: "Not only\ndoes it compile faster, it uses less memory.", WantRule: "structural:not-only-inversion",
	}, { // Test 6: A wrap with extra indentation on the continuation line.
		Name: "indented continuation", OneLine: "- Let's take a closer look at the plan.",
		Wrapped: "- Let's take a\n    closer look at the plan.", WantRule: "structural:lets-take-a-look",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if !hasRule(s.Check(test.OneLine), test.WantRule) {
				t.Fatalf("one line %q did not flag %s, so the wrapped case proves nothing", test.OneLine, test.WantRule)
			}
			findings := s.Check(test.Wrapped)
			if !hasRule(findings, test.WantRule) {
				t.Errorf("wrapped %q did not flag %s; got %v", test.Wrapped, test.WantRule, ruleNames(findings))
			}
			for _, f := range findings {
				if f.Rule != test.WantRule {
					continue
				}
				if got := test.Wrapped[f.Offset : f.Offset+len(f.Match)]; got != f.Match {
					t.Errorf("offset %d does not point at the match: text has %q, finding has %q", f.Offset, got, f.Match)
				}
			}
		})
	}
}

// TestUnwrapDoesNotJoinBlocks checks that joining wraps never lets one rule match a tell
// spanning two separate blocks, which would be a false positive the raw text never had.
func TestUnwrapDoesNotJoinBlocks(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name string
		In   string
	}{{ // Test 0: Two list items each holding one hedge are not one hedge stack.
		Name: "hedges in separate bullets", In: "- we may ship it\n- it might work\n",
	}, { // Test 1: A hedge either side of a paragraph break is not a stack.
		Name: "hedges across paragraphs", In: "It could work.\n\nWe might try.\n",
	}, { // Test 2: A heading and the line under it are not one sentence.
		Name: "heading then prose", In: "# Not just a title\nIt's a heading.\n",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			for _, f := range s.Check(test.In) {
				if strings.HasPrefix(f.Rule, "structural:") {
					t.Errorf("joined two blocks into a false positive: %s on %q", f.Rule, f.Match)
				}
			}
		})
	}
}

// hasRule reports whether findings hold one named rule.
func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// ruleNames lists the rule of every finding, for a test failure message.
func ruleNames(findings []Finding) []string {
	names := make([]string, len(findings))
	for i, f := range findings {
		names[i] = f.Rule
	}
	return names
}
