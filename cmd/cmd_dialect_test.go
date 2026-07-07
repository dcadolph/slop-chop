package cmd

import (
	"errors"
	"strings"
	"testing"
)

// TestFixDialectFlag checks that --dialect rewrites spellings in both directions.
func TestFixDialectFlag(t *testing.T) {
	tests := []struct {
		In         string
		Dialect    string
		WantResult string
	}{{ // Test 0: American rewrites a British spelling.
		In: "the colour", Dialect: "american", WantResult: "the color",
	}, { // Test 1: British rewrites an American spelling.
		In: "the color", Dialect: "british", WantResult: "the colour",
	}, { // Test 2: Off leaves the text alone.
		In: "the colour", Dialect: "off", WantResult: "the colour",
	}}

	for _, test := range tests {
		out, _, err := runCLI(t, []string{"fix", "--dialect", test.Dialect}, test.In)
		if err != nil {
			t.Fatalf("dialect %q: err = %v, want nil", test.Dialect, err)
		}
		if out != test.WantResult {
			t.Errorf("dialect %q: out = %q, want %q", test.Dialect, out, test.WantResult)
		}
	}
}

// TestCheckDialectFlag checks that check flags a foreign spelling and exits through
// errFindings.
func TestCheckDialectFlag(t *testing.T) {
	_, stderr, err := runCLI(t, []string{"check", "--dialect", "american"}, "the colour")
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(stderr, "spelling") {
		t.Errorf("stderr = %q, want a spelling finding", stderr)
	}
}

// TestDialectBadValue checks that an unknown dialect is a clear error.
func TestDialectBadValue(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "--dialect", "klingon"}, "text")
	if err == nil || !strings.Contains(err.Error(), "unknown dialect") {
		t.Errorf("err = %v, want an unknown-dialect error", err)
	}
}

// TestDialectProfilePin checks that a profile can pin a dialect and that the flag overrides
// it, so a repo default stands until a run asks for something else.
func TestDialectProfilePin(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, ".slop-chop.json", `{"dialect": "american"}`)
	t.Chdir(dir)

	// The pinned dialect flags a British spelling with no flag passed.
	if _, _, err := runCLI(t, []string{"check"}, "the colour"); !errors.Is(err, errFindings) {
		t.Fatalf("pinned dialect: err = %v, want errFindings", err)
	}
	// An American spelling passes under the pinned American dialect.
	if _, _, err := runCLI(t, []string{"check"}, "the color"); err != nil {
		t.Fatalf("pinned dialect: err = %v, want nil", err)
	}
	// The flag overrides the pin: off disables the pass even though the profile pins one.
	if _, _, err := runCLI(t, []string{"check", "--dialect", "off"}, "the colour"); err != nil {
		t.Fatalf("flag override: err = %v, want nil", err)
	}
}
