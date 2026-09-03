package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// TestEvasionsAreClean is the property the whole report rests on: every replacement the
// attack reaches for must itself pass the default profile. An evasion that trips a rule
// is not an evasion, and a report claiming otherwise would be a lie told by a test that
// nobody wrote.
func TestEvasionsAreClean(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for from, to := range evasions {
		t.Run(from, func(t *testing.T) {
			t.Parallel()
			if findings := s.Check(to); len(findings) > 0 {
				t.Errorf("evasion %q -> %q still trips %s", from, to, findings[0].Rule)
			}
		})
	}
	// A dash appears both tight against its words and fenced by spaces, and the attacked
	// text has to come back clean either way.
	for from := range charEvasions {
		for _, context := range []string{"the plan" + from + "it shipped", "the plan " + from + " it shipped"} {
			res := s.Attack(context)
			if findings := s.Check(res.Text); len(findings) > 0 {
				t.Errorf("attacking %q left %q, which trips %s", context, res.Text, findings[0].Rule)
			}
		}
	}
}

// TestAttackEvadesWordsAndHoldsShapes checks the finding the command exists to report: a
// substitution beats a word list and loses to a sentence shape.
func TestAttackEvadesWordsAndHoldsShapes(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	text := "In summary, we leverage a myriad of robust tools to seamlessly delve into " +
		"the landscape. It's not just fast, it's smart. The best part? Here are three ways."
	res := s.Attack(text)

	if got := res.ByClass["word"]; got.Evaded == 0 || got.Resisted != 0 {
		t.Errorf("word class = %+v, want every one evaded", got)
	}
	if got := res.ByClass["structural"]; got.Resisted == 0 {
		t.Errorf("structural class = %+v, want shapes to hold", got)
	}
	for _, e := range res.Evasions {
		if strings.Contains(strings.ToLower(res.Text), strings.ToLower(e.Was)) {
			t.Errorf("%q survived its own evasion", e.Was)
		}
		if e.Now == "" {
			t.Errorf("evasion of %q produced nothing", e.Was)
		}
	}
	// The attacked text is a rewrite, not a deletion: it stays about as long as it was.
	if len(res.Text) < len(text)/2 {
		t.Errorf("attacked text lost half its length: %q", res.Text)
	}
}

// TestAttackReportsHonestly checks the bookkeeping a reader trusts: the survivors are
// really what a fresh check finds, and the class tally accounts for every finding.
func TestAttackReportsHonestly(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := s.Attack("In summary, we leverage robust synergy. It's not just fast, it's smart.")

	fresh := s.Check(res.Text)
	if len(fresh) != len(res.Survived) {
		t.Errorf("survived = %d, but a fresh check of the attacked text finds %d",
			len(res.Survived), len(fresh))
	}
	counted := 0
	for _, v := range res.ByClass {
		counted += v.Evaded + v.Resisted
	}
	if want := len(res.Evasions) + len(res.Survived); counted != want {
		t.Errorf("class tally counts %d, want %d", counted, want)
	}
	if res.ScoreAfter > res.ScoreBefore {
		t.Errorf("score rose under attack: %d -> %d", res.ScoreBefore, res.ScoreAfter)
	}
}

// TestAttackCases covers the inputs an attack has to survive without doing damage.
func TestAttackCases(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name         string
		In           string
		WantEvasions int
		WantSame     bool
	}{{ // Test 0: Clean prose has nothing to evade and comes back untouched.
		Name: "clean", In: "The mail came late and nobody minded.", WantEvasions: 0, WantSame: true,
	}, { // Test 1: Empty input is not a crash.
		Name: "empty", In: "", WantEvasions: 0, WantSame: true,
	}, { // Test 2: A word with no evasion in the table is left alone and reported as held.
		Name: "no evasion known", In: "We must action this item.", WantEvasions: 0,
	}, { // Test 3: Capitalization carries onto the replacement.
		Name: "capitalized", In: "Robust systems win.", WantEvasions: 1,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			res := s.Attack(test.In)
			if len(res.Evasions) != test.WantEvasions {
				t.Errorf("evasions = %d, want %d: %+v", len(res.Evasions), test.WantEvasions, res.Evasions)
			}
			if test.WantSame && res.Text != test.In {
				t.Errorf("text changed with nothing to evade: %q", res.Text)
			}
			if test.Name == "capitalized" && !strings.HasPrefix(res.Text, "Sturdy") {
				t.Errorf("text = %q, want the replacement to keep the capital", res.Text)
			}
		})
	}
}

// TestAttackOnCorpus records what an attack achieves against the labeled corpus, the
// number the score weighting rests on. It is a floor rather than an exact figure: the
// evasion table covers a fraction of the block list, so a fuller thesaurus would evade
// more words. What matters is the gap between the classes.
func TestAttackOnCorpus(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var word, structural ClassResult
	stillFlagged, total := 0, 0
	for _, p := range loadCorpus(t) {
		if p.Label != "ai" {
			continue
		}
		total++
		res := s.Attack(p.Text)
		if tellCount(res.Survived) > 0 {
			stillFlagged++
		}
		word.Evaded += res.ByClass["word"].Evaded
		word.Resisted += res.ByClass["word"].Resisted
		structural.Evaded += res.ByClass["structural"].Evaded
		structural.Resisted += res.ByClass["structural"].Resisted
	}
	t.Logf("still flagged after the attack: %d of %d ai passages", stillFlagged, total)
	t.Logf("word tells:       evaded %d, held %d", word.Evaded, word.Resisted)
	t.Logf("structural tells: evaded %d, held %d", structural.Evaded, structural.Resisted)

	// A sentence shape must be far harder to substitute away than a word. If this ever
	// inverts, the score weighting is wrong and the report should say so.
	wordEvasion := float64(word.Evaded) / float64(word.Evaded+word.Resisted)
	shapeEvasion := float64(structural.Evaded) / float64(structural.Evaded+structural.Resisted)
	if shapeEvasion >= wordEvasion {
		t.Errorf("shapes evaded %.2f of the time and words %.2f: structural tells are no "+
			"longer the harder class, so the double weight needs revisiting",
			shapeEvasion, wordEvasion)
	}
	// Most of the corpus should still be caught after a thesaurus pass, which is the
	// case for weighting shapes above words at all.
	if float64(stillFlagged)/float64(total) < 0.8 {
		t.Errorf("only %d of %d passages still flag after an attack, below the 0.8 floor",
			stillFlagged, total)
	}
}
