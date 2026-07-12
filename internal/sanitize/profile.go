package sanitize

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Profile declares what a sanitizer bans and how it rewrites. It is the user-editable
// config that drives the rule engine.
type Profile struct {
	// CharReplace maps a literal substring to its replacement. Used for em-dashes,
	// smart quotes, ellipses, and similar character-level swaps.
	CharReplace map[string]string `json:"charReplace"`
	// PhraseReplace maps a case-insensitive phrase to its replacement. An empty
	// replacement deletes the phrase.
	PhraseReplace map[string]string `json:"phraseReplace"`
	// WordReplace maps a whole word to its replacement, matched case-insensitively with
	// the match's capitalization carried onto the replacement. Unlike a block word it
	// rewrites, so it is the safe way to swap one word for another.
	WordReplace map[string]string `json:"wordReplace"`
	// RegexReplace maps a regular expression to its replacement. The pattern is used as
	// written, so the caller controls anchoring, and a reference like $1 in the
	// replacement expands against the match.
	RegexReplace map[string]string `json:"regexReplace"`
	// BlockWords are words flagged wherever they appear. They are reported but never
	// rewritten, since a safe replacement depends on context.
	BlockWords []string `json:"blockWords"`
	// FlagPatterns maps a rule name to a regular expression that only flags its matches,
	// never rewrites them. It catches structural tells a word list cannot, like the
	// "not just X, but Y" cadence, where the fix depends on the whole sentence and is
	// left to the rewrite pass.
	FlagPatterns map[string]string `json:"flagPatterns"`
	// Allow lists words a rule must never flag or rewrite, matched case-insensitively
	// against the exact text a rule matched. It silences false positives.
	Allow []string `json:"allow"`
	// CollapseSpaces collapses runs of two or more spaces into one and removes spaces
	// and stray commas left before closing punctuation, like the debris an em-dash swap
	// or a dropped word leaves behind. Runs at the start of a line are indentation and
	// stay as they are.
	CollapseSpaces bool `json:"collapseSpaces"`
	// SplitSemicolons turns "; " into ". " and capitalizes the next word.
	SplitSemicolons bool `json:"splitSemicolons"`
	// ProtectQuotes leaves text inside double quotation marks unchanged, straight or smart,
	// so a quoted source is not reworded. Off by default, since cleaning your own draft
	// should reach inside your own quotes.
	ProtectQuotes bool `json:"protectQuotes"`
	// Tone holds optional notes on the voice to aim for. The rules pass ignores it.
	// The rewrite pass feeds it to the model so output sounds like you.
	Tone []string `json:"tone"`
	// Dialect enforces a spelling variant. "american" flags British spellings and
	// rewrites them, "british" does the reverse, and an empty value or "off" leaves
	// spelling alone.
	Dialect Dialect `json:"dialect"`
}

