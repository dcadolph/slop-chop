package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// scoreCmd builds the score subcommand.
func scoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "score [file ...]",
		Short: "Rate how much the text reads like AI wrote it, from 0 to 100.",
		Args:  cobra.ArbitraryArgs,
		RunE:  runScore,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	f.AddFlag(&config.FlagMax)
	return cmd
}

// runScore scores stdin or every file argument and returns errFindings when any score is
// above the --max gate.
func runScore(cmd *cobra.Command, args []string) error {
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
		return scoreOne(s, text, "", cmd.OutOrStdout())
	}
	over := false
	for _, path := range args {
		text, err := readInput(path, cmd.InOrStdin())
		if err != nil {
			return err
		}
		switch err := scoreOne(s, text, path, cmd.OutOrStdout()); {
		case errors.Is(err, errFindings):
			over = true
		case err != nil:
			return err
		}
	}
	if over {
		return errFindings
	}
	return nil
}

// scoreOne scores one input and writes the result to stdout. It returns errFindings when
// the score is above the --max gate, so a run can fail CI on slop.
func scoreOne(s *sanitize.Sanitizer, text, path string, stdout io.Writer) error {
	score := s.Score(text)
	if config.JSON() {
		if err := writeJSON(stdout, score, config.Pretty()); err != nil {
			return err
		}
	} else if path != "" {
		if _, err := fmt.Fprintf(stdout, "%s: %d\n", path, score.Value); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(stdout, "%d\n", score.Value); err != nil {
		return err
	}
	if max := config.Max(); max >= 0 && score.Value > max {
		return errFindings
	}
	return nil
}
