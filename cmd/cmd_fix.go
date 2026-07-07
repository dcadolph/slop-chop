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
	cmd.MarkFlagsMutuallyExclusive(config.KeyWrite, config.KeyJSON)
	return cmd
}

// runFix validates the flag and file combination, then cleans stdin, one file to
// stdout, or every file in place with --write.
func runFix(cmd *cobra.Command, args []string) error {
	switch {
	case config.Changed(config.KeyModel) && !config.Rewrite():
		return fmt.Errorf("--model needs --rewrite")
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
	if config.Rewrite() {
		rw, err := rewritePass(ctx, config.Model(), tone, out)
		if err != nil {
			return err
		}
		rw = verifyRewrite(s, text, rw, stderr)
		// The rewriter trims the reply, so put back the newline the input ended with.
		if strings.HasSuffix(text, "\n") && !strings.HasSuffix(rw, "\n") {
			rw += "\n"
		}
		out = rw
	}
	if config.JSON() {
		return writeJSON(stdout, fixReport{Cleaned: out, Findings: orEmpty(findings)}, config.Pretty())
	}
	if config.Write() {
		return writeFile(path, out)
	}
	_, err := io.WriteString(stdout, out)
	return err
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

// rewritePass runs the model rewrite over text. It is a variable so tests can swap in
// a fake model.
//
//nolint:gochecknoglobals // Test seam.
var rewritePass = func(ctx context.Context, model string, tone []string, text string) (string, error) {
	rw := rewrite.New(rewrite.NewAnthropicCompleter(model), tone...)
	return rw.Rewrite(ctx, text)
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
