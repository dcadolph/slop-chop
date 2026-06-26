package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// usage describes the command line.
const usage = `slop-chop - chop the slop from text

Usage:
  slop-chop check [-profile p.json] [file]   flag AI tells, exit non-zero if any
  slop-chop fix   [-profile p.json] [file]   write cleaned text to stdout

With no file, slop-chop reads stdin.
`

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
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	if err := run(mode, *profilePath, fs.Arg(0), os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "slop-chop:", err)
		os.Exit(2)
	}
}

// run executes one invocation. It returns an error for usage or IO problems and calls
// os.Exit(1) directly when check mode finds slop.
func run(mode, profilePath, file string, stdin io.Reader, stdout, stderr io.Writer) error {
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
		for _, f := range findings {
			fmt.Fprintln(stderr, f)
		}
		if len(findings) > 0 {
			fmt.Fprintf(stderr, "slop-chop: %d finding(s)\n", len(findings))
			os.Exit(1)
		}
		return nil
	case "fix":
		out, _ := s.Fix(text)
		_, err := io.WriteString(stdout, out)
		return err
	default:
		return fmt.Errorf("unknown mode %q (want check or fix)", mode)
	}
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
