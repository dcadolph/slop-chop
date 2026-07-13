package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestFixAppliesSwapsOnce checks that content swaps apply exactly once, so a swap whose
// replacement contains its own trigger, a chain, or a cycle cannot feed on its own output.
func TestFixAppliesSwapsOnce(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Profile    Profile
		In         string
		WantResult string
	}{{ // Test 0: A self-referential swap expands once, not until the cap.
		Profile:    Profile{WordReplace: map[string]string{"use": "make use of"}},
		In:         "use it",
		WantResult: "make use of it",
	}, { // Test 1: A chain swaps each word once, it does not cascade a to b to c.
		Profile:    Profile{WordReplace: map[string]string{"happy": "glad", "glad": "cheerful"}},
		In:         "I am happy and glad.",
		WantResult: "I am glad and cheerful.",
	}, { // Test 2: A cycle terminates, each word swapped once rather than parity-dependent.
		Profile:    Profile{WordReplace: map[string]string{"foo": "bar", "bar": "foo"}},
		In:         "foo and bar",
		WantResult: "bar and foo",
	}, { // Test 3: A phrase whose replacement contains the phrase expands once.
		Profile:    Profile{PhraseReplace: map[string]string{"in order to": "in order to really"}},
		In:         "we did it in order to win",
		WantResult: "we did it in order to really win",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(test.Profile)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWordReplacePreservesValueCase checks that a replacement keeps the capitalization it
// was written with, so a swap to a term like GitHub is not flattened to lower case.
func TestWordReplacePreservesValueCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		WantResult string
	}{{ // Test 0: A lower-case match takes the value's own casing.
		In: "we host on github", WantResult: "we host on GitHub",
	}, { // Test 1: A leading-capital match keeps the value's internal casing.
		In: "Github is down", WantResult: "GitHub is down",
	}, { // Test 2: An all-caps match uppercases the value.
		In: "GITHUB outage", WantResult: "GITHUB outage",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{WordReplace: map[string]string{"github": "GitHub"}})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRegexReplaceKeepsContext checks that a regex swap expands against the original text,
// so a boundary anchor that depends on the preceding character still fires and a capture
// group resolves, where re-running the pattern on the isolated span would drop the context.
func TestRegexReplaceKeepsContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Patterns   map[string]string
		In         string
		WantResult string
	}{{ // Test 0: \B needs the preceding word character, lost when the span is isolated.
		Patterns:   map[string]string{`\Bfoo\b`: "BAR"},
		In:         "xfoo and foo",
		WantResult: "xBAR and foo",
	}, { // Test 1: A capture group resolves against the original text.
		Patterns:   map[string]string{`(\w+)@(\w+)`: "$2.$1"},
		In:         "user@host here",
		WantResult: "host.user here",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			s, err := New(Profile{RegexReplace: test.Patterns})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, _ := s.Fix(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCheckAgreesWithFix checks that when two rewrite rules match one span, the finding
// reports the swap Fix actually performs rather than a later rule's replacement.
func TestCheckAgreesWithFix(t *testing.T) {
	t.Parallel()
	p := Profile{
		WordReplace:  map[string]string{"leverage": "use"},
		RegexReplace: map[string]string{`\bleverage\b`: "employ"},
	}
	s, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := "we leverage it"
	out, findings := s.Fix(in)

	if diff := cmp.Diff("we use it", out); diff != "" {
		t.Errorf("fix mismatch (-want +got):\n%s", diff)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if findings[0].Replacement == nil || *findings[0].Replacement != "use" {
		t.Errorf("finding replacement = %v, want %q (the swap Fix performs)", findings[0].Replacement, "use")
	}
}

// TestFixSwapAfterSpaceCollapse checks that a swap keyed on a single space still fires when
// the input held a run of spaces, and that Fix stays idempotent across the interaction.
func TestFixSwapAfterSpaceCollapse(t *testing.T) {
	t.Parallel()
	s, err := New(Profile{
		CollapseSpaces: true,
		RegexReplace:   map[string]string{`([0-9]+) %`: "$1 percent"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	once, _ := s.Fix("0  %")
	twice, _ := s.Fix(once)
	if diff := cmp.Diff("0 percent", once); diff != "" {
		t.Errorf("fix mismatch (-want +got):\n%s", diff)
	}
	if once != twice {
		t.Errorf("not idempotent: once %q, twice %q", once, twice)
	}
}
