package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestParseArgs checks mode selection, flag validation, and the help path.
func TestParseArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         []string
		WantErrSub string
		WantHelp   bool
	}{{ // Test 0: No arguments is an error.
		In: nil, WantErrSub: "missing mode",
	}, { // Test 1: help returns the help sentinel.
		In: []string{"help"}, WantHelp: true,
	}, { // Test 2: -h returns the help sentinel.
		In: []string{"-h"}, WantHelp: true,
	}, { // Test 3: --help returns the help sentinel.
		In: []string{"--help"}, WantHelp: true,
	}, { // Test 4: An unknown mode is an error.
		In: []string{"mangle"}, WantErrSub: "unknown mode",
	}, { // Test 5: check with a file parses.
		In: []string{"check", "notes.md"},
	}, { // Test 6: fix -w with a file parses.
		In: []string{"fix", "-w", "notes.md"},
	}, { // Test 7: -w with -json is rejected.
		In: []string{"fix", "-w", "-json", "notes.md"}, WantErrSub: "-w with -json",
	}, { // Test 8: -w without a file is rejected.
		In: []string{"fix", "-w"}, WantErrSub: "not stdin",
	}, { // Test 9: check rejects -w.
		In: []string{"check", "-w", "notes.md"}, WantErrSub: "fix flag",
	}, { // Test 10: check rejects -rewrite.
		In: []string{"check", "-rewrite", "notes.md"}, WantErrSub: "fix flag",
	}, { // Test 11: check rejects -model.
		In: []string{"check", "-model", "m", "notes.md"}, WantErrSub: "fix flag",
	}, { // Test 12: -model without -rewrite is rejected.
		In: []string{"fix", "-model", "m", "notes.md"}, WantErrSub: "-model needs -rewrite",
	}, { // Test 13: fix to stdout takes one file at most.
		In: []string{"fix", "a.md", "b.md"}, WantErrSub: "pass -w",
	}, { // Test 14: An unknown flag is an error.
		In: []string{"check", "-bogus"}, WantErrSub: "bogus",
	}, { // Test 15: check takes any number of files.
		In: []string{"check", "a.md", "b.md", "c.md"},
	}, { // Test 16: fix -w takes any number of files.
		In: []string{"fix", "-w", "a.md", "b.md"},
	}, { // Test 17: JSON output takes one file at most.
		In: []string{"check", "-json", "a.md", "b.md"}, WantErrSub: "-json takes at most one file",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			_, err := parseArgs(test.In)
			switch {
			case test.WantHelp:
				if !errors.Is(err, flag.ErrHelp) {
					t.Errorf("err = %v, want flag.ErrHelp", err)
				}
			case test.WantErrSub != "":
				if err == nil || !strings.Contains(err.Error(), test.WantErrSub) {
					t.Errorf("err = %v, want substring %q", err, test.WantErrSub)
				}
			default:
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}
		})
	}
}

// TestParseArgsFields checks that parsed flags land in the right options.
func TestParseArgsFields(t *testing.T) {
	t.Parallel()
	opts, err := parseArgs([]string{"fix", "-rewrite", "-model", "claude-x", "-pretty", "-json", "in.md"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := runOptions{
		mode: "fix", files: []string{"in.md"},
		jsonOut: true, pretty: true, rewrite: true, model: "claude-x",
	}
	if diff := cmp.Diff(want, opts, cmp.AllowUnexported(runOptions{})); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestRunCheckFindings checks that check mode returns errFindings on slop and nil on
// clean text. This is only testable because run no longer calls os.Exit.
func TestRunCheckFindings(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			var out, errb bytes.Buffer
			opts := runOptions{mode: "check"}
			err := run(t.Context(), opts, strings.NewReader(test.In), &out, &errb)
			if !errors.Is(err, test.Want) {
				t.Errorf("err = %v, want %v", err, test.Want)
			}
		})
	}
}

// TestRunFixStdout checks that fix writes cleaned text to stdout and leaves any file
// untouched.
func TestRunFixStdout(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	opts := runOptions{mode: "fix"}
	if err := run(t.Context(), opts, strings.NewReader("a robust plan; it works"), &out, &errb); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "a robust plan. It works" {
		t.Errorf("stdout = %q", got)
	}
}

// TestRunFixWrite checks that -w rewrites the file in place.
func TestRunFixWrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("In summary, a plan."), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	opts := runOptions{mode: "fix", files: []string{path}, write: true}
	if err := run(t.Context(), opts, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A plan." {
		t.Errorf("file = %q, want %q", got, "A plan.")
	}
}