// DefaultProfile returns the built-in profile that targets common AI tells.
func DefaultProfile() Profile {
	return Profile{
		CharReplace: map[string]string{
			"\u00a0": " ",   // non-breaking space to a normal space
			"\u202f": " ",   // narrow non-breaking space to a normal space
			"\u200b": "",    // zero-width space, usually paste cruft
			"\u2060": "",    // word joiner, usually paste cruft
			"\ufeff": "",    // zero-width no-break space or a stray byte-order mark
			"—":      ", ",  // em-dash
			"–":      "-",   // en-dash
			"‘":      "'",   // left single quote
			"’":      "'",   // right single quote
			"“":      `"`,   // left double quote
			"”":      `"`,   // right double quote
			"…":      "...", // ellipsis
		},
		PhraseReplace: map[string]string{
			"additionally, ":                "",
			"consequently, ":                "",
			"furthermore, ":                 "",
			"importantly, ":                 "",
			"it is important to note that ": "",
			"it's important to note that ":  "",
			"more importantly, ":            "",
			"moreover, ":                    "",
			"notably, ":                     "",
			"rest assured, ":                "",
			"that being said, ":             "",
			"with that said, ":              "",
			"at its core, ":                 "",
			"at the end of the day, ":       "",
			"first and foremost, ":          "",
			"giving it to you honestly, ":   "",
			"in a nutshell, ":               "",
			"in conclusion, ":               "",
			"in essence, ":                  "",
			"in summary, ":                  "",
			"in today's digital age, ":      "",
			"in today's fast-paced world, ": "",
			"it goes without saying that ":  "",
			"it is worth noting that ":      "",
			"it's worth noting that ":       "",
			"last but not least, ":          "",
			"needless to say, ":             "",
			"overall, ":                     "",
			"simply put, ":                  "",
			"to be honest, ":                "",
			"to put it simply, ":            "",
			"to recap, ":                    "",
			"without further ado, ":         "",
		},
		BlockWords: []string{
			"best-in-class", "blast radius", "comprehensive", "cutting edge", "cutting-edge",
			"delve", "delved", "delves", "delving",
			"effortless", "effortlessly", "elegant", "empower", "empowering", "empowers",
			"ever-evolving", "facilitate", "facilitates", "facilitating", "fast-paced",
			"frictionless", "game-changer", "game-changing", "groundbreaking",
			"harness the power", "holistic", "in the realm of", "innovative", "invaluable",
			"leverage", "leveraged", "leverages", "leveraging",
			"meticulous", "meticulously", "myriad", "paradigm shift", "pivotal", "plethora",
			"powerful", "revolutionize", "revolutionized", "revolutionizes", "revolutionizing",
			"robust", "seamless", "seamlessly", "showcase", "showcased", "showcases",
			"showcasing", "state-of-the-art", "streamline", "streamlined", "streamlines",
			"streamlining", "substrate", "supercharge", "supercharged", "synergies", "synergy",
			"tapestry", "testament to", "top-notch", "transformative",
			"unleash", "unleashed", "unleashes", "unleashing",
			"unlock the full potential", "unlock the potential", "unparalleled",
			"utilize", "utilized", "utilizes", "utilizing", "world-class",
		},
		FlagPatterns: map[string]string{
			// "It's not just X, it's Y" and its "this is not X, it's Y" cousin, matched in
			// the contracted "it's" and the spelled-out "this is not" forms alike.
			"its-not-x-its-y": `(?i)\b(?:it|this|that)(?:'?s|\s+(?:is|was|are|were))(?:\s+not|n'?t)\b[^.!?\n]{1,40}[,;]\s*it'?s\b`,
			// "not just X but also Y" and "not only X but also Y".
			"not-just-but-also": `(?i)\bnot (just|only)\b[^.!?\n]{1,60}\bbut\b[^.!?\n]{0,25}\balso\b`,
			// Throat-clearing openers that promise a payoff.
			"heres-the-thing": `(?i)\bhere'?s the (thing|kicker|deal|catch|secret|problem)\b`,
			// The "let's dive in" invitation and its "let's take a closer look" cousins.
			"lets-dive-in":     `(?i)\blet'?s (dive|delve|jump) in(to)?\b`,
			"lets-take-a-look": `(?i)\blet'?s (?:take a (?:closer )?look|explore|unpack|break (?:it|this) down)\b`,
			// Chatbot reply openers and sign-offs.
			"assistant-opener": `(?im)^\s{0,3}(?:certainly|absolutely|great question|i'?d be happy to|happy to help|i hope this helps)\b`,
			// "That's where X comes in", the setup-and-reveal move.
			"thats-where-comes-in": `(?i)\bthat'?s where\b[^.!?\n]{1,30}\bcomes? in\b`,
		},
		Allow: []string{
			// Technical collocations where a flagged word is a term of art, protected so a
			// swap never turns "robust regression" into "solid regression".
			"robust regression", "robust standard errors", "robust estimator",
			"robust estimation", "robust statistics", "robust control",
			"optimal substructure", "optimal control", "optimal transport",
			"optimal policy", "optimal stopping",
			"comprehensive exam", "comprehensive examination",
		},
		CollapseSpaces:  true,
		SplitSemicolons: true,
	}
}

// Load reads a profile from JSON. Any field left unset keeps its zero value, so a
// partial profile is valid.
func Load(r io.Reader) (Profile, error) {
	var p Profile
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("profile decode: %w", err)
	}
	return p, nil
}

