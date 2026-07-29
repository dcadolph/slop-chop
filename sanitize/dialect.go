package sanitize

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Dialect selects which spelling variant a sanitizer enforces.
type Dialect string

const (
	// DialectOff disables the spelling pass. It is the zero value.
	DialectOff Dialect = ""
	// DialectAmerican flags British spellings and rewrites them to American.
	DialectAmerican Dialect = "american"
	// DialectBritish flags American spellings and rewrites them to British.
	DialectBritish Dialect = "british"
)

// dialectData is the embedded spelling word list.
//
//go:embed dialect.json
var dialectData []byte

// spellingPair is one British and American spelling of the same word. OneWay marks a pair
// whose American spelling doubles as an unrelated word, so the pair rewrites only toward
// American and is skipped when targeting British, keeping "check" from becoming "cheque".
type spellingPair struct {
	// British is the British spelling.
	British string `json:"british"`
	// American is the American spelling.
	American string `json:"american"`
	// OneWay reports that the pair is safe only when rewriting toward American.
	OneWay bool `json:"oneway"`
}

// spellingPairs is the embedded word list, parsed once at startup.
//
//nolint:gochecknoglobals // Immutable table built from an embedded file.
var spellingPairs = loadSpellingPairs()

// loadSpellingPairs parses the embedded word list. It panics on a malformed list, since
// the list ships in the binary and a parse failure is a build error, not a runtime one.
func loadSpellingPairs() []spellingPair {
	var pairs []spellingPair
	if err := json.Unmarshal(dialectData, &pairs); err != nil {
		panic(fmt.Sprintf("sanitize: dialect word list: %v", err))
	}
	return pairs
}

// spellingMap returns the source-to-target spellings for a dialect, both lower case. For
// American it maps each British spelling to its American form, and for British it maps the
// American spelling back to British, skipping one-way pairs whose reverse is ambiguous.
func spellingMap(d Dialect) map[string]string {
	m := make(map[string]string, len(spellingPairs))
	for _, p := range spellingPairs {
		switch d {
		case DialectAmerican:
			m[strings.ToLower(p.British)] = strings.ToLower(p.American)
		case DialectBritish:
			if p.OneWay {
				continue
			}
			m[strings.ToLower(p.American)] = strings.ToLower(p.British)
		case DialectOff:
		}
	}
	return m
}

// spellingRule builds the rule that flags and rewrites spellings foreign to the target
// dialect. It returns ok false when the dialect enforces no spelling, and an error when
// the dialect is unknown. The dialect name is matched case-insensitively, and "off" or
// "none" disable the pass alongside the empty value.
func spellingRule(d Dialect) (Rule, bool, error) {
	switch strings.ToLower(string(d)) {
	case string(DialectOff), "off", "none":
		return Rule{}, false, nil
	case string(DialectAmerican):
		d = DialectAmerican
	case string(DialectBritish):
		d = DialectBritish
	default:
		return Rule{}, false, fmt.Errorf("%w: %q", ErrDialect, string(d))
	}
	return wordSwapRule("spelling", spellingMap(d))
}

// wordSwapRule builds one rule that rewrites any whole word found in from to its mapped
// value, carrying the match's case onto the replacement. Keys are lower case; a value
// keeps whatever capitalization it was written with, so "GitHub" is not flattened. It
// returns ok false when from is empty, so the caller can skip it.
func wordSwapRule(name string, from map[string]string) (Rule, bool, error) {
	if len(from) == 0 {
		return Rule{}, false, nil
	}
	words := slices.Sorted(maps.Keys(from))
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = regexp.QuoteMeta(w)
	}
	re, err := regexp.Compile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`)
	if err != nil {
		return Rule{}, false, fmt.Errorf("%w: %s: %w", ErrCompile, name, err)
	}
	return Rule{Name: name, re: re, replFunc: wordSwapReplace(from), rewrite: true}, true, nil
}

// wordSwapReplace returns a replFunc that swaps a matched word for its target, carrying
// over the match's capitalization. An unmapped match is left as it is, though the rule
// only matches words the map holds.
func wordSwapReplace(from map[string]string) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		match := text[loc[0]:loc[1]]
		to, ok := from[strings.ToLower(match)]
		if !ok {
			return match
		}
		return matchCase(match, to)
	}
}

// matchCase returns repl recased to mirror match: an all-caps match yields an all-caps
// replacement, a leading capital yields a leading capital, and a lower-case or mixed match
// returns repl as written, so a value that carries its own casing like "GitHub" survives.
func matchCase(match, repl string) string {
	if match == "" || repl == "" {
		return repl
	}
	if match == strings.ToUpper(match) && match != strings.ToLower(match) {
		return strings.ToUpper(repl)
	}
	if r := []rune(match); unicode.IsUpper(r[0]) {
		out := []rune(repl)
		out[0] = unicode.ToUpper(out[0])
		return string(out)
	}
	return repl
}