// TestRunCheckFilePrefix checks that findings on a file carry the path, so terminals
// can make them clickable.
func TestRunCheckFilePrefix(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("a robust plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	opts := runOptions{mode: "check", files: []string{path}}
	if err := run(t.Context(), opts, strings.NewReader(""), &out, &errb); !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(errb.String(), path+":1:3") {
		t.Errorf("stderr = %q, want prefix %q", errb.String(), path+":1:3")
	}
}

// TestRunCheckMultiFile checks that every file is scanned, each finding carries its own
// path, and one dirty file fails the whole run.
func TestRunCheckMultiFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.md")
	dirty := filepath.Join(dir, "dirty.md")
	if err := os.WriteFile(clean, []byte("a plain sentence"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirty, []byte("a robust plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	opts := runOptions{mode: "check", files: []string{clean, dirty}}
	if err := run(t.Context(), opts, strings.NewReader(""), &out, &errb); !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(errb.String(), dirty+":1:3") {
		t.Errorf("stderr = %q, want a finding for %q", errb.String(), dirty)
	}
	if strings.Contains(errb.String(), clean+":") {
		t.Errorf("stderr = %q, want no findings for %q", errb.String(), clean)
	}
}

// TestRunFixWriteMultiFile checks that -w rewrites every listed file in place.
func TestRunFixWriteMultiFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	one := filepath.Join(dir, "one.md")
	two := filepath.Join(dir, "two.md")
	if err := os.WriteFile(one, []byte("In summary, a plan."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("it works; it ships"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	opts := runOptions{mode: "fix", files: []string{one, two}, write: true}
	if err := run(t.Context(), opts, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("run: %v", err)
	}
	gotOne, err := os.ReadFile(one)
	if err != nil {
		t.Fatal(err)
	}
	gotTwo, err := os.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOne) != "A plan." {
		t.Errorf("one = %q, want %q", gotOne, "A plan.")
	}
	if string(gotTwo) != "it works. It ships" {
		t.Errorf("two = %q, want %q", gotTwo, "it works. It ships")
	}
}

// TestRunProfileDiscovery checks that a .slop-chop.json in the working directory is
// picked up when -profile is not set, and that -profile still wins over it.
func TestRunProfileDiscovery(t *testing.T) {
	dir := t.TempDir()
	discovered := `{"blockWords": ["zorp"]}`
	if err := os.WriteFile(filepath.Join(dir, ".slop-chop.json"), []byte(discovered), 0o644); err != nil {
		t.Fatal(err)
	}
	flagged := filepath.Join(dir, "other.json")
	if err := os.WriteFile(flagged, []byte(`{"blockWords": ["blimp"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// The discovered profile flags zorp and nothing else.
	var out, errb bytes.Buffer
	opts := runOptions{mode: "check"}
	err := run(t.Context(), opts, strings.NewReader("a zorp plan"), &out, &errb)
	if !errors.Is(err, errFindings) {
		t.Fatalf("discovered profile: err = %v, want errFindings", err)
	}
	if err := run(t.Context(), opts, strings.NewReader("a robust plan"), &out, &errb); err != nil {
		t.Fatalf("discovered profile dropped the defaults: err = %v, want nil", err)
	}

	// An explicit -profile wins over the discovered file.
	opts.profilePath = flagged
	if err := run(t.Context(), opts, strings.NewReader("a zorp plan"), &out, &errb); err != nil {
		t.Fatalf("explicit profile: err = %v, want nil", err)
	}
	err = run(t.Context(), opts, strings.NewReader("a blimp plan"), &out, &errb)
	if !errors.Is(err, errFindings) {
		t.Fatalf("explicit profile: err = %v, want errFindings", err)
	}
}

// TestRunCheckJSONFindings checks that check -json still exits through errFindings.
func TestRunCheckJSONFindings(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	opts := runOptions{mode: "check", jsonOut: true}
	err := run(t.Context(), opts, strings.NewReader("a robust plan"), &out, &errb)
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(out.String(), `"findings"`) {
		t.Errorf("stdout = %q, want findings JSON", out.String())
	}
}

// TestRunFixRewriteNewline checks that the trailing newline survives the rewrite pass,
// which trims the model reply.
func TestRunFixRewriteNewline(t *testing.T) {
	old := rewritePass
	rewritePass = func(_ context.Context, _ string, _ []string, _ string) (string, error) {
		return "clean text", nil
	}
	t.Cleanup(func() { rewritePass = old })

	tests := []struct {
		In         string
		WantResult string
	}{{ // Test 0: Input ending in a newline gets it back.
		In: "dirty text\n", WantResult: "clean text\n",
	}, { // Test 1: Input with no trailing newline stays without one.
		In: "dirty text", WantResult: "clean text",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			var out, errb bytes.Buffer
			opts := runOptions{mode: "fix", rewrite: true}
			if err := run(t.Context(), opts, strings.NewReader(test.In), &out, &errb); err != nil {
				t.Fatalf("run: %v", err)
			}
			if out.String() != test.WantResult {
				t.Errorf("stdout = %q, want %q", out.String(), test.WantResult)
			}
		})
	}
}
