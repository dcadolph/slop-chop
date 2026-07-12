package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// voiceDir and voiceFile name the personal voice discovered under the home directory when
// --voice is not set, so a voice applies to every run without a flag.
const (
	voiceDir  = ".slop-chop"
	voiceFile = "voice.json"
)

// defaultProfileFile is picked up from the working directory when --profile is not set,
// so a repo can pin its own style without every caller passing the flag.
const defaultProfileFile = ".slop-chop.json"

// rootCmd builds the slop-chop root command with the check and fix subcommands.
func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slop-chop",
		Short: "Chop the slop from text.",
		Long: `slop-chop finds and removes AI writing tells from text.

check reports the tells and exits non-zero when it finds any. fix rewrites the text.
With no file, both read stdin. The --rewrite pass needs the ANTHROPIC_API_KEY
environment variable. When --profile is not set and a .slop-chop.json file sits in the
working directory, that profile is used instead of the built-in one.`,
		Version:       resolveVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(checkCmd(), fixCmd(), scoreCmd(), tellsCmd(), voiceCmd())
	return cmd
}

// newSanitizer loads the configured profile, falling back to a discovered
// .slop-chop.json and then the built-in one, and builds a sanitizer from it. The
// profile is returned too so fix mode can hand its tone to the rewrite pass.
func newSanitizer() (*sanitize.Sanitizer, sanitize.Profile, error) {
	profilePath := config.Profile()
	if profilePath == "" {
		if _, err := os.Stat(defaultProfileFile); err == nil {
			profilePath = defaultProfileFile
		}
	}
	profile := sanitize.DefaultProfile()
	var projectProfile sanitize.Profile
	haveProject := false
	if profilePath != "" {
		p, err := sanitize.LoadFile(profilePath)
		if err != nil {
			return nil, sanitize.Profile{}, err
		}
		profile = p
		projectProfile = p
		haveProject = true
	}
	// The flag and its env var override the profile's own dialect. Left unset, the
	// profile's field stands, so a repo can pin a dialect in .slop-chop.json.
	if d := config.Dialect(); d != "" {
		profile.Dialect = sanitize.Dialect(d)
	}
	// Presets add their rules on top of the profile, which still wins on any conflict.
	if names := splitList(config.Preset()); len(names) > 0 {
		merged, err := sanitize.ApplyPresets(profile, names...)
		if err != nil {
			return nil, sanitize.Profile{}, err
		}
		profile = merged
	}
	// A voice overrides presets: your prefer swaps win and your keep list silences their
	// cuts. A project profile still outranks a voice, so it is re-applied on top.
	voice, err := loadVoice()
	if err != nil {
		return nil, sanitize.Profile{}, err
	}
	if !voice.Empty() {
		profile = profile.WithVoice(voice)
		if haveProject {
			profile = profile.Overlay(projectProfile)
		}
	}
	s, err := sanitize.New(profile)
	if err != nil {
		return nil, sanitize.Profile{}, err
	}
	return s, profile, nil
}

// resolveVoicePath returns the voice file to use: the --voice flag when set, else the
// personal ~/.slop-chop/voice.json when it exists, else empty for no voice.
func resolveVoicePath() string {
	if p := config.Voice(); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, voiceDir, voiceFile)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// loadVoice returns the resolved voice, or the zero Voice when none is set. A missing voice
// is not an error; callers treat the zero Voice as a no-op.
func loadVoice() (sanitize.Voice, error) {
	path := resolveVoicePath()
	if path == "" {
		return sanitize.Voice{}, nil
	}
	return sanitize.LoadVoiceFile(path)
}

// defaultVoicePath returns ~/.slop-chop/voice.json, the personal voice location written by
// `voice init` and discovered when --voice is unset.
func defaultVoicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home dir: %w", err)
	}
	return filepath.Join(home, voiceDir, voiceFile), nil
}

// splitList splits a comma-separated flag value into trimmed, non-empty items.
func splitList(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// readInput returns the text from file, or from stdin when file is empty.
func readInput(file string, stdin io.Reader) (string, error) {
	if file == "" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(b), nil
}

// writeJSON marshals v and writes it to w with a trailing newline.
func writeJSON(w io.Writer, v any, pretty bool) error {
	b, err := jsonutil.Marshal(v, pretty)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// orEmpty returns a non-nil slice so JSON shows an empty array instead of null.
func orEmpty(f []sanitize.Finding) []sanitize.Finding {
	if f == nil {
		return []sanitize.Finding{}
	}
	return f
}
