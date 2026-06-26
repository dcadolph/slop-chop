package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dcadolph/slop-chop/internal/jsonutil"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// usage describes the command line.
const usage = `slop-chop - chop the slop from text

Usage:
  slop-chop check [-profile p.json] [-json] [-pretty] [file]
  slop-chop fix   [-profile p.json] [-json] [-pretty] [file]

Flags:
  -profile path   use a JSON style profile instead of the built-in one
  -json           write JSON to stdout (findings for check, result for fix)
  -pretty         indent the JSON output

check flags AI tells and exits non-zero when it finds any.
fix writes the cleaned text to stdout.
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
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	if err := run(mode, *profilePath, fs.Arg(0), *jsonOut, *pretty,
		os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "slop-chop:", err)
		os.Exit(2)
	}
}

// run executes one invocation. It returns an error for usage or IO problems and calls
// os.Exit(1) directly when check mode finds slop.
func run(mode, profilePath, file string, jsonOut, pretty bool,
	stdin io.Reader, stdout, stderr io.Writer) error {
	profile := sanitize.DefaultProfile()
	if profilePath != "" {
		p, err := sanitize.LoadFile(profilePath)
		if err != nil {
			return err
		}
		profile = p
	}

	s, err := sanitize.New(profile)
	if err != nil {
		return err
	}

	text, err := readInput(file, stdin)
	if err != nil {
		return err
	}

	switch mode {
	case "check":
		findings := s.Check(text)
		if jsonOut {
			if err := writeJSON(stdout, checkReport{Findings: orEmpty(findings)}, pretty); err != nil {
				return err
			}
		} else {
			for _, f := range findings {
				fmt.Fprintln(stderr, f)
			}
		}
		if len(findings) > 0 {
			if !jsonOut {
				fmt.Fprintf(stderr, "slop-chop: %d finding(s)\n", len(findings))
			}
			os.Exit(1)
		}
		return nil
	case "fix":
		out, findings := s.Fix(text)
		if jsonOut {
			return writeJSON(stdout, fixReport{Cleaned: out, Findings: orEmpty(findings)}, pretty)
		}
		_, err := io.WriteString(stdout, out)
		return err
	default:
		return fmt.Errorf("unknown mode %q (want check or fix)", mode)
	}
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
