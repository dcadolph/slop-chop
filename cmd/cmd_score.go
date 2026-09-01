package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/sanitize"
)

// scoreCmd builds the score subcommand.
func scoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "score [file ...]",
		Short: "Score the density of AI-writing tells, 0 (none) to 100 (saturated).",
		Args:  cobra.ArbitraryArgs,
		RunE:  runScore,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	f.AddFlag(&config.FlagMax)
	f.AddFlag(&config.FlagByParagraph)
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
		text, ok, err := readProse(path, cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if !ok {
			continue
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

// scoreBand names the band a score falls in, the same scale the web app shows, so every
// surface reads the number the same way.
func scoreBand(v int) string {
	switch {
	case v < 25:
		return "reads clean"
	case v < 55:
		return "mixed"
	}
	return "heavy slop"
}

// scoreOne scores one input and writes the result to stdout. It returns errFindings when
// the score is above the --max gate, so a run can fail CI on slop. With --by-paragraph
// each paragraph is scored on its own, which is how a mixed document shows which part
// the machine wrote, and the gate applies to the hottest paragraph rather than the
// diluted whole.
func scoreOne(s *sanitize.Sanitizer, text, path string, stdout io.Writer) error {
	if config.ByParagraph() {
		return scoreParagraphs(s, text, path, stdout)
	}
	score := s.Score(text)
	if config.JSON() {
		if err := writeJSON(stdout, score, config.Pretty()); err != nil {
			return err
		}
	} else if path != "" {
		if _, err := fmt.Fprintf(stdout, "%s: %d (%s: %d tells in %d words)\n",
			path, score.Value, scoreBand(score.Value), score.Tells, score.Words); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(stdout, "%d (%s: %d tells in %d words)\n",
		score.Value, scoreBand(score.Value), score.Tells, score.Words); err != nil {
		return err
	}
	if max := config.Max(); max >= 0 && score.Value > max {
		return errFindings
	}
	return nil
}

// scoreParagraphs writes one line per scored paragraph and gates on the hottest one.
func scoreParagraphs(s *sanitize.Sanitizer, text, path string, stdout io.Writer) error {
	paras := s.ScoreByParagraph(text)
	if config.JSON() {
		if err := writeJSON(stdout, struct {
			Paragraphs []sanitize.ParagraphScore `json:"paragraphs"`
		}{jsonutil.OrEmpty(paras)}, config.Pretty()); err != nil {
			return err
		}
	} else {
		prefix := ""
		if path != "" {
			prefix = path + ":"
		}
		for _, p := range paras {
			if _, err := fmt.Fprintf(stdout, "%s%d: %d (%s: %d tells in %d words)\n",
				prefix, p.Line, p.Score.Value, scoreBand(p.Score.Value), p.Score.Tells, p.Words); err != nil {
				return err
			}
		}
	}
	if max := config.Max(); max >= 0 {
		for _, p := range paras {
			if p.Score.Value > max {
				return errFindings
			}
		}
	}
	return nil
}
