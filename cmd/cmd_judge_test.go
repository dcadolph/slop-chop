package cmd

import (
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/internal/rewrite"
)

// TestJudgeFlagGuards checks that the judge flags are rejected without --verify.
func TestJudgeFlagGuards(t *testing.T) {
	tests := []struct {
		Name string
		Args []string
		Want string
	}{{
		Name: "judge-provider needs verify",
		Args: []string{"fix", "--rewrite", "--judge-provider", "openai"},
		Want: "--judge-provider needs --verify",
	}, {
		Name: "judge-model needs verify",
		Args: []string{"fix", "--rewrite", "--judge-model", "m"},
		Want: "--judge-model needs --verify",
	}, {
		Name: "judge-base-url needs verify",
		Args: []string{"fix", "--rewrite", "--judge-base-url", "http://localhost:1"},
		Want: "--judge-base-url needs --verify",
	}}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			_, _, err := runCLI(t, test.Args, "text")
			if err == nil || !strings.Contains(err.Error(), test.Want) {
				t.Errorf("err = %v, want containing %q", err, test.Want)
			}
		})
	}
}

// TestJudgeRouting checks that the meaning check runs on the judge's backend, not the
// rewriter's. The rewrite pass is stubbed to succeed; the judge is left real and pointed at
// openai with its key cleared, so the failure names the judge's provider and the fix falls
// back closed to the rules output.
func TestJudgeRouting(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	fakeRewrite(t, "clean text")
	out, stderr, err := runCLI(t,
		[]string{"fix", "--rewrite", "--verify", "--judge-provider", "openai"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, "OPENAI_API_KEY") {
		t.Errorf("stderr = %q, want the judge's missing openai key", stderr)
	}
	if out != "dirty text" {
		t.Errorf("out = %q, want the rules output kept", out)
	}
}

// TestJudgeDistinctNoWarning checks that a judge on its own backend raises no shared-model
// warning.
func TestJudgeDistinctNoWarning(t *testing.T) {
	fakeRewrite(t, "clean text")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	_, stderr, err := runCLI(t,
		[]string{"fix", "--rewrite", "--verify", "--judge-provider", "openai"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(stderr, "judge shares") {
		t.Errorf("stderr = %q, want no shared-judge warning", stderr)
	}
}

// TestJudgeExplicitSameBackendWarns checks that naming the rewriter's own backend as the
// judge still counts as shared, so the warning fires on the fact, not the flags.
func TestJudgeExplicitSameBackendWarns(t *testing.T) {
	fakeRewrite(t, "clean text")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	_, stderr, err := runCLI(t,
		[]string{"fix", "--rewrite", "--verify", "--judge-provider", "anthropic"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, "judge shares the rewriter's model") {
		t.Errorf("stderr = %q, want the shared-judge warning", stderr)
	}
}
