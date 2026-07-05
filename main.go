package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/internal/rewrite"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// usage describes the command line.
const usage = `slop-chop - chop the slop from text

Usage:
  slop-chop check [-profile p.json] [-json] [-pretty] [file]
  slop-chop fix   [-profile p.json] [-json] [-pretty] [-w] [-rewrite [-model id]] [file]
  slop-chop help

Flags:
  -profile path   use a JSON style profile instead of the built-in one
  -json           write JSON to stdout (findings for check, result for fix)
  -pretty         indent the JSON output
  -w              write the result back to the file instead of stdout (fix only)
  -rewrite        after the rules pass, send the text to a model for a deeper clean
  -model id       model for -rewrite (default ` + rewrite.DefaultModel + `)

check flags AI tells and exits non-zero when it finds any.
fix writes the cleaned text to stdout. It does not touch the file unless you pass -w.
The -rewrite pass needs the ANTHROPIC_API_KEY environment variable.
With no file, slop-chop reads stdin.
`

// checkReport is the JSON shape returned by check mode.
type checkReport struct {
	// Findings is every rule match in the input.
	Findings []sanitize.Finding `json:"findings"`
}

// fixReport is the JSON shape returned by fix mode.
type fixReport struct {
	// Cleaned is the rewritten text.
	Cleaned string `json:"cleaned"`
	// Findings is every rule match in the original input.
	Findings []sanitize.Finding `json:"findings"`
}

// main parses the command line and dispatches. Exit codes: 0 clean, 1 findings in
// check mode, 2 on error.
func main() {
	opts, err := parseArgs(os.Args[1:])
	switch {
	case errors.Is(err, flag.ErrHelp):
		_, _ = fmt.Fprint(os.Stdout, usage)
		return
	case err != nil:
		fmt.Fprintln(os.Stderr, "slop-chop:", err)
		fmt.Fprintln(os.Stderr, "run slop-chop help for usage")
		os.Exit(2)
	}

	// A first interrupt cancels the context so a long rewrite call unwinds cleanly. A
	// second one falls back to the default hard stop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch err := run(ctx, opts, os.Stdin, os.Stdout, os.Stderr); {
	case err == nil:
	case errors.Is(err, errFindings):
		os.Exit(1)
	default:
		fmt.Fprintln(os.Stderr, "slop-chop:", err)
		os.Exit(2)
	}
}

// runOptions holds the parsed command line for one invocation.
type runOptions struct {
	// mode is check or fix.
	mode string
	// profilePath points at a JSON profile, or is empty for the built-in one.
	profilePath string
	// file is the input path, or empty to read stdin.
	file string
	// jsonOut writes JSON instead of text or findings.
	jsonOut bool
	// pretty indents the JSON output.
	pretty bool
	// write saves the result back to the file instead of writing to stdout.
	write bool
	// rewrite runs the model pass after the rules pass.
	rewrite bool
	// model is the model id for the rewrite pass.
	model string
}

