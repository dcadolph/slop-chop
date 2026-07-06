package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestFixStdout checks that fix writes cleaned text to stdout.
func TestFixStdout(t *testing.T) {
	out, _, err := runCLI(t, []string{"fix"}, "a robust plan; it works")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "a robust plan. It works" {
		t.Errorf("stdout = %q", out)
	}
}

// TestFixWrite checks that --write rewrites the file in place.
func TestFixWrite(t *testing.T) {
	path := writeTemp(t, t.TempDir(), "notes.md", "In summary, a plan.")
	if _, _, err := runCLI(t, []string{"fix", "-w", path}, ""); err != nil {
		t.Fatalf("fix: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A plan." {
		t.Errorf("file = %q, want %q", got, "A plan.")
	}
}

// TestFixWriteMultiFile checks that --write rewrites every listed file in place.
func TestFixWriteMultiFile(t *testing.T) {
	dir := t.TempDir()
	one := writeTemp(t, dir, "one.md", "In summary, a plan.")
	two := writeTemp(t, dir, "two.md", "it works; it ships")
	if _, _, err := runCLI(t, []string{"fix", "-w", one, two}, ""); err != nil {
		t.Fatalf("fix: %v", err)
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

// TestFixRewriteNewline checks that the trailing newline survives the rewrite pass,
// which trims the model reply.
func TestFixRewriteNewline(t *testing.T) {
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
			out, _, err := runCLI(t, []string{"fix", "--rewrite"}, test.In)
			if err != nil {
				t.Fatalf("fix: %v", err)
			}
			if out != test.WantResult {
				t.Errorf("stdout = %q, want %q", out, test.WantResult)
			}
		})
	}
}