// LoadFile reads a profile from a JSON file at path.
func LoadFile(path string) (Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return Profile{}, fmt.Errorf("profile open: %w", err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}

// compile turns the profile into ordered rules. Character swaps run first, then
// phrases, then word flags, then whitespace and punctuation cleanup.
func (p Profile) compile() ([]Rule, error) {
	var rules []Rule

	for _, from := range slices.Sorted(maps.Keys(p.CharReplace)) {
		re, err := regexp.Compile(regexp.QuoteMeta(from))
		if err != nil {
			return nil, fmt.Errorf("%w: char swap %q: %w", ErrCompile, from, err)
		}
		rules = append(rules, Rule{
			Name:    "char:" + from,
			re:      re,
			repl:    p.CharReplace[from],
			rewrite: true,
		})
	}

	for _, phrase := range slices.Sorted(maps.Keys(p.PhraseReplace)) {
		r, err := phraseRule(phrase, p.PhraseReplace[phrase])
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	spelling, ok, err := spellingRule(p.Dialect)
	if err != nil {
		return nil, err
	}
	if ok {
		rules = append(rules, spelling)
	}

	swaps, drops := splitDrops(p.WordReplace)
	replace, ok, err := wordSwapRule("replace", lowerBoth(swaps))
	if err != nil {
		return nil, err
	}
	if ok {
		rules = append(rules, replace)
	}
	for _, w := range drops {
		r, err := deletionRule("drop:"+w, w)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	for _, pat := range slices.Sorted(maps.Keys(p.RegexReplace)) {
		r, err := regexRule(pat, p.RegexReplace[pat])
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	block, ok, err := blockWordRule(p.BlockWords)
	if err != nil {
		return nil, err
	}
	if ok {
		rules = append(rules, block)
	}

	for _, name := range slices.Sorted(maps.Keys(p.FlagPatterns)) {
		re, err := regexp.Compile(p.FlagPatterns[name])
		if err != nil {
			return nil, fmt.Errorf("%w: flag pattern %q: %w", ErrCompile, name, err)
		}
		rules = append(rules, Rule{
			Name:    "structural:" + name,
			re:      re,
			rewrite: false,
		})
	}

	if p.SplitSemicolons {
		rules = append(rules, Rule{
			// The pattern stays within one line, so a semicolon before a line break never
			// swallows the newline and reflows the paragraph.
			Name:     "semicolon",
			re:       regexp.MustCompile(`;[ \t]+(\p{L})`),
			replFunc: splitSemicolon,
			keep: func(text string, start, _ int) bool {
				return semicolonJoinsClauses(text, start)
			},
			rewrite: true,
		})
	}

	if p.CollapseSpaces {
		rules = append(rules, Rule{
			// Runs before space-before-punct so the sentence's separating space is still
			// there to keep, not eaten as space before a comma.
			Name:     "orphan-comma",
			re:       regexp.MustCompile(`,[ \t]*(\p{L})`),
			replFunc: stripOrphanComma,
			keep:     commaOpensSentence,
			rewrite:  true,
		})
		rules = append(rules, Rule{
			Name:     "space-before-punct",
			re:       regexp.MustCompile(`[ \t]+[,.!?;:]`),
			replFunc: trimLeadingSpace,
			keep:     spaceBeforePunctKeep,
			rewrite:  true,
		})
		rules = append(rules, Rule{
			Name:     "comma-before-stop",
			re:       regexp.MustCompile(`,+[.!?;:]`),
			replFunc: keepFinalByte,
			rewrite:  true,
		})
		rules = append(rules, Rule{
			Name:    "comma-run",
			re:      regexp.MustCompile(`,{2,}`),
			repl:    ",",
			rewrite: true,
		})
		rules = append(rules, Rule{
			Name:    "double-space",
			re:      regexp.MustCompile(`  +`),
			repl:    " ",
			keep:    collapsibleRun,
			rewrite: true,
		})
	}

	if allow := allowSet(p.Allow); allow != nil {
		for i := range rules {
			rules[i].allow = allow
		}
	}

	return rules, nil
}

// allowPhraseRe compiles the multi-word entries of an allow list into one alternation whose
// matches are protected from every rule, so a term of art like "robust regression" keeps its
// word even when the bare word is a tell. Single-word entries are left to the per-rule allow
// set. It returns nil when there are no multi-word entries.
func allowPhraseRe(allow []string) (*regexp.Regexp, error) {
	var parts []string
	for _, a := range allow {
		fields := strings.Fields(a)
		if len(fields) < 2 {
			continue
		}
		parts = append(parts, flexSpaces(regexp.QuoteMeta(strings.Join(fields, " "))))
	}
	if len(parts) == 0 {
		return nil, nil
	}
	slices.Sort(parts)
	re, err := regexp.Compile(`(?i)\b(?:` + strings.Join(parts, "|") + `)\b`)
	if err != nil {
		return nil, fmt.Errorf("%w: allow phrases: %w", ErrCompile, err)
	}
	return re, nil
}

// allowSet turns the allow list into a lower-cased lookup, or nil when it is empty.
func allowSet(words []string) map[string]bool {
	if len(words) == 0 {
		return nil
	}
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[strings.ToLower(w)] = true
	}
	return set
}

// splitDrops separates word entries that swap in a new word from those that cut a word.
// A blank target marks a drop, which deletionRule handles so the cut leaves no double
// space or orphaned capital. The drops come back sorted for a stable rule order.
func splitDrops(m map[string]string) (swaps map[string]string, drops []string) {
	swaps = make(map[string]string, len(m))
	for k, v := range m {
		if k == "" {
			continue
		}
		if v == "" {
			drops = append(drops, k)
			continue
		}
		swaps[k] = v
	}
	slices.Sort(drops)
	return swaps, drops
}

// lowerBoth returns m with every key and value lower-cased and empty keys dropped, the
// shape wordSwapRule expects. It returns nil for an empty map.
func lowerBoth(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k == "" {
			continue
		}
		out[strings.ToLower(k)] = strings.ToLower(v)
	}
	return out
}

// regexRule compiles a user regular expression into a rewriting rule. The pattern is used
// as written, so the caller controls anchoring and boundaries, and a reference like $1 in
// the replacement expands against the match. Zero-width matches are skipped so a pattern
// that can match nothing does not insert its replacement between every character.
func regexRule(pattern, repl string) (Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: regex %q: %w", ErrCompile, pattern, err)
	}
	return Rule{
		Name: "regex:" + pattern,
		re:   re,
		replFunc: func(text string, loc []int) string {
			span := text[loc[0]:loc[1]]
			sub := re.FindStringSubmatchIndex(span)
			if sub == nil {
				return span
			}
			return string(re.ExpandString(nil, repl, span, sub))
		},
		keep:    func(_ string, start, end int) bool { return end > start },
		rewrite: true,
	}, nil
}

// wsGap matches the whitespace between two words: spaces and tabs crossing at most one
// line break. It lets a phrase or a multi-word term match when a line wrap splits it,
// without ever reaching across a paragraph break.
const wsGap = `(?:[ \t]+(?:\n[ \t]*)?|\n[ \t]*)`

// flexSpaces widens each literal space in a quoted pattern into wsGap, so the words
// around it still match when a line wrap sits between them.
func flexSpaces(quoted string) string {
	return strings.ReplaceAll(quoted, " ", wsGap)
}

// blockWordRule compiles every block word into one flag-only rule. Folding them into a
// single alternation turns a full-text scan per word into one scan, and nameByMatch keeps
// each finding named for the word it caught. Longer words sort first so a longer term wins
// over a shorter one it contains at the same spot. It returns ok false for an empty list.
func blockWordRule(words []string) (Rule, bool, error) {
	alts := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			alts = append(alts, w)
		}
	}
	if len(alts) == 0 {
		return Rule{}, false, nil
	}
	slices.SortFunc(alts, func(a, b string) int {
		if d := len(b) - len(a); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	parts := make([]string, len(alts))
	for i, w := range alts {
		parts[i] = `\b` + flexSpaces(regexp.QuoteMeta(w)) + `\b`
	}
	re, err := regexp.Compile(`(?i)(?:` + strings.Join(parts, "|") + `)`)
	if err != nil {
		return Rule{}, false, fmt.Errorf("%w: block words: %w", ErrCompile, err)
	}
	return Rule{Name: "word", re: re, rewrite: false, nameByMatch: true}, true, nil
}

// endsWithWordChar reports whether s ends in an ASCII word character, the set the \b
// boundary recognizes, so a closing boundary is added only where it would hold.
func endsWithWordChar(s string) bool {
	if s == "" {
		return false
	}
	c := s[len(s)-1]
	return c == '_' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// phraseRule builds the rule for one phrase swap. A leading word boundary keeps the
// phrase from matching inside another word. A deletion is handled by deletionRule, which
// restores a sentence's opening capital. A non-empty replacement is a plain swap.
func phraseRule(phrase, repl string) (Rule, error) {
	if repl == "" {
		return deletionRule("phrase:"+strings.TrimSpace(phrase), phrase)
	}
	trimmed := strings.TrimRight(phrase, " ")
	core := `(?i)\b` + flexSpaces(regexp.QuoteMeta(trimmed))
	// A phrase ending in a word character gets a closing boundary so a key like "cat"
	// never fires inside "category". A phrase ending in punctuation, like the trailing
	// comma on "in summary,", is already bounded and takes no extra anchor.
	if endsWithWordChar(trimmed) {
		core += `\b`
	}
	re, err := regexp.Compile(core)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: phrase %q: %w", ErrCompile, phrase, err)
	}
	return Rule{Name: "phrase:" + strings.TrimSpace(phrase), re: re, replFunc: phraseSwap(repl), rewrite: true}, nil
}

// phraseSwap returns a replFunc that swaps a matched phrase for repl, carrying the
// match's capitalization onto it. A phrase opening a sentence keeps the opening capital,
// so "In order to ship" becomes "To ship" and not "to ship".
func phraseSwap(repl string) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		return matchCase(text[loc[0]:loc[1]], repl)
	}
}

