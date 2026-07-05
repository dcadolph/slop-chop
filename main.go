package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/internal/rewrite"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// errFindings signals that check mode found slop. main maps it to exit code 1 so the
// exit lifecycle stays in main and run stays testable.
var errFindings = errors.New("findings")

// usage describes the command line.
const usage = `slop-chop - chop the slop from text

Usage:
  slop-chop check [-profile p.json] [-json] [-pretty] [file]
  slop-chop fix   [-profile p.json] [-json] [-pretty] [-w] [-rewrite [-model id]] [file]

Flags:
  -profile path   use a JSON style profile instead of the built-in one
  -json           write JSON to stdout (findings for check, result for fix)
  -pretty         indent the JSON output
  -w              write the result back to the file instead of stdout (fix only)
  -rewrite        after the rules pass, send the text to a model for a deeper clean
  -model id       model for -rewrite (default claude-opus-4-8)

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

// main parses the mode and dispatches. Exit codes: 0 clean, 1 findings in check
// mode, 2 on error.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	mode := os.Args[1]
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	profilePath := fs.String("profile", "", "path to a JSON style profile (default: built-in)")
	jsonOut := fs.Bool("json", false, "write JSON to stdout")
	pretty := fs.Bool("pretty", false, "indent the JSON output")
	doRewrite := fs.Bool("rewrite", false, "run the model rewrite pass after the rules")
	model := fs.String("model", rewrite.DefaultModel, "model for -rewrite")
	write := fs.Bool("w", false, "write the result back to the file instead of stdout")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

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
	switch err := run(context.Background(), opts, os.Stdin, os.Stdout, os.Stderr); {
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
		findings := s.Check(text)
		if opts.jsonOut {
			if err := writeJSON(stdout, checkReport{Findings: orEmpty(findings)}, opts.pretty); err != nil {
				return err
			}
		} else {
			for _, f := range findings {
				_, _ = fmt.Fprintln(stderr, f)
			}
		}
		if len(findings) > 0 {
			if !opts.jsonOut {
				_, _ = fmt.Fprintf(stderr, "slop-chop: %d finding(s)\n", len(findings))
			}
			return errFindings
		}
		return nil
	case "fix":
		out, findings := s.Fix(text)
		if opts.rewrite {
			out, err = rewritePass(ctx, opts.model, profile.Tone, out)
			if err != nil {
				return err
			}
		}
		if opts.jsonOut {
			if opts.write {
				return fmt.Errorf("cannot use -w with -json")
			}
			return writeJSON(stdout, fixReport{Cleaned: out, Findings: orEmpty(findings)}, opts.pretty)
		}
		if opts.write {
			return writeFile(opts.file, out)
		}
		_, err := io.WriteString(stdout, out)
		return err
	default:
		return fmt.Errorf("unknown mode %q (want check or fix)", opts.mode)
	}
}

// rewritePass runs the model rewrite over text. It requires ANTHROPIC_API_KEY.
func rewritePass(ctx context.Context, model string, tone []string, text string) (string, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return "", fmt.Errorf("rewrite needs the ANTHROPIC_API_KEY environment variable")
	}
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

// writeFile writes out back to path, keeping the file's existing mode. It needs a real
// file, since there is nothing to write in place to when reading from stdin.
func writeFile(path, out string) error {
	if path == "" {
		return fmt.Errorf("-w needs a file argument, not stdin")
	}
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
