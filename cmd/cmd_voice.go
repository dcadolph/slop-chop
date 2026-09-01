package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/rewrite"
	"github.com/dcadolph/slop-chop/rewrite/prompt"
	"github.com/dcadolph/slop-chop/sanitize"
)

// voiceExample is the starter voice written by `voice init`. It is valid JSON with sample
// entries that show the shape: keep protects your words, prefer swaps them, avoid flags them.
//
//nolint:gochecknoglobals // Scaffold content, read once.
var voiceExample = sanitize.Voice{
	Keep:   []string{"ship it", "gnarly"},
	Prefer: map[string]string{"utilize": "use", "a myriad of": "a bunch of"},
	Avoid:  []string{"synergy", "circle back"},
	Tone:   []string{"short, direct sentences", "no marketing voice"},
}

// voiceCmd builds the voice subcommand, which manages the personal keep, prefer, and avoid
// lists that make output sound like you.
func voiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage your personal voice: keep, prefer, avoid, and tone.",
		Long: `voice manages a personal style file.

keep protects words and phrases so no rule or preset cuts them. prefer swaps a word or
phrase to the one you want, and an empty replacement drops it. avoid flags your own words
wherever they appear. tone holds short notes on how you write, which the --rewrite pass
matches; write them by hand or derive them from your own writing with voice learn. The
file lives at ~/.slop-chop/voice.json and applies to every run; --voice points at a
different one, and a project's .slop-chop.json still outranks it.`,
	}
	cmd.AddCommand(voiceInitCmd(), voiceShowCmd(), voiceLearnCmd(), voiceDiffCmd())
	return cmd
}

// voiceInitCmd builds the voice init subcommand, which writes a starter voice file.
func voiceInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Write a starter voice file (default ~/.slop-chop/voice.json).",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runVoiceInit,
	}
	cmd.Flags().Bool("force", false, "Overwrite an existing voice file.")
	return cmd
}

// runVoiceInit writes the starter voice to the given path or the personal default, refusing
// to clobber an existing file unless --force is set.
func runVoiceInit(cmd *cobra.Command, args []string) error {
	path := ""
	if len(args) == 1 {
		path = args[0]
	} else {
		p, err := defaultVoicePath()
		if err != nil {
			return err
		}
		path = p
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("voice file already exists at %s: pass --force to overwrite", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create voice dir: %w", err)
	}
	b, err := jsonutil.Marshal(voiceExample, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write voice file: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", path)
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
		"edit keep, prefer, and avoid, then it applies to every run.")
	return nil
}

// voiceShowCmd builds the voice show subcommand, which prints the resolved voice.
func voiceShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved voice and where it came from.",
		Args:  cobra.NoArgs,
		RunE:  runVoiceShow,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagPretty)
	return cmd
}

// runVoiceShow resolves the voice and writes it as JSON to stdout, with its source path on
// stderr. When no voice is set it says so and exits zero.
func runVoiceShow(cmd *cobra.Command, _ []string) error {
	path := resolveVoicePath()
	if path == "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"no voice set: run `slop-chop voice init` to create ~/.slop-chop/voice.json.")
		return nil
	}
	v, err := sanitize.LoadVoiceFile(path)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "voice: %s\n", path)
	return writeJSON(cmd.OutOrStdout(), v, config.Pretty())
}

// maxLearnBytes caps how much sample text one learn call sends to the model.
const maxLearnBytes = 32 * 1024

// learnPass asks the model to derive tone notes from writing samples. It is a variable so
// tests can swap in a fake model.
//
//nolint:gochecknoglobals // Test seam.
var learnPass = func(ctx context.Context, c rewrite.Completer, samples string) (string, error) {
	return c.Complete(ctx, prompt.Learn(), samples)
}

// voiceLearnCmd builds the voice learn subcommand, which derives tone notes from writing
// samples.
func voiceLearnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learn [file ...]",
		Short: "Derive tone notes from samples of your writing.",
		Long: `learn reads samples of your writing, from files or stdin, and asks the
configured model to describe your voice as short tone notes. The notes are merged into
your voice file's tone list, which the --rewrite pass matches so output sounds like you.
It needs the same provider setup as fix --rewrite.`,
		RunE: runVoiceLearn,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagProvider)
	f.AddFlag(&config.FlagModel)
	f.AddFlag(&config.FlagBaseURL)
	return cmd
}

// runVoiceLearn reads the samples, derives tone notes, and merges them into the voice file.
func runVoiceLearn(cmd *cobra.Command, args []string) error {
	var sb strings.Builder
	if len(args) == 0 {
		text, err := readInput("", cmd.InOrStdin())
		if err != nil {
			return err
		}
		sb.WriteString(text)
	}
	for _, path := range args {
		text, err := readInput(path, cmd.InOrStdin())
		if err != nil {
			return err
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	samples := strings.TrimSpace(sb.String())
	if samples == "" {
		return fmt.Errorf("no samples: pass files or pipe your writing on stdin")
	}
	if len(samples) > maxLearnBytes {
		samples = samples[:maxLearnBytes]
	}

	completer, err := newRewriteCompleter()
	if err != nil {
		return err
	}
	reply, err := learnPass(cmd.Context(), completer, samples)
	if err != nil {
		return fmt.Errorf("learn failed: %w", err)
	}
	notes, err := parseToneNotes(reply)
	if err != nil {
		return err
	}

	path := resolveVoicePath()
	if path == "" {
		p, err := defaultVoicePath()
		if err != nil {
			return err
		}
		path = p
	}
	voice := sanitize.Voice{}
	if _, err := os.Stat(path); err == nil {
		v, err := sanitize.LoadVoiceFile(path)
		if err != nil {
			return err
		}
		voice = v
	}
	voice.Tone = mergeToneNotes(voice.Tone, notes)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create voice dir: %w", err)
	}
	b, err := jsonutil.Marshal(voice, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write voice file: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "learned %d tone note(s) into %s\n", len(notes), path)
	for _, n := range notes {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "- %s\n", n)
	}
	return nil
}

