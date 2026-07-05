package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			err := run(context.Background(), opts, strings.NewReader(test.In), &out, &errb)
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
	if err := run(context.Background(), opts, strings.NewReader("a robust plan; it works"), &out, &errb); err != nil {
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
	if err := run(context.Background(), opts, strings.NewReader(""), &out, &errb); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a plan." {
		t.Errorf("file = %q, want %q", got, "a plan.")
	}
}