// deletionRule builds a rule that cuts text and restores the sentence's opening capital.
// It eats the horizontal space after the match and captures the letter that follows, so
// the letter becomes a capital when the cut opened a sentence. It crosses a line break
// only when a word follows on the next line, so a cut never merges prose into a code
// fence or an indented block. Used for both stock-phrase openers and dropped words.
func deletionRule(name, text string) (Rule, error) {
	trimmed := strings.TrimRight(text, " ")
	core := `(?i)\b` + flexSpaces(regexp.QuoteMeta(trimmed))
	if endsWithWordChar(trimmed) {
		core += `\b`
	}
	re, err := regexp.Compile(core + `[ \t]*(?:\n[ \t]*(\p{L})|(\p{L})?)`)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: %s: %w", ErrCompile, name, err)
	}
	return Rule{Name: name, re: re, replFunc: deleteWithRecap(re), rewrite: true}, nil
}

// deleteWithRecap returns a replFunc that drops a phrase match, keeping the letter
// captured after it. The letter turns into a capital when the phrase opened a sentence,
// so deleting "In summary, it works." leaves "It works." and not "it works.". The letter
// may sit on the next line, which the match pulled up.
func deleteWithRecap(re *regexp.Regexp) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		start, end := recapLetter(re.FindStringSubmatchIndex(text[loc[0]:loc[1]]))
		if start < 0 {
			return ""
		}
		letter := text[loc[0]+start : loc[0]+end]
		if sentenceStart(text, loc[0]) {
			return strings.ToUpper(letter)
		}
		return letter
	}
}

