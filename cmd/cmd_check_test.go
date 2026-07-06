package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestCheckFindings checks that check returns errFindings on slop and nil on clean
// text.
func TestCheckFindings(t *testing.T) {
	tests := []struct {
		In   string
		Want error
	}{{ // Test 0: Slop returns the sentinel.
		In: "a robust plan", Want: errFindings,
	}, { // Test 1: Clean text returns nil.
		In: "a plain sentence", Want: nil,
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			_, _, err := runCLI(t, []string{"check"}, test.In)
			if !errors.Is(err, test.Want) {
				t.Errorf("err = %v, want %v", err, test.Want)
			}
		})
	}
}

// TestCheckFilePrefix checks that findings on a file carry the path, so terminals can
// make them clickable.
func TestCheckFilePrefix(t *testing.T) {
	path := writeTemp(t, t.TempDir(), "notes.md", "a robust plan")
	_, stderr, err := runCLI(t, []string{"check", path}, "")
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(stderr, path+":1:3") {
		t.Errorf("stderr = %q, want prefix %q", stderr, path+":1:3")
	}
}

// TestCheckMultiFile checks that every file is scanned, each finding carries its own
// path, and one dirty file fails the whole run.
func TestCheckMultiFile(t *testing.T) {
	dir := t.TempDir()
	clean := writeTemp(t, dir, "clean.md", "a plain sentence")
	dirty := writeTemp(t, dir, "dirty.md", "a robust plan")
	_, stderr, err := runCLI(t, []string{"check", clean, dirty}, "")
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(stderr, dirty+":1:3") {
		t.Errorf("stderr = %q, want a finding for %q", stderr, dirty)
	}
	if strings.Contains(stderr, clean+":") {
		t.Errorf("stderr = %q, want no findings for %q", stderr, clean)
	}
}

// TestCheckJSONFindings checks that check --json still exits through errFindings.
func TestCheckJSONFindings(t *testing.T) {
	out, _, err := runCLI(t, []string{"check", "--json"}, "a robust plan")
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(out, `"findings"`) {
		t.Errorf("stdout = %q, want findings JSON", out)
	}
}
