package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/internal/rewrite"
	"github.com/dcadolph/slop-chop/internal/rewrite/prompt"
	"github.com/dcadolph/slop-chop/internal/sanitize"
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
	cmd.AddCommand(voiceInitCmd(), voiceShowCmd(), voiceLearnCmd())
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
