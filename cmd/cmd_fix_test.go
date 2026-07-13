package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/internal/rewrite"
)

// fakeRewrite swaps rewritePass to return reply verbatim for the duration of a test.
func fakeRewrite(t *testing.T, reply string) {
	t.Helper()
	old := rewritePass
	rewritePass = func(_ context.Context, _ rewrite.Completer, _ []string, _ string, _ ...string) (string, error) {
		return reply, nil
	}
	t.Cleanup(func() { rewritePass = old })
}

// fakeJudge swaps judgePass to return verdict and err for the duration of a test.
func fakeJudge(t *testing.T, verdict rewrite.Verdict, err error) {
	t.Helper()
	old := judgePass
	judgePass = func(_ context.Context, _ rewrite.Completer, _, _ string) (rewrite.Verdict, error) {
		return verdict, err
	}
	t.Cleanup(func() { judgePass = old })
}

// fakeRewriteSeq swaps rewritePass to return replies in order, returning the last reply
// once they run out. It records the feedback passed on each call so a test can check that
// a retry carried the flagged issues.
func fakeRewriteSeq(t *testing.T, replies ...string) *[][]string {
	t.Helper()
	old := rewritePass
	calls := 0
	feedbacks := &[][]string{}
	rewritePass = func(_ context.Context, _ rewrite.Completer, _ []string, _ string, feedback ...string) (string, error) {
		*feedbacks = append(*feedbacks, feedback)
		reply := replies[len(replies)-1]
		if calls < len(replies) {
			reply = replies[calls]
		}
		calls++
		return reply, nil
	}
	t.Cleanup(func() { rewritePass = old })
	return feedbacks
}

// fakeJudgeSeq swaps judgePass to return verdicts in order, returning the last verdict
// once they run out.
func fakeJudgeSeq(t *testing.T, verdicts ...rewrite.Verdict) {
	t.Helper()
	old := judgePass
	calls := 0
	judgePass = func(_ context.Context, _ rewrite.Completer, _, _ string) (rewrite.Verdict, error) {
		verdict := verdicts[len(verdicts)-1]
		if calls < len(verdicts) {
			verdict = verdicts[calls]
		}
		calls++
		return verdict, nil
	}
	t.Cleanup(func() { judgePass = old })
}

// TestFixWriteJSONEnvGuard checks that --write and --json set through the environment
// are still rejected, since cobra's mutual-exclusion only sees command-line flags.
func TestFixWriteJSONEnvGuard(t *testing.T) {
	path := writeTemp(t, t.TempDir(), "notes.md", "In summary, a plan.")
	t.Setenv("SLOP_CHOP_JSON", "true")
	_, _, err := runCLI(t, []string{"fix", "-w", path}, "")
	if err == nil || !strings.Contains(err.Error(), "cannot use --write with --json") {
		t.Errorf("err = %v, want a write-with-json rejection", err)
	}
}

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

// TestFixMissingFile checks that a missing input file is a read error, not a crash.
func TestFixMissingFile(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "/no/such/file.md"}, "")
	if err == nil || !strings.Contains(err.Error(), "read file") {
		t.Errorf("err = %v, want a read-file error", err)
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

// TestFixRewriteReintroducedTell checks that a reply that undoes a rule is cleaned again
// and the user is warned.
func TestFixRewriteReintroducedTell(t *testing.T) {
	fakeRewrite(t, "a plan — and more")
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite"}, "seed")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(out, "—") {
		t.Errorf("stdout = %q, still has an em-dash", out)
	}
	if !strings.Contains(stderr, "carried tells the rules had to clean") {
		t.Errorf("stderr = %q, want a carried-tells warning", stderr)
	}
}

// TestFixRewriteLeftBlockWord checks that a buzzword the model failed to drop is warned
// about, since the rules only flag it and cannot remove it.
func TestFixRewriteLeftBlockWord(t *testing.T) {
	fakeRewrite(t, "a robust plan")
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite"}, "seed")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "a robust plan" {
		t.Errorf("stdout = %q, want the reply unchanged", out)
	}
	if !strings.Contains(stderr, "left word:robust") {
		t.Errorf("stderr = %q, want a left-buzzword warning", stderr)
	}
}

// TestFixRewriteChangedCode checks that a reply which dropped the input's code block is
// flagged as a code change.
func TestFixRewriteChangedCode(t *testing.T) {
	fakeRewrite(t, "just prose now")
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite"}, "text\n```\ncode()\n```\n")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, "changed code") {
		t.Errorf("stderr = %q, want a changed-code warning", stderr)
	}
}

