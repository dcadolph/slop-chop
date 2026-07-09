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
