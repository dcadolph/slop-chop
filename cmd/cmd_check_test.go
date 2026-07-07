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

// TestCheckMissingFile checks that a missing input file is a read error, not a crash.
func TestCheckMissingFile(t *testing.T) {
	_, _, err := runCLI(t, []string{"check", "/no/such/file.md"}, "")
	if err == nil || !strings.Contains(err.Error(), "read file") {
		t.Errorf("err = %v, want a read-file error", err)
	}
}

// TestCheckMalformedProfile checks that a profile that is not valid JSON is an error.
func TestCheckMalformedProfile(t *testing.T) {
	bad := writeTemp(t, t.TempDir(), "bad.json", "{not json")
	_, _, err := runCLI(t, []string{"check", "--profile", bad}, "some text")
	if err == nil || !strings.Contains(err.Error(), "profile decode") {
		t.Errorf("err = %v, want a profile-decode error", err)
	}
}

// TestCheckMissingProfile checks that a profile path that does not exist is an error.
func TestCheckMissingProfile(t *testing.T) {
	_, _, err := runCLI(t, []string{"check", "--profile", "/no/such/profile.json"}, "some text")
	if err == nil || !strings.Contains(err.Error(), "profile open") {
		t.Errorf("err = %v, want a profile-open error", err)
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
