package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSourceFile checks the gate that keeps prose rules away from code and data files.
func TestSourceFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		WantResult bool
	}{{ // Test 0: Go source is code.
		In: "main.go", WantResult: true,
	}, { // Test 1: extension match is case-insensitive.
		In: "dir/APP.PY", WantResult: true,
	}, { // Test 2: a Makefile has no extension but is code.
		In: "sub/Makefile", WantResult: true,
	}, { // Test 3: YAML is machine-read data.
		In: "config.yaml", WantResult: true,
	}, { // Test 4: Markdown is prose.
		In: "README.md", WantResult: false,
	}, { // Test 5: plain text is prose.
		In: "notes.txt", WantResult: false,
	}, { // Test 6: an extensionless prose file passes.
		In: "README", WantResult: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := sourceFile(test.In); got != test.WantResult {
				t.Errorf("sourceFile(%q) = %v, want %v", test.In, got, test.WantResult)
			}
		})
	}
}

// TestCheckSkipsSource checks that check skips a source file with a warning instead of
// flagging its code as prose.
func TestCheckSkipsSource(t *testing.T) {
	dir := t.TempDir()
	goFile := writeTemp(t, dir, "main.go", "package main\n\n// A robust plan; really.\nfunc main() {}\n")
	_, stderr, err := runCLI(t, []string{"check", goFile}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil for a skipped source file", err)
	}
	if !strings.Contains(stderr, "skipping "+goFile) {
		t.Errorf("stderr = %q, want a skip warning for %q", stderr, goFile)
	}
}

// TestFixRefusesSource checks that fix to stdout refuses a source file outright, and that
// fix --write skips it and leaves the file byte for byte intact.
func TestFixRefusesSource(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc main() {\n\tif n := len(os.Args); n > 0 {\n\t\treturn\n\t}\n}\n"
	goFile := writeTemp(t, dir, "main.go", src)

	if _, _, err := runCLI(t, []string{"fix", goFile}, ""); err == nil {
		t.Fatal("fix on a source file returned nil, want an error")
	}

	_, stderr, err := runCLI(t, []string{"fix", "--write", goFile}, "")
	if err != nil {
		t.Fatalf("fix --write err = %v, want nil with the file skipped", err)
	}
	if !strings.Contains(stderr, "skipping "+goFile) {
		t.Errorf("stderr = %q, want a skip warning for %q", stderr, goFile)
	}
	got, err := os.ReadFile(filepath.Clean(goFile))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != src {
		t.Errorf("fix --write changed a source file:\n got %q\nwant %q", got, src)
	}
}

// TestScoreSkipsSource checks that score skips a source file rather than rating code.
func TestScoreSkipsSource(t *testing.T) {
	dir := t.TempDir()
	goFile := writeTemp(t, dir, "gen.go", "package gen\n\nvar x = 1\n")
	stdout, stderr, err := runCLI(t, []string{"score", goFile}, "")
	if err != nil && !errors.Is(err, errFindings) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(stdout, goFile+":") {
		t.Errorf("stdout = %q, want no score line for a skipped file", stdout)
	}
	if !strings.Contains(stderr, "skipping "+goFile) {
		t.Errorf("stderr = %q, want a skip warning for %q", stderr, goFile)
	}
}
