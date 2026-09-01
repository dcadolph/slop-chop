package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	cmd.AddCommand(voiceInitCmd(), voiceShowCmd(), voiceLearnCmd(), voiceDiffCmd(),
		voiceFingerprintCmd(), voiceDriftCmd())
	return cmd
}

// openVoiceForWrite returns the voice to edit and the path it came from, creating neither.
// A missing file is an empty voice, so the first write scaffolds it.
func openVoiceForWrite() (sanitize.Voice, string, error) {
	path := resolveVoicePath()
	if path == "" {
		p, err := defaultVoicePath()
		if err != nil {
			return sanitize.Voice{}, "", err
		}
		path = p
	}
	if _, err := os.Stat(path); err != nil {
		return sanitize.Voice{}, path, nil
	}
	v, err := sanitize.LoadVoiceFile(path)
	if err != nil {
		return sanitize.Voice{}, "", err
	}
	return v, path, nil
}

// saveVoice writes the voice to path as indented JSON, creating its directory.
func saveVoice(v sanitize.Voice, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create voice dir: %w", err)
	}
	b, err := jsonutil.Marshal(v, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write voice file: %w", err)
	}
	return nil
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

	voice, path, err := openVoiceForWrite()
	if err != nil {
		return err
	}
	voice.Tone = mergeToneNotes(voice.Tone, notes)
	if err := saveVoice(voice, path); err != nil {
		return err
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
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("usage: slop-chop voice diff <draft> <final>")
			}
			return nil
		},
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
		suggested, _ := suggestedVoice(candidates)
		report := diffReport{
			Candidates: jsonutil.OrEmpty(candidates),
			Suggested:  suggested,
		}
		return writeJSON(cmd.OutOrStdout(), report, config.Pretty())
	}
	writeDiffReport(cmd.OutOrStdout(), candidates, args[0], args[1])
	return nil
}

// suggestedVoice turns the proposals into the voice they would add, and separately
// names the words whose edits conflict. A kept tell becomes a keep entry, and a change
// nothing flagged becomes a prefer entry, where an empty replacement drops the word. A
// confirmation adds nothing, since the rule already exists, and a word edited two
// different ways is reported rather than silently resolved by document order.
func suggestedVoice(candidates []sanitize.Candidate) (sanitize.Voice, []string) {
	var v sanitize.Voice
	conflict := map[string]bool{}
	for _, c := range candidates {
		switch c.Kind {
		case sanitize.CandidateKeep:
			v.Keep = append(v.Keep, c.Match)
		case sanitize.CandidateNew:
			if v.Prefer == nil {
				v.Prefer = make(map[string]string)
			}
			key := strings.ToLower(c.Pair.Was)
			if prev, ok := v.Prefer[key]; ok && prev != c.Pair.Now {
				conflict[key] = true
				continue
			}
			v.Prefer[key] = c.Pair.Now
		case sanitize.CandidateConfirms:
		}
	}
	var conflicts []string
	for w := range conflict {
		delete(v.Prefer, w)
		conflicts = append(conflicts, w)
	}
	slices.Sort(conflicts)
	return v, conflicts
}

// writeDiffReport prints the proposals grouped by what they mean, keeps first, since a
// tell you read and shipped is the rules being wrong rather than a fact about your voice.
func writeDiffReport(w io.Writer, candidates []sanitize.Candidate, draftPath, finalPath string) {
	if len(candidates) == 0 {
		_, _ = fmt.Fprintf(w, "no proposals from %s and %s: identical texts, fact fixes, moves,\n"+
			"case or punctuation edits, insertions, and large restructures are not turned into rules\n",
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
	v, conflicts := suggestedVoice(candidates)
	for _, c := range conflicts {
		_, _ = fmt.Fprintf(w, "\nedited two different ways, not proposed: %q\n", c)
	}
	if v.Empty() {
		return
	}
	b, err := jsonutil.Marshal(v, true)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "\nmerge what you agree with into ~/.slop-chop/voice.json\n"+
		"(create one with 'slop-chop voice init'):\n%s\n", string(b))
}

// voiceFingerprintCmd builds the voice fingerprint subcommand, which measures how you
// write and stores the numbers in your voice file.
func voiceFingerprintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fingerprint [file ...]",
		Short: "Measure how you write and store the numbers in your voice.",
		Long: `fingerprint reads things you wrote, from files or stdin, and measures how you
write rather than what you write: sentence rhythm, punctuation habits, and register. The
numbers land in your voice file, where voice drift reads them.

keep, prefer, and avoid hold your words, and a model that reads them can hand them back
to you. Nobody quotes a comma rate, so these numbers stay yours. Feed it several pieces
of your own finished writing and no machine drafts, since a fingerprint taken from a
model's prose measures the model.`,
		RunE: runVoiceFingerprint,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	return cmd
}

