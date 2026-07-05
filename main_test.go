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
	}, { // Test 13: More than one file is an error.
		In: []string{"fix", "a.md", "b.md"}, WantErrSub: "too many arguments",
	}, { // Test 14: An unknown flag is an error.
		In: []string{"check", "-bogus"}, WantErrSub: "bogus",
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
		mode: "fix", file: "in.md", jsonOut: true, pretty: true, rewrite: true, model: "claude-x",
	}
	if opts != want {
		t.Errorf("opts = %+v, want %+v", opts, want)
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
	opts := runOptions{mode: "fix", file: path, write: true}
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
	opts := runOptions{mode: "check", file: path}
	if err := run(t.Context(), opts, strings.NewReader(""), &out, &errb); !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings", err)
	}
	if !strings.Contains(errb.String(), path+":1:3") {
		t.Errorf("stderr = %q, want prefix %q", errb.String(), path+":1:3")
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