// recapLetter returns the byte range of the recaptured letter within a submatch, taking
// whichever of the two capture groups matched, or -1, -1 when neither did.
func recapLetter(sub []int) (start, end int) {
	switch {
	case sub == nil:
		return -1, -1
	case sub[2] >= 0:
		return sub[2], sub[3]
	case len(sub) >= 6 && sub[4] >= 0:
		return sub[4], sub[5]
	default:
		return -1, -1
	}
}

// sentenceStart reports whether offset sits at the start of a sentence: at the start of
// the text, or after sentence-ending punctuation or a line break, with any spaces in
// between ignored.
func sentenceStart(text string, offset int) bool {
	i := offset - 1
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i < 0 {
		return true
	}
	switch text[i] {
	case '\n', '.', '!', '?':
		return true
	}
	return false
}

// trimLeadingSpace returns the match without its leading spaces and tabs, leaving just
// the punctuation.
func trimLeadingSpace(text string, loc []int) string {
	return strings.TrimLeft(text[loc[0]:loc[1]], " \t")
}

// keepFinalByte rewrites a match to its final byte. It drops a comma run pressed
// against closing punctuation, the debris left when a word between them is cut.
func keepFinalByte(text string, loc []int) string {
	return text[loc[1]-1 : loc[1]]
}

