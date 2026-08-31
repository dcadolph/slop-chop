package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dcadolph/slop-chop/sanitize"
)

// sourceExts lists file extensions whose content is source code or machine-read data
// rather than prose. The engine's cleanups are prose rules: a semicolon split or an
// orphan-comma strip applied to code breaks it, so these files are skipped instead of
// corrupted. Piping a file through stdin bypasses the gate for the rare deliberate run.
//
//nolint:gochecknoglobals // Immutable lookup.
var sourceExts = map[string]bool{
	".asm": true, ".bash": true, ".c": true, ".cc": true, ".clj": true, ".cpp": true,
	".cs": true, ".css": true, ".csv": true, ".dart": true, ".erl": true, ".ex": true,
	".exs": true, ".fish": true, ".go": true, ".h": true, ".hcl": true, ".hpp": true,
	".hs": true, ".ini": true, ".java": true, ".jl": true, ".js": true, ".json": true,
	".jsonc": true, ".jsx": true, ".kt": true, ".lua": true, ".m": true, ".ml": true,
	".mm": true, ".nim": true, ".php": true, ".pl": true, ".proto": true, ".py": true,
	".r": true, ".rb": true, ".rs": true, ".scala": true, ".sh": true, ".sql": true,
	".swift": true, ".tf": true, ".toml": true, ".ts": true, ".tsx": true, ".tsv": true,
	".vim": true, ".yaml": true, ".yml": true, ".zig": true, ".zsh": true,
}

// sourceNames lists extensionless file names that are code or build configuration.
//
//nolint:gochecknoglobals // Immutable lookup.
var sourceNames = map[string]bool{
	"dockerfile": true, "makefile": true, "gnumakefile": true, "rakefile": true,
	"gemfile": true, "justfile": true,
}

// sourceFile reports whether path names a source code or data file the prose rules must
// not touch.
func sourceFile(path string) bool {
	if sourceExts[strings.ToLower(filepath.Ext(path))] {
		return true
	}
	return sourceNames[strings.ToLower(filepath.Base(path))]
}

// warnSourceSkip tells the user a file was skipped as source code and how to run it
// anyway.
func warnSourceSkip(stderr io.Writer, path string) {
	_, _ = fmt.Fprintf(stderr,
		"slop-chop: skipping %s: source code, not prose; pipe it through stdin to run anyway\n", path)
}

// readProse returns the text of path ready for the prose rules: a prose file as it is,
// and a source file as its comment mask, so check and score read the comments without
// exposing the code. ok is false when the file was skipped because its type has no
// comments to scan. An empty path reads stdin unmasked.
func readProse(path string, stdin io.Reader, stderr io.Writer) (text string, ok bool, err error) {
	text, err = readInput(path, stdin)
	if err != nil {
		return "", false, err
	}
	if path == "" || !sourceFile(path) {
		return text, true, nil
	}
	masked, scannable := maskSource(path, text)
	if !scannable {
		warnSourceSkip(stderr, path)
		return "", false, nil
	}
	return masked, true, nil
}

// dropTidyFindings removes cleanup findings from a source file's report. The comments
// are scanned for tells, and a spacing or punctuation cleanup inside one is neither a
// tell nor something fix will ever rewrite there.
func dropTidyFindings(findings []sanitize.Finding) []sanitize.Finding {
	out := findings[:0]
	for _, f := range findings {
		if !sanitize.TidyRule(f.Rule) {
			out = append(out, f)
		}
	}
	return out
}