// TestFixRewriteAnchorDrift checks that a reply which changed a number is flagged as a
// possible fact change, dropping the old value and adding the new one.
func TestFixRewriteAnchorDrift(t *testing.T) {
	fakeRewrite(t, "we reached 99% uptime")
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, `dropped "99.9%"`) {
		t.Errorf("stderr = %q, want a dropped-anchor warning", stderr)
	}
	if !strings.Contains(stderr, `added "99%"`) {
		t.Errorf("stderr = %q, want an added-anchor warning", stderr)
	}
}

// TestFixRewriteFaithfulNoDrift checks that a reply keeping the same anchors raises no
// drift warning, so faithful rewrites stay quiet.
func TestFixRewriteFaithfulNoDrift(t *testing.T) {
	fakeRewrite(t, "we reached 99.9% uptime")
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(stderr, "fact may have changed") {
		t.Errorf("stderr = %q, want no drift warning", stderr)
	}
}

// TestFixVerifyNeedsRewrite checks that --verify without --rewrite is rejected.
func TestFixVerifyNeedsRewrite(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "--verify"}, "some text")
	if err == nil || !strings.Contains(err.Error(), "--verify needs --rewrite") {
		t.Errorf("err = %v, want a verify-needs-rewrite error", err)
	}
}

// TestFixVerifyReportsIssues checks that a meaning change from the judge is warned.
func TestFixVerifyReportsIssues(t *testing.T) {
	fakeRewrite(t, "we reached 99% uptime")
	fakeJudge(t, rewrite.Verdict{
		Faithful: false,
		Issues:   []rewrite.Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
	}, nil)
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, `meaning changed: was "99.9%" now "99%"`) {
		t.Errorf("stderr = %q, want a meaning-change warning", stderr)
	}
}

// TestFixVerifyFaithfulQuiet checks that a faithful verdict with a distinct judge raises no
// meaning warning at all.
func TestFixVerifyFaithfulQuiet(t *testing.T) {
	fakeRewrite(t, "we reached 99.9% uptime")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	_, stderr, err := runCLI(t,
		[]string{"fix", "--rewrite", "--verify", "--judge-model", "other-model"},
		"we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(stderr, "meaning") {
		t.Errorf("stderr = %q, want no meaning warning", stderr)
	}
}

// TestFixVerifySharedJudgeWarns checks that leaving the judge on the rewriter's backend says
// so, since the rewriter is then grading its own work.
func TestFixVerifySharedJudgeWarns(t *testing.T) {
	fakeRewrite(t, "we reached 99.9% uptime")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, "judge shares the rewriter's model") {
		t.Errorf("stderr = %q, want a shared-judge warning", stderr)
	}
}

// TestFixVerifyJudgeErrorFallsBack checks that when the meaning check cannot run, the fix
// fails closed: it keeps the deterministic rules output rather than ship an unverified
// rewrite, and says so.
func TestFixVerifyJudgeErrorFallsBack(t *testing.T) {
	fakeRewrite(t, "clean text")
	fakeJudge(t, rewrite.Verdict{}, errors.New("api down"))
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "dirty text" {
		t.Errorf("stdout = %q, want the rules output kept, not the unverified rewrite", out)
	}
	if !strings.Contains(stderr, "meaning check could not run") {
		t.Errorf("stderr = %q, want a could-not-run warning", stderr)
	}
}

// TestFixVerifyStrictGates checks that --verify-strict exits non-zero on a flagged change
// and keeps the safe rules output rather than the flagged rewrite.
func TestFixVerifyStrictGates(t *testing.T) {
	fakeRewrite(t, "we reached 99% uptime")
	fakeJudge(t, rewrite.Verdict{
		Faithful: false,
		Issues:   []rewrite.Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
	}, nil)
	out, _, err := runCLI(t, []string{"fix", "--rewrite", "--verify", "--verify-strict"}, "we hit 99.9% uptime")
	if err == nil || !strings.Contains(err.Error(), "meaning check flagged the rewrite") {
		t.Errorf("err = %v, want a strict gate error", err)
	}
	if out != "we hit 99.9% uptime" {
		t.Errorf("stdout = %q, want the rules output with the original figure, not the flagged rewrite", out)
	}
}

// TestFixVerifyUnfaithfulKeepsRules checks that without --verify-strict, an unfaithful
// rewrite is dropped for the deterministic rules output rather than emitted.
func TestFixVerifyUnfaithfulKeepsRules(t *testing.T) {
	fakeRewrite(t, "we reached 99% uptime")
	fakeJudge(t, rewrite.Verdict{
		Faithful: false,
		Issues:   []rewrite.Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
	}, nil)
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "we hit 99.9% uptime" {
		t.Errorf("stdout = %q, want the rules output, not the flagged rewrite", out)
	}
	if !strings.Contains(stderr, "kept the rules output") {
		t.Errorf("stderr = %q, want a kept-rules-output note", stderr)
	}
}

