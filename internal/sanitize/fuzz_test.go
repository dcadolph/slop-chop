package sanitize

import (
	"testing"
	"unicode/utf8"
)

// fuzzSeeds are inputs that exercise the tricky corners: code fences, inline spans, smart
// characters, phrase openers, semicolons, table rows, hard breaks, control bytes, and
// multibyte runes. The fuzzer mutates these to reach the rest.
//
//nolint:gochecknoglobals // Shared seed corpus for the fuzz targets.
var fuzzSeeds = []string{
	"",
	"plain text",
	"In summary, a comprehensive—robust plan; it works.",
	"“smart” quotes and … ellipsis",
	"```\ncode — stays; here\n```\ntext — changes",
	"run `a; b` now and ` unclosed",
	"a    b   c   ",
	"line one  \nline two",
	"| a  | b |\n| c | d |",
	"it's worth noting that\nit works",
	"multi\n\nparagraph\n\nblocks",
	"\x00\x01 control \t bytes",
	"emoji 😀 and 汉字 mixed",
	"; leading semicolon then word",
	"trailing em-dash —",
	"needless to say, 42 wins",
	"word;\nword; and list; items",
}

// buildFuzzSanitizer builds a sanitizer over a profile that turns on every rewriting rule,
// so the fuzzer reaches word swaps, regex swaps, spelling, and the ignore and allow paths,
// not only the defaults.
func buildFuzzSanitizer(tb testing.TB) *Sanitizer {
	tb.Helper()
	p := DefaultProfile()
	p.Dialect = DialectAmerican
	p.WordReplace = map[string]string{"utilize": "use"}
	p.RegexReplace = map[string]string{"([0-9]+) ?%": "$1 percent"}
	p.Allow = []string{"robust"}
	s, err := New(p)
	if err != nil {
		tb.Fatalf("New: %v", err)
	}
	return s
}

// FuzzFix checks the invariants that must hold for any input: the engine never panics,
// every finding sits at the offset it reports, and valid UTF-8 in stays valid UTF-8 out.
func FuzzFix(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	s := buildFuzzSanitizer(f)

	f.Fuzz(func(t *testing.T, in string) {
		for _, fd := range s.Check(in) {
			if fd.Offset < 0 || fd.Offset > len(in) {
				t.Fatalf("finding offset %d out of bounds for len %d: %+v", fd.Offset, len(in), fd)
			}
			if fd.Line < 1 || fd.Col < 1 {
				t.Fatalf("finding has non-positive line or column: %+v", fd)
			}
			// The reported match must be the exact text at the reported offset, which
			// catches any drift between where a rule matched and what it recorded.
			end := fd.Offset + len(fd.Match)
			if end > len(in) || in[fd.Offset:end] != fd.Match {
				t.Fatalf("match %q is not at offset %d in %q", fd.Match, fd.Offset, in)
			}
		}
		out, _ := s.Fix(in)
		if utf8.ValidString(in) && !utf8.ValidString(out) {
			t.Fatalf("Fix produced invalid UTF-8 from valid input %q -> %q", in, out)
		}
	})
}

// FuzzFixIdempotent checks that a second pass changes nothing, so the cleaner converges in
// one run and a caller can safely fix already-fixed text.
func FuzzFixIdempotent(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	s := buildFuzzSanitizer(f)

	f.Fuzz(func(t *testing.T, in string) {
		once, _ := s.Fix(in)
		twice, _ := s.Fix(once)
		if once != twice {
			t.Fatalf("Fix is not idempotent:\n in:    %q\n once:  %q\n twice: %q", in, once, twice)
		}
	})
}