// runVoiceFingerprint measures the samples, stores the fingerprint in the voice file, and
// reports what it measured.
func runVoiceFingerprint(cmd *cobra.Command, args []string) error {
	var samples []string
	if len(args) == 0 {
		text, err := readInput("", cmd.InOrStdin())
		if err != nil {
			return err
		}
		samples = append(samples, text)
	}
	for _, path := range args {
		text, ok, err := readProse(path, cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		samples = append(samples, text)
	}
	f, err := sanitize.NewFingerprint(samples...)
	if err != nil {
		return err
	}
	voice, path, err := openVoiceForWrite()
	if err != nil {
		return err
	}
	voice.Fingerprint = &f
	if err := saveVoice(voice, path); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "measured %d words in %d sentences across %d sample(s) into %s\n",
		f.Words, f.Sentences, f.Samples, path)
	if config.JSON() {
		return writeJSON(cmd.OutOrStdout(), f, config.Pretty())
	}
	writeFingerprint(cmd.OutOrStdout(), f)
	return nil
}

// writeFingerprint prints one line per trait: the measured value, the band it may move
// inside, and what the number counts.
func writeFingerprint(w io.Writer, f sanitize.Fingerprint) {
	for _, m := range sanitize.MetricList() {
		v, ok := f.Metrics[m.Name]
		if !ok {
			continue
		}
		_, _ = fmt.Fprintf(w, "  %-18s %8.2f  give or take %.2f  %s\n", m.Name, v.Value, v.Band, m.Unit)
	}
}

// voiceDriftCmd builds the voice drift subcommand, which reports where a text stops
// sounding like the writer the fingerprint was taken from.
func voiceDriftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift [file ...]",
		Short: "Report where a text stops sounding like you.",
		Long: `drift measures a text against the fingerprint in your voice file and names
every trait that landed outside your range: sentences longer than you write, a heavier
vocabulary, a register that is more formal than yours.

Drift is not slop. A trait outside your range can be good writing that belongs to
somebody else, which is exactly what a model hands you, so drift names the difference and
leaves the verdict to you. Take a fingerprint first with voice fingerprint. --bands fails
the run when a trait lands that many bands out, which gates a house voice in CI.`,
		Args: cobra.ArbitraryArgs,
		RunE: runVoiceDrift,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagJSON)
	f.AddFlag(&config.FlagPretty)
	f.AddFlag(&config.FlagBands)
	return cmd
}

// runVoiceDrift compares stdin or every file argument against the stored fingerprint and
// returns errFindings when a trait drifts past the --bands gate.
func runVoiceDrift(cmd *cobra.Command, args []string) error {
	if config.JSON() && len(args) > 1 {
		return fmt.Errorf("--json takes at most one file")
	}
	path := resolveVoicePath()
	if path == "" {
		return fmt.Errorf("no voice set: measure your writing with `slop-chop voice fingerprint <file ...>` first")
	}
	v, err := sanitize.LoadVoiceFile(path)
	if err != nil {
		return err
	}
	if v.Fingerprint == nil || v.Fingerprint.Empty() {
		return fmt.Errorf("no fingerprint in %s: measure your writing with "+
			"`slop-chop voice fingerprint <file ...>` first", path)
	}
	if len(args) == 0 {
		text, err := readInput("", cmd.InOrStdin())
		if err != nil {
			return err
		}
		return driftOne(*v.Fingerprint, text, "", cmd.OutOrStdout())
	}
	out := false
	for _, file := range args {
		text, ok, err := readProse(file, cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		switch err := driftOne(*v.Fingerprint, text, file, cmd.OutOrStdout()); {
		case errors.Is(err, errFindings):
			out = true
		case err != nil:
			return err
		}
	}
	if out {
		return errFindings
	}
	return nil
}

// driftOne compares one text against the fingerprint and writes the report. It returns
// errFindings when a trait lands further out than the --bands gate allows.
func driftOne(f sanitize.Fingerprint, text, path string, stdout io.Writer) error {
	drifts, err := f.Compare(text)
	if err != nil {
		return err
	}
	if config.JSON() {
		if err := writeJSON(stdout, struct {
			Drift []sanitize.Drift `json:"drift"`
		}{jsonutil.OrEmpty(drifts)}, config.Pretty()); err != nil {
			return err
		}
	} else {
		writeDrift(stdout, drifts, path, len(f.Metrics))
	}
	if bands := config.Bands(); bands >= 0 {
		for _, d := range drifts {
			if d.Off > float64(bands) {
				return errFindings
			}
		}
	}
	return nil
}

// writeDrift prints the drifted traits, the furthest out first, or says the text reads
// like the writer when none did.
func writeDrift(w io.Writer, drifts []sanitize.Drift, path string, traits int) {
	subject := "this text"
	if path != "" {
		subject = path
	}
	if len(drifts) == 0 {
		_, _ = fmt.Fprintf(w, "%s reads like you on all %d traits\n", subject, traits)
		return
	}
	_, _ = fmt.Fprintf(w, "%s reads unlike you on %d of %d traits\n", subject, len(drifts), traits)
	for _, d := range drifts {
		_, _ = fmt.Fprintf(w, "  %-44s %.2f against your %.2f (%s)\n", d.Note, d.Got, d.Want, d.Unit)
	}
}
