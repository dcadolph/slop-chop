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
	rewritePass = func(_ context.Context, _ string, _ []string, _ string) (string, error) {
		return reply, nil
	}
	t.Cleanup(func() { rewritePass = old })
}

// fakeJudge swaps judgePass to return verdict and err for the duration of a test.
func fakeJudge(t *testing.T, verdict rewrite.Verdict, err error) {
	t.Helper()
	old := judgePass
	judgePass = func(_ context.Context, _, _, _ string) (rewrite.Verdict, error) {
		return verdict, err
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

// TestFixVerifyFaithfulQuiet checks that a faithful verdict raises no meaning warning.
func TestFixVerifyFaithfulQuiet(t *testing.T) {
	fakeRewrite(t, "we reached 99.9% uptime")
	fakeJudge(t, rewrite.Verdict{Faithful: true}, nil)
	_, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "we hit 99.9% uptime")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if strings.Contains(stderr, "meaning") {
		t.Errorf("stderr = %q, want no meaning warning", stderr)
	}
}

// TestFixVerifyJudgeErrorWarns checks that a judge that cannot run warns without failing
// the fix, since the rewrite itself is valid output.
func TestFixVerifyJudgeErrorWarns(t *testing.T) {
	fakeRewrite(t, "clean text")
	fakeJudge(t, rewrite.Verdict{}, errors.New("api down"))
	out, stderr, err := runCLI(t, []string{"fix", "--rewrite", "--verify"}, "dirty text")
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if out != "clean text" {
		t.Errorf("stdout = %q, want the rewrite delivered anyway", out)
	}
	if !strings.Contains(stderr, "meaning check could not run") {
		t.Errorf("stderr = %q, want a could-not-run warning", stderr)
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
