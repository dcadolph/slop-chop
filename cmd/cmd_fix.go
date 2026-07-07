package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/rewrite"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// fixReport is the JSON shape returned by fix mode.
type fixReport struct {
	// Cleaned is the rewritten text.
	Cleaned string `json:"cleaned"`
	// Findings is every rule match in the original input.
	Findings []sanitize.Finding `json:"findings"`
	// Verify is the model meaning check verdict, present only when --verify ran.
	Verify *rewrite.Verdict `json:"verify,omitempty"`
}

// fixCmd builds the fix subcommand.
func fixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix [file ...]",
		Short: "Rewrite the input with the slop chopped out.",
		Args:  cobra.ArbitraryArgs,
		RunE:  runFix,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	f.AddFlag(&config.FlagWrite)
	f.AddFlag(&config.FlagRewrite)
	f.AddFlag(&config.FlagModel)
	f.AddFlag(&config.FlagVerify)
	f.AddFlag(&config.FlagVerifyStrict)
	f.AddFlag(&config.FlagVerifyRetry)
	cmd.MarkFlagsMutuallyExclusive(config.KeyWrite, config.KeyJSON)
	return cmd
}

// runFix validates the flag and file combination, then cleans stdin, one file to
// stdout, or every file in place with --write.
func runFix(cmd *cobra.Command, args []string) error {
	switch {
	case config.Changed(config.KeyModel) && !config.Rewrite():
		return fmt.Errorf("--model needs --rewrite")
	case config.Verify() && !config.Rewrite():
		return fmt.Errorf("--verify needs --rewrite")
	case config.VerifyStrict() && !config.Verify():
		return fmt.Errorf("--verify-strict needs --verify")
	case config.Changed(config.KeyVerifyRetry) && !config.Verify():
		return fmt.Errorf("--verify-retry needs --verify")
	case config.Write() && config.JSON():
		// Cobra rejects this when both come from the command line, but env vars can set
		// either without tripping that check, so guard it here too.
		return fmt.Errorf("cannot use --write with --json")
	case config.JSON() && len(args) > 1:
		return fmt.Errorf("--json takes at most one file")
	case config.Write() && len(args) == 0:
		return fmt.Errorf("--write needs a file argument, not stdin")
	case !config.Write() && len(args) > 1:
		return fmt.Errorf("fix writes one file to stdout: pass --write to rewrite several in place")
	}

	s, profile, err := newSanitizer()
	if err != nil {
		return err
	}

	if config.Write() {
		for _, path := range args {
			text, err := readInput(path, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if err := fixOne(cmd.Context(), s, profile.Tone, text, path,
				cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
		}
		return nil
	}
	path := ""
	if len(args) == 1 {
		path = args[0]
	}
	text, err := readInput(path, cmd.InOrStdin())
	if err != nil {
		return err
	}
	return fixOne(cmd.Context(), s, profile.Tone, text, path, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

// fixOne cleans one input and writes it to stdout, back into its file with --write, or
// as JSON. With --rewrite it runs the model pass on the rules output first, then verifies
// the reply against the deterministic guarantees.
func fixOne(ctx context.Context, s *sanitize.Sanitizer, tone []string, text, path string,
	stdout, stderr io.Writer) error {
	out, findings := s.Fix(text)
	var verdict *rewrite.Verdict
	if config.Rewrite() {
		rw, v, err := rewriteAndVerify(ctx, s, tone, text, out, stderr)
		if err != nil {
			return err
		}
		out, verdict = rw, v
	}
	if config.JSON() {
		report := fixReport{Cleaned: out, Findings: orEmpty(findings), Verify: verdict}
		if err := writeJSON(stdout, report, config.Pretty()); err != nil {
			return err
		}
	} else if config.Write() {
		if err := writeFile(path, out); err != nil {
			return err
		}
	} else if _, err := io.WriteString(stdout, out); err != nil {
		return err
	}
	// Gate after the output is written so --verify-strict still hands back the rewrite and
	// only then fails with a non-zero exit.
	if config.VerifyStrict() && verdict != nil && !verdict.Faithful {
		return fmt.Errorf("meaning check flagged the rewrite")
	}
	return nil
}

// rewriteAndVerify runs the model rewrite over rulesOut and re-checks it against the
// deterministic guarantees. When --verify is set it runs the meaning check, and on a
// flagged change it re-rewrites up to --verify-retry more times, feeding the flagged
// issues back so the model can preserve them. It returns the final text and the last
// verdict, which is nil when --verify is off or the check could not run.
func rewriteAndVerify(ctx context.Context, s *sanitize.Sanitizer, tone []string,
	original, rulesOut string, stderr io.Writer) (string, *rewrite.Verdict, error) {
	tries := 1
	if config.Verify() {
		tries += config.VerifyRetry()
	}
	var feedback []string
	var out string
	for attempt := 0; attempt < tries; attempt++ {
		rw, err := rewritePass(ctx, config.Model(), tone, rulesOut, feedback...)
		if err != nil {
			return "", nil, err
		}
		rw = verifyRewrite(s, original, rw, stderr)
		// The rewriter trims the reply, so put back the newline the input ended with.
		if strings.HasSuffix(original, "\n") && !strings.HasSuffix(rw, "\n") {
			rw += "\n"
		}
		out = rw
		if !config.Verify() {
			return out, nil, nil
		}
		verdict, err := judgePass(ctx, config.Model(), original, out)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "slop-chop: the meaning check could not run: %v\n", err)
			return out, nil, nil
		}
		if verdict.Faithful || attempt+1 == tries {
			reportVerdict(verdict, stderr)
			return out, &verdict, nil
		}
		feedback = feedbackNotes(verdict.Issues)
		_, _ = fmt.Fprintf(stderr, "slop-chop: the meaning check flagged the rewrite; retrying (%d of %d)\n",
			attempt+1, tries-1)
	}
	return out, nil, nil
}

// verifyRewrite re-checks the model's reply against the deterministic guarantees. A
// model can undo the rules, reintroducing tells the rules removed, or disturb the code
// the rules protect. So this runs the rules once more over the reply, cleaning any
// fixable tells, and warns on stderr when the reply still carries buzzwords the rules
// only flag or when its code segments no longer match the original. The re-cleaned text
// is returned.
func verifyRewrite(s *sanitize.Sanitizer, original, reply string, stderr io.Writer) string {
	cleaned, _ := s.Fix(reply)
	if cleaned != reply {
		_, _ = fmt.Fprintln(stderr, "slop-chop: the rewrite carried tells the rules had to clean")
	}
	for _, f := range s.Check(cleaned) {
		if f.Replacement == nil {
			_, _ = fmt.Fprintf(stderr, "slop-chop: the rewrite left %s %q at %d:%d\n", f.Rule, f.Match, f.Line, f.Col)
		}
	}
	if in, out := sanitize.CodeSegments(original), sanitize.CodeSegments(cleaned); !slices.Equal(in, out) {
		_, _ = fmt.Fprintf(stderr, "slop-chop: the rewrite changed code (%d segment(s) in, %d out); check the output\n",
			len(in), len(out))
	}
	reportAnchorDrift(original, cleaned, stderr)
	return cleaned
}

// reportAnchorDrift warns when the rewrite dropped or added a load-bearing token like a
// number, link, or acronym, which usually means a fact changed rather than the wording.
func reportAnchorDrift(original, cleaned string, stderr io.Writer) {
	dropped, added := anchorDelta(sanitize.Anchors(original), sanitize.Anchors(cleaned))
	warnAnchors(stderr, "dropped", dropped)
	warnAnchors(stderr, "added", added)
}

// warnAnchors writes one line per anchor, capped so a wholesale rewrite cannot flood the
// output.
func warnAnchors(stderr io.Writer, verb string, anchors []string) {
	const limit = 20
	for i, a := range anchors {
		if i == limit {
			_, _ = fmt.Fprintf(stderr, "slop-chop: ... and %d more %s\n", len(anchors)-limit, verb)
			return
		}
		_, _ = fmt.Fprintf(stderr, "slop-chop: the rewrite %s %q; a fact may have changed\n", verb, a)
	}
}

// anchorDelta returns the anchors present more often in before than after (dropped) and
// more often in after than before (added), each sorted for a stable report.
func anchorDelta(before, after []string) (dropped, added []string) {
	bc, ac := anchorCounts(before), anchorCounts(after)
	for v, n := range bc {
		for i := 0; i < n-ac[v]; i++ {
			dropped = append(dropped, v)
		}
	}
	for v, n := range ac {
		for i := 0; i < n-bc[v]; i++ {
			added = append(added, v)
		}
	}
	slices.Sort(dropped)
	slices.Sort(added)
	return dropped, added
}

// anchorCounts tallies how many times each anchor appears.
func anchorCounts(anchors []string) map[string]int {
	m := make(map[string]int, len(anchors))
	for _, a := range anchors {
		m[a]++
	}
	return m
}

// reportVerdict warns on stderr for each meaning change the judge found. A faithful
// verdict stays quiet. An unfaithful verdict with no detail still gets a line so the
// warning is never silent.
func reportVerdict(verdict rewrite.Verdict, stderr io.Writer) {
	for _, issue := range verdict.Issues {
		_, _ = fmt.Fprintf(stderr, "slop-chop: meaning %s: was %q now %q (%s)\n",
			issue.Kind, issue.Was, issue.Now, issue.Note)
	}
	if !verdict.Faithful && len(verdict.Issues) == 0 {
		_, _ = fmt.Fprintln(stderr, "slop-chop: the meaning check flagged the rewrite but gave no detail")
	}
}

// feedbackNotes turns judge issues into short instructions the retry rewrite can act on,
// naming the fact to keep so the model does not repeat the change.
func feedbackNotes(issues []rewrite.Issue) []string {
	notes := make([]string, 0, len(issues))
	for _, issue := range issues {
		switch {
		case issue.Was != "" && issue.Now != "":
			notes = append(notes, fmt.Sprintf("keep %q, do not change it to %q (%s)", issue.Was, issue.Now, issue.Note))
		case issue.Was != "":
			notes = append(notes, fmt.Sprintf("keep %q, do not drop it (%s)", issue.Was, issue.Note))
		case issue.Now != "":
			notes = append(notes, fmt.Sprintf("do not add %q (%s)", issue.Now, issue.Note))
		case issue.Note != "":
			notes = append(notes, issue.Note)
		}
	}
	return notes
}

// rewritePass runs the model rewrite over text. It is a variable so tests can swap in
// a fake model.
//
//nolint:gochecknoglobals // Test seam.
var rewritePass = func(ctx context.Context, model string, tone []string, text string,
	feedback ...string) (string, error) {
	rw := rewrite.New(rewrite.NewAnthropicCompleter(model), tone...)
	return rw.Rewrite(ctx, text, feedback...)
}

// judgePass runs the model meaning check over the original and the rewrite. It is a
// variable so tests can swap in a fake judge.
//
//nolint:gochecknoglobals // Test seam.
var judgePass = func(ctx context.Context, model, original, rewritten string) (rewrite.Verdict, error) {
	return rewrite.NewJudge(rewrite.NewAnthropicCompleter(model)).Judge(ctx, original, rewritten)
}

// writeFile writes out back to path, keeping the file's existing mode.
func writeFile(path, out string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(out), mode); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
