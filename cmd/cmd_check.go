package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// checkReport is the JSON shape returned by check mode.
type checkReport struct {
	// Findings is every rule match in the input.
	Findings []sanitize.Finding `json:"findings"`
}

// checkCmd builds the check subcommand.
func checkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [file ...]",
		Short: "Report AI tells and exit non-zero when any are found.",
		Args:  cobra.ArbitraryArgs,
		RunE:  runCheck,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	return cmd
}

// runCheck scans stdin or every file argument and returns errFindings when any input
// has slop.
func runCheck(cmd *cobra.Command, args []string) error {
	if config.JSON() && len(args) > 1 {
		return fmt.Errorf("--json takes at most one file")
	}
	s, _, err := newSanitizer()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		text, err := readInput("", cmd.InOrStdin())
		if err != nil {
			return err
		}
		return checkOne(s, text, "", cmd.OutOrStdout(), cmd.ErrOrStderr())
	}
	found := false
	for _, path := range args {
		text, err := readInput(path, cmd.InOrStdin())
		if err != nil {
			return err
		}
		switch err := checkOne(s, text, path, cmd.OutOrStdout(), cmd.ErrOrStderr()); {
		case errors.Is(err, errFindings):
			found = true
		case err != nil:
			return err
		}
	}
	if found {
		return errFindings
	}
	return nil
}

// checkOne reports findings for one input and returns errFindings when there are any.
// Findings on a file are prefixed with its path, so a terminal can jump to the spot.
func checkOne(s *sanitize.Sanitizer, text, path string, stdout, stderr io.Writer) error {
	findings := s.Check(text)
	if config.JSON() {
		if err := writeJSON(stdout, checkReport{Findings: orEmpty(findings)}, config.Pretty()); err != nil {
			return err
		}
	} else {
		prefix := ""
		if path != "" {
			prefix = path + ":"
		}
		for _, f := range findings {
			_, _ = fmt.Fprintf(stderr, "%s%s\n", prefix, f)
		}
		if len(findings) > 0 {
			_, _ = fmt.Fprintf(stderr, "slop-chop: %d finding(s)\n", len(findings))
		}
	}
	if len(findings) > 0 {
		return errFindings
	}
	return nil
}
