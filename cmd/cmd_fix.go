package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
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
			if err := fixOne(cmd.Context(), s, profile.Tone, text, path, cmd.OutOrStdout()); err != nil {
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
	return fixOne(cmd.Context(), s, profile.Tone, text, path, cmd.OutOrStdout())
}

// fixOne cleans one input and writes it to stdout, back into its file with --write, or
// as JSON. With --rewrite it runs the model pass on the rules output first.
func fixOne(ctx context.Context, s *sanitize.Sanitizer, tone []string, text, path string,
	stdout io.Writer) error {
	out, findings := s.Fix(text)
	if config.Rewrite() {
		rw, err := rewritePass(ctx, config.Model(), tone, out)
		if err != nil {
			return err
		}
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
