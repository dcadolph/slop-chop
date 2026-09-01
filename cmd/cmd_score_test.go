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

// TestScoreByParagraph checks that --by-paragraph scores each paragraph on its own with
// its starting line, and that --max gates on the hottest paragraph rather than the
// diluted whole.
func TestScoreByParagraph(t *testing.T) {
	human := "The mail arrived late and nobody minded much at all today, and the dog barked twice.\n\n"
	doc := human +
		"In summary, we leverage comprehensive synergy to seamlessly revolutionize robust workflows.\n\n" +
		strings.Repeat(human, 12)
	dir := t.TempDir()
	path := writeTemp(t, dir, "mixed.md", doc)

	stdout, _, err := runCLI(t, []string{"score", "--by-paragraph", path}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil without a gate", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 14 {
		t.Fatalf("lines = %d, want 14 (%q)", len(lines), stdout)
	}
	if !strings.Contains(lines[1], path+":3:") || !strings.Contains(lines[1], "heavy slop") {
		t.Errorf("slop paragraph = %q, want line 3 flagged heavy", lines[1])
	}
	for i, l := range lines {
		if i != 1 && !strings.Contains(l, "reads clean") {
			t.Errorf("human paragraph %d = %q, want clean", i, l)
		}
	}

	// The whole document dilutes below the gate; the paragraph view does not.
	if _, _, err := runCLI(t, []string{"score", "--max", "50", path}, ""); err != nil {
		t.Fatalf("whole-doc gate: err = %v, want nil (diluted)", err)
	}
	if _, _, err := runCLI(t, []string{"score", "--by-paragraph", "--max", "50", path}, ""); !errors.Is(err, errFindings) {
		t.Errorf("by-paragraph gate: err = %v, want errFindings on the hot paragraph", err)
	}
}
