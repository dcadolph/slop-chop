package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/cmd/config"
)

// Tests in this package run serially because flag state lives in package-global
// pflag.Flag values shared by every command build.

// runCLI executes the root command with args and stdin, returning stdout, stderr, and
// the command error. It resets the shared flag state around the run.
func runCLI(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	config.Reset()
	t.Cleanup(config.Reset)
	root := rootCmd()
	var out, errb bytes.Buffer
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.ExecuteContext(t.Context())
	return out.String(), errb.String(), err
}

// writeTemp writes content to name under dir and returns the path.
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExecuteValidation checks command selection, flag validation, and the help path.
func TestExecuteValidation(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.md", "a plain sentence")
	b := writeTemp(t, dir, "b.md", "a plain sentence")
	c := writeTemp(t, dir, "c.md", "a plain sentence")

	tests := []struct {
		In         []string
		WantErrSub string
	}{{ // Test 0: No arguments shows help and succeeds.
		In: nil,
	}, { // Test 1: The help command succeeds.
		In: []string{"help"},
	}, { // Test 2: -h succeeds.
		In: []string{"-h"},
	}, { // Test 3: --help succeeds.
		In: []string{"--help"},
	}, { // Test 4: An unknown command is an error.
		In: []string{"mangle"}, WantErrSub: "unknown command",
	}, { // Test 5: check with a file runs.
		In: []string{"check", a},
	}, { // Test 6: fix -w with a file runs.
		In: []string{"fix", "-w", a},
	}, { // Test 7: --write with --json is rejected.
		In: []string{"fix", "-w", "--json", "notes.md"}, WantErrSub: "none of the others can be",
	}, { // Test 8: --write without a file is rejected.
		In: []string{"fix", "-w"}, WantErrSub: "not stdin",
	}, { // Test 9: check rejects -w.
		In: []string{"check", "-w", "notes.md"}, WantErrSub: "unknown shorthand flag",
	}, { // Test 10: check rejects --rewrite.
		In: []string{"check", "--rewrite", "notes.md"}, WantErrSub: "unknown flag",
	}, { // Test 11: check rejects --model.
		In: []string{"check", "--model", "m", "notes.md"}, WantErrSub: "unknown flag",
	}, { // Test 12: --model without --rewrite is rejected.
		In: []string{"fix", "--model", "m", "notes.md"}, WantErrSub: "--model needs --rewrite",
	}, { // Test 13: fix to stdout takes one file at most.
		In: []string{"fix", "a.md", "b.md"}, WantErrSub: "pass --write",
	}, { // Test 14: An unknown flag is an error.
		In: []string{"check", "--bogus"}, WantErrSub: "bogus",
	}, { // Test 15: check takes any number of files.
		In: []string{"check", a, b, c},
	}, { // Test 16: fix -w takes any number of files.
		In: []string{"fix", "-w", a, b},
	}, { // Test 17: JSON output takes one file at most.
		In: []string{"check", "--json", "a.md", "b.md"}, WantErrSub: "--json takes at most one file",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			_, _, err := runCLI(t, test.In, "")
			if test.WantErrSub == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.WantErrSub) {
				t.Errorf("err = %v, want substring %q", err, test.WantErrSub)
			}
		})
	}
}

// TestHelpOutput checks that bare invocation and the help command print usage.
func TestHelpOutput(t *testing.T) {
	for testNum, args := range [][]string{nil, {"help"}, {"--help"}} { // Test N: usage prints.
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			out, _, err := runCLI(t, args, "")
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !strings.Contains(out, "Usage:") {
				t.Errorf("stdout = %q, want usage text", out)
			}
		})
	}
}

// TestRunProfileDiscovery checks that a .slop-chop.json in the working directory is
// picked up when --profile is not set, and that --profile still wins over it.
func TestRunProfileDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, ".slop-chop.json", `{"blockWords": ["zorp"]}`)
	flagged := writeTemp(t, dir, "other.json", `{"blockWords": ["blimp"]}`)
	t.Chdir(dir)

	// The discovered profile flags zorp and nothing else.
	if _, _, err := runCLI(t, []string{"check"}, "a zorp plan"); !errors.Is(err, errFindings) {
		t.Fatalf("discovered profile: err = %v, want errFindings", err)
	}
	if _, _, err := runCLI(t, []string{"check"}, "a robust plan"); err != nil {
		t.Fatalf("discovered profile dropped the defaults: err = %v, want nil", err)
	}

	// An explicit --profile wins over the discovered file.
	if _, _, err := runCLI(t, []string{"check", "--profile", flagged}, "a zorp plan"); err != nil {
		t.Fatalf("explicit profile: err = %v, want nil", err)
	}
	if _, _, err := runCLI(t, []string{"check", "--profile", flagged}, "a blimp plan"); !errors.Is(err, errFindings) {
		t.Fatalf("explicit profile: err = %v, want errFindings", err)
	}
}