// TestFixVerifyStrictFaithfulPasses checks that a faithful verdict does not trip the gate.
func TestFixVerifyStrictFaithfulPasses(t *testing.T) {
	fakeRewrite(t, "we reached 99.9% uptime")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	if _, _, err := runCLI(t, []string{"fix", "--rewrite", "--verify", "--verify-strict"},
		"we hit 99.9% uptime"); err != nil {
		t.Fatalf("fix: %v", err)
	}
}

// TestFixVerifyStrictNeedsVerify checks that --verify-strict without --verify is rejected.
func TestFixVerifyStrictNeedsVerify(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "--rewrite", "--verify-strict"}, "some text")
	if err == nil || !strings.Contains(err.Error(), "--verify-strict needs --verify") {
		t.Errorf("err = %v, want a strict-needs-verify error", err)
	}
}

// TestFixVerifyRetryNeedsVerify checks that --verify-retry without --verify is rejected.
func TestFixVerifyRetryNeedsVerify(t *testing.T) {
	_, _, err := runCLI(t, []string{"fix", "--rewrite", "--verify-retry", "2"}, "some text")
	if err == nil || !strings.Contains(err.Error(), "--verify-retry needs --verify") {
		t.Errorf("err = %v, want a retry-needs-verify error", err)
	}
}

// TestFixVerifyRetrySucceeds checks that a flagged rewrite is retried with the issue fed
// back and the faithful second attempt is delivered.
func TestFixVerifyRetrySucceeds(t *testing.T) {
	feedbacks := fakeRewriteSeq(t, "we reached 99% uptime", "we reached 99.9% uptime")
	fakeJudgeSeq(t,
		rewrite.Verdict{
			Faithful: false,
			Issues:   []rewrite.Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
		},
		rewrite.Verdict{Faithful: true},
	)
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify", "--verify-retry", "1"},
		"we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "we reached 99.9% uptime" {
		t.Errorf("stdout = %q, want the retry result", out)
	}
	if !strings.Contains(stderr, "retrying (1 of 1)") {
		t.Errorf("stderr = %q, want a retry notice", stderr)
	}
	if len(*feedbacks) != 2 || len((*feedbacks)[1]) == 0 || !strings.Contains((*feedbacks)[1][0], "99.9%") {
		t.Errorf("feedback = %v, want the second call to carry the flagged fact", *feedbacks)
	}
}

// TestFixVerifyRetryExhausted checks that a rewrite that stays flagged reports the final
// verdict after the retries run out, and without --verify-strict does not fail.
func TestFixVerifyRetryExhausted(t *testing.T) {
	fakeRewrite(t, "we reached 99% uptime")
	fakeJudge(t, rewrite.Verdict{
		Faithful: false,
		Issues:   []rewrite.Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
	}, nil)
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify", "--verify-retry", "1"},
		"we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(stderr, "retrying (1 of 1)") {
		t.Errorf("stderr = %q, want a retry notice", stderr)
	}
	if !strings.Contains(stderr, `meaning changed: was "99.9%" now "99%"`) {
		t.Errorf("stderr = %q, want the final verdict reported", stderr)
	}
}

// TestFixVerifyJSON checks that the verdict rides along in the JSON report.
func TestFixVerifyJSON(t *testing.T) {
	fakeRewrite(t, "clean text")
	fakeJudge(t, rewrite.Verdict{
		Faithful: false,
		Issues:   []rewrite.Issue{{Kind: "removed", Was: "a fact", Note: "dropped"}},
	}, nil)
	out, _, err := runCLI(t, []string{"fix", "--rewrite", "--verify", "--json"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(out, `"verify"`) || !strings.Contains(out, `"faithful":false`) {
		t.Errorf("stdout = %q, want the verdict in the JSON report", out)
	}
}

// TestFixJSONNoVerifyOmitsVerdict checks that the verify field is absent when the meaning
// check did not run.
func TestFixJSONNoVerifyOmitsVerdict(t *testing.T) {
	fakeRewrite(t, "clean text")
	out, _, err := runCLI(t, []string{"fix", "--rewrite", "--json"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(out, "verify") {
		t.Errorf("stdout = %q, want no verify field without --verify", out)
	}
}

// TestFixRewriteNewline checks that the trailing newline survives the rewrite pass,
// which trims the model reply.
func TestFixRewriteNewline(t *testing.T) {
	fakeRewrite(t, "clean text")

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