// parseToneNotes pulls the JSON array of tone notes out of a model reply, tolerating prose
// or fences around it.
func parseToneNotes(reply string) ([]string, error) {
	start := strings.Index(reply, "[")
	end := strings.LastIndex(reply, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("learn reply had no JSON array: %q", reply)
	}
	var notes []string
	if err := json.Unmarshal([]byte(reply[start:end+1]), &notes); err != nil {
		return nil, fmt.Errorf("learn reply decode: %w", err)
	}
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("learn reply had no tone notes")
	}
	return out, nil
}

// mergeToneNotes appends the new notes to the existing ones, dropping duplicates
// case-insensitively so a re-learn does not stack the same lines.
func mergeToneNotes(existing, notes []string) []string {
	seen := make(map[string]bool, len(existing)+len(notes))
	out := make([]string, 0, len(existing)+len(notes))
	for _, n := range append(append([]string{}, existing...), notes...) {
		key := strings.ToLower(strings.TrimSpace(n))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, n)
	}
	return out
}

// diffReport is the JSON shape returned by voice diff.
type diffReport struct {
	// Candidates is every proposal drawn from the two files, in report order.
	Candidates []sanitize.Candidate `json:"candidates"`
	// Suggested is the voice those proposals would add, ready to merge by hand.
	Suggested sanitize.Voice `json:"suggested"`
}

// voiceDiffCmd builds the voice diff subcommand, which reads a draft against the text you
// shipped and proposes voice entries from what you changed.
func voiceDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <draft> <final>",
		Short: "Propose voice entries from what you changed in a draft.",
		Long: `diff reads a draft and the version you shipped and reports what your edits
say about how you write.

Your edits are the one voice signal a model cannot contaminate: the draft's words are
its own, but every change to them is yours. A change whose two sides carry different
numbers, links, or acronyms is treated as a corrected fact and ignored, so only wording
is read as style.

It proposes and never writes. Merge the entries you agree with into your voice file.`,
		Args: cobra.ExactArgs(2),
		RunE: runVoiceDiff,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	return cmd
}

// runVoiceDiff reads the two files, classifies the changes between them, and writes the
// proposals to stdout.
func runVoiceDiff(cmd *cobra.Command, args []string) error {
	draft, err := readInput(args[0], cmd.InOrStdin())
	if err != nil {
		return err
	}
	final, err := readInput(args[1], cmd.InOrStdin())
	if err != nil {
		return err
	}
	s, _, err := newSanitizer()
	if err != nil {
		return err
	}
	candidates := s.Candidates(draft, final)
	if config.JSON() {
		report := diffReport{
			Candidates: jsonutil.OrEmpty(candidates),
			Suggested:  suggestedVoice(candidates),
		}
		return writeJSON(cmd.OutOrStdout(), report, config.Pretty())
	}
	writeDiffReport(cmd.OutOrStdout(), candidates, args[0], args[1])
	return nil
}

// suggestedVoice turns the proposals into the voice they would add. A kept tell becomes a
// keep entry, and a change nothing flagged becomes a prefer entry, where an empty
// replacement drops the word. A confirmation adds nothing, since the rule already exists.
func suggestedVoice(candidates []sanitize.Candidate) sanitize.Voice {
	var v sanitize.Voice
	for _, c := range candidates {
		switch c.Kind {
		case sanitize.CandidateKeep:
			v.Keep = append(v.Keep, c.Match)
		case sanitize.CandidateNew:
			if v.Prefer == nil {
				v.Prefer = make(map[string]string)
			}
			v.Prefer[c.Pair.Was] = c.Pair.Now
		case sanitize.CandidateConfirms:
		}
	}
	return v
}

// writeDiffReport prints the proposals grouped by what they mean, keeps first, since a
// tell you read and shipped is the rules being wrong rather than a fact about your voice.
func writeDiffReport(w io.Writer, candidates []sanitize.Candidate, draftPath, finalPath string) {
	if len(candidates) == 0 {
		_, _ = fmt.Fprintf(w, "no candidates: %s and %s differ only in facts or not at all\n",
			draftPath, finalPath)
		return
	}
	groups := []struct {
		Kind  sanitize.CandidateKind
		Title string
	}{
		{sanitize.CandidateKeep, "keep: the rules flag these and you shipped them anyway"},
		{sanitize.CandidateNew, "prefer: you changed these and no rule caught them"},
		{sanitize.CandidateConfirms, "confirms: you cut what the rules already flag"},
	}
	for _, g := range groups {
		var rows []sanitize.Candidate
		for _, c := range candidates {
			if c.Kind == g.Kind {
				rows = append(rows, c)
			}
		}
		if len(rows) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "\n%s\n", g.Title)
		for _, c := range rows {
			switch {
			case c.Kind == sanitize.CandidateKeep:
				_, _ = fmt.Fprintf(w, "  %-24s %q\n", c.Rule, c.Match)
			case c.Pair.Now == "":
				_, _ = fmt.Fprintf(w, "  %-24s %q -> (cut)\n", c.Rule, c.Pair.Was)
			default:
				_, _ = fmt.Fprintf(w, "  %-24s %q -> %q\n", c.Rule, c.Pair.Was, c.Pair.Now)
			}
		}
	}
	v := suggestedVoice(candidates)
	if v.Empty() {
		return
	}
	b, err := jsonutil.Marshal(v, true)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "\nmerge what you agree with into your voice file:\n%s\n", string(b))
}
