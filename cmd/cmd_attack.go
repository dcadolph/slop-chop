package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/sanitize"
)

// attackCmd builds the attack subcommand, which rewrites text to dodge the rules and
// reports what still fires.
func attackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attack [file ...]",
		Short: "Rewrite text to dodge the rules and report what still fires.",
		Long: `attack is the engine pointed at itself. It rewrites text to evade as many
rules as it can without improving the writing: a listed buzzword becomes an unlisted
one, a stock opener becomes an unstocked one, an em-dash becomes punctuation no rule
reads. Then it reports what survived.

What survives is the point. A word list is a lookup and loses to a thesaurus. A sentence
shape has to be rebuilt to escape it, which no substitution can do, and that is why a
structural tell counts double toward the score. Run this on your own corpus to find where
the rules are thin, and treat a class that evades easily as a class to strengthen.

The attack writes no files and changes no rules. Its output is a measurement.`,
		Args: cobra.ArbitraryArgs,
		RunE: runAttack,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	f.AddFlag(&config.FlagWrite)
	return cmd
}

// runAttack attacks stdin or every file argument and writes the report.
func runAttack(cmd *cobra.Command, args []string) error {
	if config.JSON() && len(args) > 1 {
		return fmt.Errorf("--json takes at most one file")
	}
	if config.Write() && config.JSON() {
		return fmt.Errorf("cannot use --write with --json")
	}
	if config.Write() && len(args) == 0 {
		return fmt.Errorf("--write needs a file argument, not stdin")
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
		return attackOne(s, text, "", cmd.OutOrStdout())
	}
	for _, path := range args {
		text, ok, err := readProse(path, cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := attackOne(s, text, path, cmd.OutOrStdout()); err != nil {
			return err
		}
	}
	return nil
}

// attackOne attacks one input and writes its report. With -w the attacked text replaces
// the file, which is how a corpus of evasive samples gets built for the benchmark.
func attackOne(s *sanitize.Sanitizer, text, path string, stdout io.Writer) error {
	res := s.Attack(text)
	if config.Write() && path != "" {
		if err := writeFile(path, res.Text); err != nil {
			return err
		}
	}
	if config.JSON() {
		return writeJSON(stdout, res, config.Pretty())
	}
	writeAttack(stdout, res, path)
	return nil
}

// writeAttack prints the report: the score movement, what the swaps dodged, what held,
// and the per-class tally the score weights rest on.
func writeAttack(w io.Writer, res sanitize.AttackResult, path string) {
	subject := "stdin"
	if path != "" {
		subject = path
	}
	_, _ = fmt.Fprintf(w, "%s: %d -> %d after %s\n", subject, res.ScoreBefore, res.ScoreAfter,
		countLabel(len(res.Evasions), "evasion"))
	for _, e := range res.Evasions {
		_, _ = fmt.Fprintf(w, "  evaded    %-28s %q -> %q\n", e.Rule, e.Was, e.Now)
	}
	for _, f := range res.Survived {
		_, _ = fmt.Fprintf(w, "  held      %-28s %q\n", f.Rule, f.Match)
	}
	if len(res.ByClass) == 0 {
		return
	}
	classes := make([]string, 0, len(res.ByClass))
	for c := range res.ByClass {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	_, _ = fmt.Fprintln(w, "  by class:")
	for _, c := range classes {
		v := res.ByClass[c]
		_, _ = fmt.Fprintf(w, "    %-12s evaded %d, held %d\n", c, v.Evaded, v.Resisted)
	}
}

// countLabel renders a count with its noun, pluralized.
func countLabel(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
