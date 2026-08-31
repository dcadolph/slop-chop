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

// TestCheckScansComments checks that check on a source file reads the comments and only
// the comments: a tell in a comment is flagged at its real position, while the same word
// as an identifier, a tell inside a string literal, and gofmt alignment spacing draw
// nothing.
func TestCheckScansComments(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n" +
		"\n" +
		"// This delivers a comprehensive solution.\n" +
		"func main() {\n" +
		"\tcomprehensive := 1     // twelve\n" +
		"\ts := \"a robust plan\"\n" +
		"\t_, _ = comprehensive, s\n" +
		"}\n"
	goFile := writeTemp(t, dir, "main.go", src)
	_, stderr, err := runCLI(t, []string{"check", goFile}, "")
	if !errors.Is(err, errFindings) {
		t.Fatalf("err = %v, want errFindings for a tell in a comment", err)
	}
	if !strings.Contains(stderr, goFile+":3:") || !strings.Contains(stderr, "word:comprehensive") {
		t.Errorf("stderr = %q, want word:comprehensive reported on line 3", stderr)
	}
	if strings.Contains(stderr, "word:robust") {
		t.Errorf("stderr = %q, want no finding for a tell inside a string literal", stderr)
	}
	if strings.Contains(stderr, "double-space") {
		t.Errorf("stderr = %q, want no cleanup findings on a source file", stderr)
	}
	if strings.Contains(stderr, goFile+":5:2") {
		t.Errorf("stderr = %q, want no finding for the identifier on line 5", stderr)
	}
}

// TestCheckSkipsDataFile checks that a file type with no comments still skips with a
// warning.
func TestCheckSkipsDataFile(t *testing.T) {
	dir := t.TempDir()
	jsonFile := writeTemp(t, dir, "cfg.json", "{\"note\": \"a robust plan\"}\n")
	_, stderr, err := runCLI(t, []string{"check", jsonFile}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil for a skipped data file", err)
	}
	if !strings.Contains(stderr, "skipping "+jsonFile) {
		t.Errorf("stderr = %q, want a skip warning for %q", stderr, jsonFile)
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

// TestScoreScansComments checks that score on a source file rates its comments, so a
// slop-commented file scores above a tersely commented one.
func TestScoreScansComments(t *testing.T) {
	dir := t.TempDir()
	sloppy := writeTemp(t, dir, "sloppy.go",
		"package gen\n\n// In summary, this comprehensive solution leverages robust synergy.\nvar x = 1\n")
	terse := writeTemp(t, dir, "terse.go",
		"package gen\n\n// x counts retries.\nvar x = 1\n")
	stdout, _, err := runCLI(t, []string{"score", sloppy, terse}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil without a gate", err)
	}
	if !strings.Contains(stdout, sloppy+": ") || !strings.Contains(stdout, terse+": 0") {
		t.Errorf("stdout = %q, want a high score for the sloppy comments and 0 for the terse ones", stdout)
	}
}