// commaOpensSentence reports whether the comma at start begins a sentence, which marks
// it as debris left when an opening word was cut. A comma anywhere else is ordinary.
func commaOpensSentence(text string, start, _ int) bool {
	return sentenceStart(text, start)
}

// stripOrphanComma drops a sentence-opening comma and the spaces after it, keeping the
// next letter as a capital, so cutting an opener like "Seamlessly," leaves a clean start.
func stripOrphanComma(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	return string(unicode.ToUpper(r[len(r)-1]))
}

// notLineStart reports whether the match at start has text before it on the same line.
// It keeps indentation, like a markdown code block leading into a dot, out of reach of
// the punctuation cleanup.
func notLineStart(text string, start, _ int) bool {
	return start > 0 && text[start-1] != '\n' && text[start-1] != '\r'
}

// spaceBeforePunctKeep reports whether a space-before-punctuation match is real cleanup and
// not Markdown structure. It keeps indentation out of reach like notLineStart, and skips the
// "!" that opens an inline image, where the space belongs before the image.
func spaceBeforePunctKeep(text string, start, end int) bool {
	if !notLineStart(text, start, end) {
		return false
	}
	return !(text[end-1] == '!' && end < len(text) && text[end] == '[')
}

// collapsibleRun reports whether a run of spaces should collapse. A run at the start of
// a line is indentation, a run that reaches the end of a line can be a markdown hard
// break, and a run on a table row is alignment padding, so all three stay.
func collapsibleRun(text string, start, end int) bool {
	if !notLineStart(text, start, end) || inTableRow(text, start) {
		return false
	}
	return end < len(text) && text[end] != '\n' && text[end] != '\r'
}

// inTableRow reports whether offset sits on a line whose first character is a pipe,
// which marks a markdown table row.
func inTableRow(text string, offset int) bool {
	i := offset
	for i > 0 && text[i-1] != '\n' {
		i--
	}
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return i < len(text) && text[i] == '|'
}

// splitSemicolon rewrites a "; x" match into ". X", ending the clause and capitalizing
// the next word. When the clause already ends in sentence punctuation, the semicolon is
// dropped without adding a second period, so "2.; the" does not become "2.. The".
func splitSemicolon(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	last := string(unicode.ToUpper(r[len(r)-1]))
	if loc[0] > 0 {
		switch text[loc[0]-1] {
		case '.', '!', '?':
			return " " + last
		}
	}
	return ". " + last
}

// semicolonConjunctions are the words that, right after a semicolon, mark it as a list
// separator rather than a clause join.
var semicolonConjunctions = []string{"and ", "or ", "but ", "nor ", "yet ", "so "}

// semicolonJoinsClauses reports whether the semicolon at offset semi joins two clauses,
// which is safe to split, rather than separating list items, which is not. It treats a
// semicolon as a list separator when its sentence holds more than one semicolon, or when
// a coordinating conjunction follows it, since both usually mean a deliberate list.
func semicolonJoinsClauses(text string, semi int) bool {
	start, end := sentenceBounds(text, semi)
	if strings.Count(text[start:end], ";") > 1 {
		return false
	}
	if inTableRow(text, semi) || inParens(text[start:semi]) {
		return false
	}
	rest := strings.ToLower(strings.TrimLeft(text[semi+1:end], " \t"))
	for _, conj := range semicolonConjunctions {
		if strings.HasPrefix(rest, conj) {
			return false
		}
	}
	return true
}

// inParens reports whether prefix, the text from the sentence start up to a semicolon,
// leaves a parenthesis open, which means the semicolon sits inside a parenthetical and is
// almost always a list separator rather than a clause join.
func inParens(prefix string) bool {
	return strings.Count(prefix, "(") > strings.Count(prefix, ")")
}

// sentenceBounds returns the byte range of the sentence around offset, bounded by
// sentence-ending punctuation or a newline.
func sentenceBounds(text string, offset int) (start, end int) {
	for i := offset - 1; i >= 0; i-- {
		if c := text[i]; c == '\n' || c == '.' || c == '!' || c == '?' {
			start = i + 1
			break
		}
	}
	end = len(text)
	for i := offset + 1; i < len(text); i++ {
		if c := text[i]; c == '\n' || c == '.' || c == '!' || c == '?' {
			end = i
			break
		}
	}
	return start, end
}
