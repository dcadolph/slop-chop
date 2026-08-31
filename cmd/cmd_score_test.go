package cmd

import (
	"errors"
	"strings"
	"testing"
)

// TestScoreStdout checks that score prints a bare integer for stdin input.
func TestScoreStdout(t *testing.T) {
	stdout, _, err := runCLI(t, []string{"score"}, "We leverage cutting-edge synergy to revolutionize.")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("stdout = %q, want a score", stdout)
	}
}

// TestScoreJSON checks that score --json reports the value and the density fields.
func TestScoreJSON(t *testing.T) {
	stdout, _, err := runCLI(t, []string{"score", "--json"}, "We leverage robust synergy.")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	for _, want := range []string{`"value"`, `"tells"`, `"tellsPer100"`, `"cadenceCv"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want field %q", stdout, want)
		}
	}
}

// TestScoreMaxGate checks that --max fails the run when the score is above the gate and
// passes when it is at or below.
func TestScoreMaxGate(t *testing.T) {
	dirty := "We leverage cutting-edge synergy to revolutionize a robust, seamless paradigm."
	if _, _, err := runCLI(t, []string{"score", "--max", "10"}, dirty); !errors.Is(err, errFindings) {
		t.Errorf("over-gate err = %v, want errFindings", err)
	}
	clean := "The dog barked at the mail truck. Rain fell all day."
	if _, _, err := runCLI(t, []string{"score", "--max", "90"}, clean); err != nil {
		t.Errorf("under-gate err = %v, want nil", err)
	}
}

// TestScoreFiles checks the multi-file path: each file gets its own prefixed score line,
// and one file over the gate fails the whole run.
func TestScoreFiles(t *testing.T) {
	dir := t.TempDir()
	clean := writeTemp(t, dir, "clean.md", "The dog barked at the mail truck. Rain fell all day.")
	dirty := writeTemp(t, dir, "dirty.md",
		"We leverage cutting-edge synergy to revolutionize a robust, seamless paradigm.")

	stdout, _, err := runCLI(t, []string{"score", clean, dirty}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil without a gate", err)
	}
	if !strings.Contains(stdout, clean+": ") || !strings.Contains(stdout, dirty+": ") {
		t.Errorf("stdout = %q, want a prefixed line per file", stdout)
	}

	if _, _, err := runCLI(t, []string{"score", "--max", "10", clean, dirty}, ""); !errors.Is(err, errFindings) {
		t.Errorf("gated err = %v, want errFindings when one file is over", err)
	}
}

// TestScoreJSONMultiFile checks that --json refuses more than one file.
func TestScoreJSONMultiFile(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.md", "text")
	b := writeTemp(t, dir, "b.md", "text")
	if _, _, err := runCLI(t, []string{"score", "--json", a, b}, ""); err == nil {
		t.Error("score --json with two files returned nil, want an error")
	}
}
