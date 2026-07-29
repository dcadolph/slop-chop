package sanitize

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestMatchCase checks that a replacement takes on the capitalization of the match.
func TestMatchCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Match      string
		Repl       string
		WantResult string
	}{{ // Test 0: All lower case stays lower.
		Match: "behaviour", Repl: "behavior", WantResult: "behavior",
	}, { // Test 1: A leading capital carries over.
		Match: "Behaviour", Repl: "behavior", WantResult: "Behavior",
	}, { // Test 2: All caps carries over.
		Match: "BEHAVIOUR", Repl: "behavior", WantResult: "BEHAVIOR",
	}, { // Test 3: A lower-case first letter with inner caps is left plain.
		Match: "behavioUr", Repl: "behavior", WantResult: "behavior",
	}, { // Test 4: A capital first letter with inner caps title-cases.
		Match: "BeHaViour", Repl: "behavior", WantResult: "Behavior",
	}, { // Test 5: An empty match returns the replacement unchanged.
		Match: "", Repl: "x", WantResult: "x",
	}, { // Test 6: An empty replacement stays empty.
		Match: "x", Repl: "", WantResult: "",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := matchCase(test.Match, test.Repl)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDialectFix checks the spelling pass in both directions, including case handling,
// one-way pairs, word boundaries, and protected code.
func TestDialectFix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		Dialect    Dialect
		WantResult string
	}{{ // Test 0: American rewrites a British spelling.
		In: "the behaviour", Dialect: DialectAmerican, WantResult: "the behavior",
	}, { // Test 1: A leading capital survives the swap.
		In: "Behaviour matters", Dialect: DialectAmerican, WantResult: "Behavior matters",
	}, { // Test 2: All caps survives the swap.
		In: "BEHAVIOUR", Dialect: DialectAmerican, WantResult: "BEHAVIOR",
	}, { // Test 3: Several words in one line.
		In: "the colour we organise", Dialect: DialectAmerican, WantResult: "the color we organize",
	}, { // Test 4: British rewrites an American spelling.
		In: "the behavior", Dialect: DialectBritish, WantResult: "the behaviour",
	}, { // Test 5: British swaps an American -ize spelling.
		In: "we organize it", Dialect: DialectBritish, WantResult: "we organise it",
	}, { // Test 6: British leaves one-way homographs alone.
		In: "check the tire while you can", Dialect: DialectBritish,
		WantResult: "check the tire while you can",
	}, { // Test 7: American still applies a one-way pair.
		In: "a cheque", Dialect: DialectAmerican, WantResult: "a check",
	}, { // Test 8: Off leaves the text alone.
		In: "the colour", Dialect: DialectOff, WantResult: "the colour",
	}, { // Test 9: Text already in the target dialect is untouched.
		In: "the color", Dialect: DialectAmerican, WantResult: "the color",
	}, { // Test 10: A word boundary keeps a longer word from matching.
		In: "recolour", Dialect: DialectAmerican, WantResult: "recolour",
	}, { // Test 11: An inline code span is protected.
		In: "colour `colour` end", Dialect: DialectAmerican, WantResult: "color `colour` end",
	}, { // Test 12: A longer entry wins over a shorter one it starts with.
		In: "colourful", Dialect: DialectAmerican, WantResult: "colorful",
	}, { // Test 13: A possessive keeps its suffix.
		In: "the colour's hue", Dialect: DialectAmerican, WantResult: "the color's hue",
	}, { // Test 14: A hyphen bounds the match.
		In: "colour-coded", Dialect: DialectAmerican, WantResult: "color-coded",
	}, { // Test 15: Every foreign word in a line is swapped.
		In: "the colour; behaviour matters", Dialect: DialectAmerican,
		WantResult: "the color; behavior matters",
	}, { // Test 16: A word spelled the same in both dialects is left alone.
		In: "advertise the size", Dialect: DialectAmerican, WantResult: "advertise the size",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{Dialect: test.Dialect})
			if err != nil {
				t.Fatalf("New(%q): %v", test.Dialect, err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDialectRoundTrip checks that a bidirectional word survives a trip to American and
// back to British unchanged, so the two directions are true inverses for those pairs.
func TestDialectRoundTrip(t *testing.T) {
	t.Parallel()
	in := "colour behaviour organise centre defence catalogue theatre fibre"
	us, err := New(Profile{Dialect: DialectAmerican})
	if err != nil {
		t.Fatalf("New american: %v", err)
	}
	uk, err := New(Profile{Dialect: DialectBritish})
	if err != nil {
		t.Fatalf("New british: %v", err)
	}
	american, _ := us.Fix(in)
	back, _ := uk.Fix(american)
	if diff := cmp.Diff(in, back); diff != "" {
		t.Errorf("round trip changed the text (-want +got):\n%s", diff)
	}
}

// TestDialectWithDefaultProfile checks that the spelling pass composes with the built-in
// rules, so a dialect chosen alongside the defaults cleans spelling, punctuation, and
// semicolons in one run.
func TestDialectWithDefaultProfile(t *testing.T) {
	t.Parallel()
	p := DefaultProfile()
	p.Dialect = DialectAmerican
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, _ := s.Fix("In summary, the colour—behaviour; it works")
	want := "The color, behavior. It works"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestDialectCheck checks that the spelling pass reports a finding with the rule name and
// the suggested replacement.
func TestDialectCheck(t *testing.T) {
	t.Parallel()
	s, err := New(Profile{Dialect: DialectAmerican})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	findings := s.Check("the behaviour here")
	want := "behavior"
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Rule != "spelling" || f.Match != "behaviour" || f.Replacement == nil || *f.Replacement != want {
		t.Errorf("finding = %+v, want spelling behaviour -> %q", f, want)
	}
}

// TestDialectUnknown checks that an unknown dialect is an error from New.
func TestDialectUnknown(t *testing.T) {
	t.Parallel()
	_, err := New(Profile{Dialect: "klingon"})
	if !errors.Is(err, ErrDialect) {
		t.Errorf("err = %v, want ErrDialect", err)
	}
}

// TestDialectDisableValues checks the values that disable the pass and that recognized
// names are matched without regard to case.
func TestDialectDisableValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Dialect    Dialect
		WantResult string
	}{{ // Test 0: Empty is off.
		Dialect: "", WantResult: "the colour",
	}, { // Test 1: "off" is off.
		Dialect: "off", WantResult: "the colour",
	}, { // Test 2: "none" is off.
		Dialect: "none", WantResult: "the colour",
	}, { // Test 3: Mixed-case "OFF" is off.
		Dialect: "OFF", WantResult: "the colour",
	}, { // Test 4: Mixed-case "American" is recognized.
		Dialect: "American", WantResult: "the color",
	}, { // Test 5: Upper-case "BRITISH" is recognized.
		Dialect: "BRITISH", WantResult: "the colour",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{Dialect: test.Dialect})
			if err != nil {
				t.Fatalf("New(%q): %v", test.Dialect, err)
			}
			got, _ := s.Fix("the colour")
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSpellingWordList checks that the embedded word list is well formed: no empty or
// duplicate entries, both spellings differ, everything is lower case, and each dialect
// compiles into a rule.
func TestSpellingWordList(t *testing.T) {
	t.Parallel()
	if len(spellingPairs) == 0 {
		t.Fatal("spellingPairs is empty")
	}
	seenBritish := make(map[string]bool, len(spellingPairs))
	seenAmerican := make(map[string]bool, len(spellingPairs))
	for i, p := range spellingPairs {
		switch {
		case p.British == "" || p.American == "":
			t.Errorf("pair %d: empty field: %+v", i, p)
		case p.British == p.American:
			t.Errorf("pair %d: british == american: %q", i, p.British)
		case p.British != strings.ToLower(p.British) || p.American != strings.ToLower(p.American):
			t.Errorf("pair %d: not lower case: %+v", i, p)
		}
		if seenBritish[p.British] {
			t.Errorf("pair %d: duplicate british %q", i, p.British)
		}
		seenBritish[p.British] = true
		// A bidirectional pair supplies the British direction's lookup key, so its American
		// spelling must be unique. One-way pairs never key that map, so they may repeat.
		if !p.OneWay {
			if seenAmerican[p.American] {
				t.Errorf("pair %d: duplicate british-safe american %q", i, p.American)
			}
			seenAmerican[p.American] = true
		}
	}
	for _, d := range []Dialect{DialectAmerican, DialectBritish} {
		if _, ok, err := spellingRule(d); err != nil || !ok {
			t.Errorf("spellingRule(%q): ok = %v, err = %v", d, ok, err)
		}
	}
}
