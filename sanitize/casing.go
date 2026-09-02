package sanitize

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Casing judgment: telling a proper noun from a sentence-opening capital, so a rule never
// rewrites a name, and the voice's own casing preferences.

// notProperNoun reports whether a match should be acted on rather than skipped as a likely
// proper noun. A Title-case word mid-sentence, like a brand name, is skipped, while the same
// word at a sentence start is ordinary capitalization and still counts, unless the document
// itself proves the word is a name by using it Title-cased mid-sentence somewhere else. A
// lower-case or an all-caps match always counts.
func notProperNoun(text string, start, end int) bool {
	match := text[start:end]
	r := []rune(match)
	if len(r) == 0 || !unicode.IsUpper(r[0]) {
		return true
	}
	if match == strings.ToUpper(match) {
		return true
	}
	if !sentenceStart(text, start) {
		return false
	}
	return !titleCaseMidSentence(text, match, start)
}

// properNounWindow bounds how far around a match titleCaseMidSentence looks for proof.
// Any document a person actually writes fits inside it, so the search is effectively
// whole-document there, while a generated multi-megabyte input stays linear instead of
// rescanning everything for every sentence-start match.
const properNounWindow = 1 << 16

// titleCaseMidSentence reports whether match, kept in its exact casing, appears as a whole
// word at a non-sentence-start position near its occurrence at self. One such occurrence
// proves the word is a proper noun in this document, so its sentence-start occurrences are
// names too, like "Delve is a debugger" in a document that later says "attach Delve".
func titleCaseMidSentence(text, match string, self int) bool {
	lo := max(0, self-properNounWindow)
	hi := min(len(text), self+properNounWindow)
	for i := lo; ; {
		j := strings.Index(text[i:hi], match)
		if j < 0 {
			return false
		}
		pos := i + j
		i = pos + 1
		if pos == self {
			continue
		}
		if pos > 0 && isWordByte(text[pos-1]) {
			continue
		}
		if e := pos + len(match); e < len(text) && isWordByte(text[e]) {
			continue
		}
		if !sentenceStart(text, pos) {
			return true
		}
	}
}

// isWordByte reports whether c is an ASCII word character, the set the \b boundary
// recognizes.
func isWordByte(c byte) bool {
	return c == '_' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// splitCasing separates word swaps whose replacement differs from the key only by case,
// like github to GitHub or Internet to internet. Those are casing conventions, not
// vocabulary, and the ordinary swap machinery cannot serve them: matchCase re-imposes
// the match's capital on the replacement, and the proper-noun guard skips the exact
// miscasings the entry exists to fix.
func splitCasing(m map[string]string) (casing, rest map[string]string) {
	casing = make(map[string]string)
	rest = make(map[string]string, len(m))
	for from, to := range m {
		if to != "" && from != to && strings.EqualFold(from, to) {
			casing[from] = to
			continue
		}
		rest[from] = to
	}
	return casing, rest
}

// casingRule builds the rule for one casing convention: every case variant of the word
// rewrites to the exact target, and occurrences already cased right are left alone.
func casingRule(from, to string) (Rule, error) {
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(from) + `\b`)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: casing %q: %w", ErrCompile, from, err)
	}
	return Rule{
		Name: "case:" + strings.ToLower(from),
		re:   re,
		repl: to,
		keep: func(text string, start, end int) bool {
			m := text[start:end]
			if m == to {
				return false
			}
			// An all-caps use is deliberate styling, a heading or emphasis, so the
			// convention does not reach it.
			return len(m) <= 1 || m != strings.ToUpper(m)
		},
		rewrite: true,
	}, nil
}

// zwBetweenLetters reports whether a zero-width joiner or non-joiner sits between two
// ASCII letters, the smuggling position, so only there is it stripped.
func zwBetweenLetters(text string, start, end int) bool {
	if start == 0 || end >= len(text) {
		return false
	}
	prev, next := text[start-1], text[end]
	return isWordByte(prev) && prev < 128 && isWordByte(next) && next < 128
}

// britishOrdinals are the default ordinal swaps suppressed under a British dialect.
//
//nolint:gochecknoglobals // Immutable lookup.
var britishOrdinals = map[string]string{
	"firstly": "first", "secondly": "second", "thirdly": "third",
	"fourthly": "fourth", "lastly": "last",
}
