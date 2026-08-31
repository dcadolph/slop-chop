package prompt

import (
	"strings"
	"testing"
)

// TestJudgeAndLearn checks that the fixed prompts are non-empty and name their jobs.
func TestJudgeAndLearn(t *testing.T) {
	t.Parallel()
	if got := Judge(); !strings.Contains(got, "REWRITE") {
		t.Errorf("Judge() = %q, want it to describe comparing a rewrite", got)
	}
	if got := Learn(); !strings.Contains(got, "voice") {
		t.Errorf("Learn() = %q, want it to describe learning a voice", got)
	}
}

// TestSystem checks the assembled rewrite instruction with and without tone and feedback.
func TestSystem(t *testing.T) {
	t.Parallel()

	// Test 0: The bare prompt has the core instruction and no optional sections.
	bare := System(nil, nil)
	if !strings.Contains(bare, "rewrite text") {
		t.Errorf("System(nil, nil) = %q, want the core instruction", bare)
	}
	if strings.Contains(bare, "Match this voice") || strings.Contains(bare, "Keep these facts") {
		t.Errorf("System(nil, nil) = %q, want no tone or feedback section", bare)
	}

	// Test 1: Tone notes appear under the voice heading.
	toned := System([]string{"short sentences"}, nil)
	if !strings.Contains(toned, "Match this voice:\n- short sentences\n") {
		t.Errorf("System(tone, nil) = %q, want the tone note listed", toned)
	}

	// Test 2: Feedback notes appear under the retry heading.
	retried := System(nil, []string{"keep the 42% figure"})
	if !strings.Contains(retried, "Keep these facts exactly this time:\n- keep the 42% figure\n") {
		t.Errorf("System(nil, feedback) = %q, want the feedback note listed", retried)
	}
}