// parseArgs turns command-line arguments into runOptions, validating everything that
// can be caught before any work, like flag combinations that only fail later. It
// returns flag.ErrHelp when the user asked for help.
func parseArgs(args []string) (runOptions, error) {
	if len(args) == 0 {
		return runOptions{}, fmt.Errorf("missing mode (want check or fix)")
	}
	mode := args[0]
	switch mode {
	case "help", "-h", "--help":
		return runOptions{}, flag.ErrHelp
	case "check", "fix":
	default:
		return runOptions{}, fmt.Errorf("unknown mode %q (want check or fix)", mode)
	}

	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	profilePath := fs.String("profile", "", "path to a JSON style profile (default: built-in)")
	jsonOut := fs.Bool("json", false, "write JSON to stdout")
	pretty := fs.Bool("pretty", false, "indent the JSON output")
	doRewrite := fs.Bool("rewrite", false, "run the model rewrite pass after the rules")
	model := fs.String("model", rewrite.DefaultModel, "model for -rewrite")
	write := fs.Bool("w", false, "write the result back to the file instead of stdout")
	if err := fs.Parse(args[1:]); err != nil {
		return runOptions{}, err
	}

	if fs.NArg() > 1 {
		return runOptions{}, fmt.Errorf("too many arguments: at most one file, got %d", fs.NArg())
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	opts := runOptions{
		mode:        mode,
		profilePath: *profilePath,
		file:        fs.Arg(0),
		jsonOut:     *jsonOut,
		pretty:      *pretty,
		write:       *write,
		rewrite:     *doRewrite,
		model:       *model,
	}

	if mode == "check" {
		for _, name := range []string{"w", "rewrite", "model"} {
			if set[name] {
				return runOptions{}, fmt.Errorf("-%s is a fix flag, not a check flag", name)
			}
		}
	}
	if set["model"] && !opts.rewrite {
		return runOptions{}, fmt.Errorf("-model needs -rewrite")
	}
	if opts.write && opts.jsonOut {
		return runOptions{}, fmt.Errorf("cannot use -w with -json")
	}
	if opts.write && opts.file == "" {
		return runOptions{}, fmt.Errorf("-w needs a file argument, not stdin")
	}
	return opts, nil
}

// run executes one invocation. It returns an error for usage or IO problems, and
// errFindings when check mode finds slop, leaving the exit code to main.
func run(ctx context.Context, opts runOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	profile := sanitize.DefaultProfile()
	if opts.profilePath != "" {
		p, err := sanitize.LoadFile(opts.profilePath)
		if err != nil {
			return err
		}
		profile = p
	}

	s, err := sanitize.New(profile)
	if err != nil {
		return err
	}

	text, err := readInput(opts.file, stdin)
	if err != nil {
		return err
	}

	switch opts.mode {
	case "check":
		return runCheck(s, text, opts, stdout, stderr)
	case "fix":
		return runFix(ctx, s, profile.Tone, text, opts, stdout)
	default:
		return fmt.Errorf("unknown mode %q (want check or fix)", opts.mode)
	}
}

// runCheck reports findings and returns errFindings when there are any. Findings on a
// file are prefixed with its path, so a terminal can jump to the spot.
func runCheck(s *sanitize.Sanitizer, text string, opts runOptions, stdout, stderr io.Writer) error {
	findings := s.Check(text)
	if opts.jsonOut {
		if err := writeJSON(stdout, checkReport{Findings: orEmpty(findings)}, opts.pretty); err != nil {
			return err
		}
	} else {
		prefix := ""
		if opts.file != "" {
			prefix = opts.file + ":"
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

// runFix writes the cleaned text to stdout, back into the file with -w, or as JSON.
// With -rewrite it runs the model pass on the rules output first.
func runFix(ctx context.Context, s *sanitize.Sanitizer, tone []string, text string,
	opts runOptions, stdout io.Writer) error {
	out, findings := s.Fix(text)
	if opts.rewrite {
		rw, err := rewritePass(ctx, opts.model, tone, out)
		if err != nil {
			return err
		}
		// The rewriter trims the reply, so put back the newline the input ended with.
		if strings.HasSuffix(text, "\n") && !strings.HasSuffix(rw, "\n") {
			rw += "\n"
		}
		out = rw
	}
	if opts.jsonOut {
		return writeJSON(stdout, fixReport{Cleaned: out, Findings: orEmpty(findings)}, opts.pretty)
	}
	if opts.write {
		return writeFile(opts.file, out)
	}
	_, err := io.WriteString(stdout, out)
	return err
}

// rewritePass runs the model rewrite over text. It is a variable so tests can swap in
// a fake model.
var rewritePass = func(ctx context.Context, model string, tone []string, text string) (string, error) {
	rw := rewrite.New(rewrite.NewAnthropicCompleter(model), tone...)
	return rw.Rewrite(ctx, text)
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

// writeFile writes out back to path, keeping the file's existing mode.
func writeFile(path, out string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(out), mode); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
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
