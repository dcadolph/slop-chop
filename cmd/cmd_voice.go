package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/jsonutil"
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
}

// voiceCmd builds the voice subcommand, which manages the personal keep, prefer, and avoid
// lists that make output sound like you.
func voiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage your personal voice: keep, prefer, and avoid.",
		Long: `voice manages a personal style file of three lists.

keep protects words and phrases so no rule or preset cuts them. prefer swaps a word or
phrase to the one you want, and an empty replacement drops it. avoid flags your own words
wherever they appear. The file lives at ~/.slop-chop/voice.json and applies to every run;
--voice points at a different one, and a project's .slop-chop.json still outranks it.`,
	}
	cmd.AddCommand(voiceInitCmd(), voiceShowCmd())
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
