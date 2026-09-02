package sanitize

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Compiling a profile into rules: the ordered rule list the sanitizer runs, and the
// builders for each rule kind.

// compile turns the profile into ordered rules. Character swaps run first, then
// phrases, then word flags, then whitespace and punctuation cleanup.
func (p Profile) compile() ([]Rule, error) {
	var rules []Rule

	for _, from := range slices.Sorted(maps.Keys(p.CharReplace)) {
		re, err := regexp.Compile(regexp.QuoteMeta(from))
		if err != nil {
			return nil, fmt.Errorf("%w: char swap %q: %w", ErrCompile, from, err)
		}
		r := Rule{
			Name:    "char:" + from,
			re:      re,
			repl:    p.CharReplace[from],
			rewrite: true,
		}
		// A pair of em-dashes fencing a phrase that opens on a conjunction is emphasis,
		// not an aside, so commas leave "comprehensive, and robust, plan". Only the
		// default comma swap is made context-aware; a profile that maps the dash to
		// something else of its own gets what it asked for.
		if from == emDash && r.repl == emDashComma {
			r.replFunc = emDashSwap(r.repl)
			r.keep = emDashKeep
		}
		// The zero-width joiners are stripped only when smuggled between Latin
		// letters. Inside an emoji sequence or a complex script they are load-bearing.
		if from == "\u200c" || from == "\u200d" {
			r.keep = zwBetweenLetters
		}
		rules = append(rules, r)
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

	wordSwaps := p.WordReplace
	if p.Dialect == DialectBritish {
		// "Firstly, ... Secondly, ..." is ordinary British enumeration, so the default
		// ordinal swaps stand down under a British dialect. A user's own different
		// mapping for these words is kept.
		wordSwaps = maps.Clone(wordSwaps)
		for from, to := range britishOrdinals {
			if wordSwaps[from] == to {
				delete(wordSwaps, from)
			}
		}
	}
	swaps, drops := splitDrops(wordSwaps)
	casing, swaps := splitCasing(swaps)
	for _, from := range slices.Sorted(maps.Keys(casing)) {
		r, err := casingRule(from, casing[from])
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	replace, ok, err := wordSwapRule("replace", lowerKeys(swaps))
	if err != nil {
		return nil, err
	}
	if ok {
		replace.keep = notProperNoun
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
		block.keep = notProperNoun
		rules = append(rules, block)
	}

	always, ok, err := blockWordRule(p.BlockAlways)
	if err != nil {
		return nil, err
	}
	if ok {
		// No proper-noun keep: these are the user's own banned words, and "Synergy"
		// capitalized is still synergy.
		rules = append(rules, always)
	}

	for _, name := range slices.Sorted(maps.Keys(p.FlagPatterns)) {
		re, err := regexp.Compile(flexPatternSpaces(p.FlagPatterns[name]))
		if err != nil {
			return nil, fmt.Errorf("%w: flag pattern %q: %w", ErrCompile, name, err)
		}
		rules = append(rules, Rule{
			Name:    "structural:" + name,
			re:      re,
			keep:    flagPatternKeeps[name],
			rewrite: false,
			unwrap:  true,
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
			tidy:    true,
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
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:     "space-before-punct",
			re:       regexp.MustCompile(`[ \t]+[,.!?;:]`),
			replFunc: trimLeadingSpace,
			keep:     spaceBeforePunctKeep,
			rewrite:  true,
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:     "comma-before-stop",
			re:       regexp.MustCompile(`,+[.!?;:]`),
			replFunc: keepFinalByte,
			rewrite:  true,
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:    "comma-run",
			re:      regexp.MustCompile(`,{2,}`),
			repl:    ",",
			rewrite: true,
			tidy:    true,
		})
		rules = append(rules, Rule{
			Name:    "double-space",
			re:      regexp.MustCompile(`  +`),
			repl:    " ",
			keep:    collapsibleRun,
			rewrite: true,
			tidy:    true,
		})
	}

	if p.FixArticles {
		rules = append(rules, Rule{Name: "article", re: articleRe, replFunc: fixArticle, keep: articleNeedsFix, rewrite: true, tidy: true})
	}

	if allow := allowSet(p.Allow); allow != nil {
		for i := range rules {
			rules[i].allow = allow
		}
	}

	return rules, nil
}

// allowPhraseRe compiles the multi-word and punctuated entries of an allow list into one
// alternation whose matches are protected from every rule, so a term of art like "robust
// regression" keeps its word even when the bare word is a tell. Bare single words are
// left to the per-rule allow set. An entry carrying punctuation, like "in summary,"
// copied exactly as the tool displays the phrase, is protected here too, with each
// boundary applied only where a word character can hold it, since a \b after a comma can
// never match and would make the keep a silent no-op.
func allowPhraseRe(allow []string) (*regexp.Regexp, error) {
	var parts []string
	for _, a := range allow {
		fields := strings.Fields(a)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 2 && plainWords(a) {
			continue
		}
		joined := strings.Join(fields, " ")
		part := flexSpaces(regexp.QuoteMeta(joined))
		if isWordByte(joined[0]) {
			part = `\b` + part
		}
		if endsWithWordChar(joined) {
			part += `\b`
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	slices.Sort(parts)
	re, err := regexp.Compile(`(?i)(?:` + strings.Join(parts, "|") + `)`)
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

// lowerKeys returns m with every key lower-cased and empty keys dropped, the shape
// wordSwapRule expects for case-insensitive matching. Values are left as written, so a
// replacement's intended capitalization, like "GitHub" or "iPhone", survives instead of
// being flattened to lower case. It returns nil for an empty map.
func lowerKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k == "" {
			continue
		}
		out[strings.ToLower(k)] = v
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
			// loc already holds the submatch indices from matching the full text, so the
			// replacement expands against the original with its surrounding context intact.
			// Re-running the pattern on the isolated span would drop the preceding character
			// a boundary anchor like \b or \B depends on, silently voiding the swap.
			return string(re.ExpandString(nil, repl, text, loc))
		},
		keep:    func(_ string, start, end int) bool { return end > start },
		rewrite: true,
	}, nil
}

// wsGap matches the whitespace between two words: spaces and tabs crossing at most one
// line break, LF or CRLF. It lets a phrase or a multi-word term match when a line wrap
// splits it, without ever reaching across a paragraph break.
const wsGap = `(?:[ \t]+(?:\r?\n[ \t]*)?|\r?\n[ \t]*)`

// flagPatternKeeps attaches judgment to the built-in flag patterns whose regex alone
// over-matches. A pattern absent here keeps every match.
//
//nolint:gochecknoglobals // Immutable lookup.
var flagPatternKeeps = map[string]func(text string, start, end int) bool{
	"its-not-x-its-y": notXRepeated,
	"triad-fragment":  triadVaries,
}

// triadVaries keeps a triad-fragment match only when its three items differ. "Location,
// location, location." repeats one word on purpose, epizeuxis rather than the model's
// fast-cheap-reliable fragment, so an identical triad is exempt.
func triadVaries(text string, start, end int) bool {
	items := strings.Split(strings.Trim(text[start:end], " \t\n.!?"), ",")
	if len(items) < 2 {
		return true
	}
	first := strings.ToLower(strings.TrimSpace(items[0]))
	// The match may open with the previous sentence's closing punctuation, which the
	// trim above removed along with any lead-in; only the word remains.
	if i := strings.LastIndexAny(first, " \t"); i >= 0 {
		first = first[i+1:]
	}
	for _, item := range items[1:] {
		if strings.ToLower(strings.TrimSpace(item)) != first {
			return true
		}
	}
	return false
}

// notXNegated pulls the negated phrase out of an its-not-x-its-y match: the text between
// the negation and the clause separator.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var notXNegated = regexp.MustCompile(`(?i)(?:\bnot|n'?t)\b\s*([^.!?\n]{1,40}?)\s*(?:[,;.]|—|–)`)

// notXRepeated keeps an its-not-x-its-y match only when the second clause says something
// new. "It's not fair. It's not fair." repeats the line for emphasis, epizeuxis rather
// than the model's not-X-but-Y contrast, so a Y that opens by negating the same X again
// is exempt.
func notXRepeated(text string, start, end int) bool {
	m := notXNegated.FindStringSubmatch(text[start:end])
	if m == nil {
		return true
	}
	x := strings.ToLower(strings.TrimSpace(m[1]))
	tail := strings.TrimSpace(text[end:])
	lower := strings.ToLower(tail)
	for _, neg := range []string{"not ", "n't "} {
		if rest, ok := strings.CutPrefix(lower, neg); ok {
			return !strings.HasPrefix(rest, x)
		}
	}
	return true
}

// flexSpaces widens each literal space in a quoted pattern into wsGap, so the words
// around it still match when a line wrap sits between them.
func flexSpaces(quoted string) string {
	return strings.ReplaceAll(quoted, " ", wsGap)
}

// flexPatternSpaces widens each literal space in a hand-written pattern into a run of one
// or more spaces and tabs, so a flag pattern still matches where a wrap left extra
// indentation between two words. Unlike flexSpaces the input is a real pattern, not quoted
// text, so a space inside a character class is left alone: widening the one in "[ \t]+"
// would break the class it belongs to. A space after a backslash is an escaped literal and
// is widened like any other.
func flexPatternSpaces(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern))
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			if pattern[i+1] == ' ' && !inClass {
				b.WriteString(`[ \t]+`)
			} else {
				b.WriteByte(c)
				b.WriteByte(pattern[i+1])
			}
			i++
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		case c == ' ' && !inClass:
			b.WriteString(`[ \t]+`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// blockWordRule folds every block word into one flag-only rule backed by the word-start
// scanner, so hundreds of terms cost one linear pass instead of one giant alternation.
// nameByMatch keeps each finding named for the word it caught, and the scanner gives a
// longer term the win over a shorter one it contains at the same spot. It returns ok
// false for an empty list.
func blockWordRule(words []string) (Rule, bool, error) {
	for _, w := range words {
		// The scanner matches bytes and cannot trip on a malformed term the way a regex
		// compile did, so a garbage profile is rejected here instead of failing silently.
		if !utf8.ValidString(w) {
			return Rule{}, false, fmt.Errorf("%w: block word %q: invalid UTF-8", ErrCompile, w)
		}
	}
	idx := newBlockIndex(words)
	if len(idx) == 0 {
		return Rule{}, false, nil
	}
	return Rule{Name: "word", matchFunc: idx.scan, rewrite: false, nameByMatch: true}, true, nil
}

// endsWithWordChar reports whether s ends in an ASCII word character, so a closing \b
// boundary is added only where it would hold.
func endsWithWordChar(s string) bool {
	return s != "" && isWordByte(s[len(s)-1])
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
