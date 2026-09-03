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
	"github.com/dcadolph/slop-chop/sanitize"
)

// voiceDir and voiceFile name the personal voice discovered under the home directory when
// --voice is not set, so a voice applies to every run without a flag.
const (
	voiceDir  = ".slop-chop"
	voiceFile = "voice.json"
)

// defaultProfileFile is discovered from the working directory upward when --profile is
// not set, so a repo can pin its own style without every caller passing the flag.
const defaultProfileFile = ".slop-chop.json"

// discoverProfile walks from the working directory toward the filesystem root and returns
// the first .slop-chop.json it finds, or empty when there is none. Walking up is what
// makes a repo's profile hold from any of its subdirectories, the way .gitignore and
// .editorconfig do, so an editor plugin or a cd'd shell gets the same rules as a run from
// the root.
func discoverProfile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, defaultProfileFile)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// rootCmd builds the slop-chop root command with the check and fix subcommands.
func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slop-chop",
		Short: "Chop the slop from text.",
		Long: `slop-chop finds and removes AI writing tells from text.

check reports the tells and exits non-zero when it finds any. fix rewrites the text.
With no file, both read stdin. The --rewrite pass needs the ANTHROPIC_API_KEY
environment variable. When --profile is not set, the nearest .slop-chop.json from the
working directory upward extends the built-in profile, so a repo's rules hold from any
of its subdirectories.`,
		Version:       resolveVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(checkCmd(), fixCmd(), scoreCmd(), attackCmd(), tellsCmd(), voiceCmd(),
		lspCmd(), mcpCmd())
	return cmd
}

// newSanitizer loads the configured profile, falling back to a discovered
// .slop-chop.json and then the built-in one, and builds a sanitizer from it. The
// profile is returned too so fix mode can hand its tone to the rewrite pass.
func newSanitizer() (*sanitize.Sanitizer, sanitize.Profile, error) {
	return sanitizerFor(splitList(config.Preset()), config.Dialect())
}

// sanitizerFor builds a sanitizer with presetNames and dialect applied in place of the flag
// values, so a caller carrying its own per-request overrides, such as the MCP server, reuses
// this one precedence rather than repeating it. Everything else, the profile, the discovered
// project file, and the voice, resolves the same way it does for every command.
func sanitizerFor(presetNames []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error) {
	profilePath := config.Profile()
	if profilePath == "" {
		profilePath = discoverProfile()
	}
	profile := sanitize.DefaultProfile()
	var projectProfile sanitize.Profile
	haveProject := false
	if profilePath != "" {
		p, err := sanitize.LoadFile(profilePath)
		if err != nil {
			return nil, sanitize.Profile{}, err
		}
		// A profile extends the built-in default unless it says standalone. The
		// two-line team file, allow one word and ban another, means the default
		// detection plus those two decisions, not a profile of two rules.
		if !p.Standalone {
			p = sanitize.DefaultProfile().Overlay(p)
		}
		profile = p
		projectProfile = p
		haveProject = true
	}
	// The caller's dialect overrides the profile's own. Left unset, the profile's field
	// stands, so a repo can pin a dialect in .slop-chop.json.
	if dialect != "" {
		profile.Dialect = sanitize.Dialect(dialect)
	}
	// Presets add their rules on top of the profile, which still wins on any conflict.
	if len(presetNames) > 0 {
		merged, err := sanitize.ApplyPresets(profile, presetNames...)
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
// personal ~/.slop-chop/voice.json when it exists, else empty for no voice. The value
// "off" disables the voice for one run, which is how a CI script or a doc generator
// gets the profile with no personal layer.
func resolveVoicePath() string {
	if p := config.Voice(); p != "" {
		if p == "off" {
			return ""
		}
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
	// The personal voice shapes every run it touches, and two machines with different
	// voice files disagree about the same text. Saying so keeps that from reading as
	// nondeterminism.
	fmt.Fprintf(os.Stderr, "slop-chop: voice: %s (disable with --voice off)\n", p)
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
