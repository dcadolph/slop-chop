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
	// BlockWords are words flagged wherever they appear. They are reported but never
	// rewritten, since a safe replacement depends on context.
	BlockWords []string `json:"blockWords"`
	// CollapseSpaces collapses runs of two or more spaces into one and removes spaces
	// left before closing punctuation, like the debris an em-dash swap leaves behind.
	// Runs at the start of a line are indentation and stay as they are.
	CollapseSpaces bool `json:"collapseSpaces"`
	// SplitSemicolons turns "; " into ". " and capitalizes the next word.
	SplitSemicolons bool `json:"splitSemicolons"`
	// Tone holds optional notes on the voice to aim for. The rules pass ignores it.
	// The rewrite pass feeds it to the model so output sounds like you.
	Tone []string `json:"tone"`
}

// DefaultProfile returns the built-in profile that targets common AI tells.
func DefaultProfile() Profile {
	return Profile{
		CharReplace: map[string]string{
			"—": ", ",  // em-dash
			"–": "-",   // en-dash
			"‘": "'",   // left single quote
			"’": "'",   // right single quote
			"“": `"`,   // left double quote
			"”": `"`,   // right double quote
			"…": "...", // ellipsis
		},
		PhraseReplace: map[string]string{
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

	for _, w := range p.BlockWords {
		re, err := regexp.Compile(`(?i)\b` + flexSpaces(regexp.QuoteMeta(w)) + `\b`)
		if err != nil {
			return nil, fmt.Errorf("%w: block word %q: %w", ErrCompile, w, err)
		}
		rules = append(rules, Rule{
			Name:    "word:" + w,
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
			keep:     semicolonJoinsClauses,
			rewrite:  true,
		})
	}

	if p.CollapseSpaces {
		rules = append(rules, Rule{
			Name:     "space-before-punct",
			re:       regexp.MustCompile(`[ \t]+[,.!?;:]`),
			replFunc: trimLeadingSpace,
			keep:     notLineStart,
			rewrite:  true,
		})
		rules = append(rules, Rule{
			Name:    "double-space",
			re:      regexp.MustCompile(`  +`),
			repl:    " ",
			keep:    collapsibleRun,
			rewrite: true,
		})
	}

	return rules, nil
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

// phraseRule builds the rule for one phrase swap. A deletion also captures the letter
// after the phrase so the rewrite can restore the capital when the phrase opened its
// sentence.
func phraseRule(phrase, repl string) (Rule, error) {
	name := "phrase:" + strings.TrimSpace(phrase)
	pat := "(?i)" + flexSpaces(regexp.QuoteMeta(phrase))
	if repl != "" {
		re, err := regexp.Compile(pat)
		if err != nil {
			return Rule{}, fmt.Errorf("%w: phrase %q: %w", ErrCompile, phrase, err)
		}
		return Rule{Name: name, re: re, repl: repl, rewrite: true}, nil
	}
	re, err := regexp.Compile(pat + `(\p{L})?`)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: phrase %q: %w", ErrCompile, phrase, err)
	}
	return Rule{Name: name, re: re, replFunc: deleteWithRecap(re), rewrite: true}, nil
}

// deleteWithRecap returns a replFunc that drops a phrase match, keeping the letter
// captured after it. The letter turns into a capital when the phrase opened a sentence,
// so deleting "In summary, it works." leaves "It works." and not "it works.".
func deleteWithRecap(re *regexp.Regexp) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		sub := re.FindStringSubmatchIndex(text[loc[0]:loc[1]])
		if sub == nil || sub[2] < 0 {
			return ""
		}
		letter := text[loc[0]+sub[2] : loc[0]+sub[3]]
		if sentenceStart(text, loc[0]) {
			return strings.ToUpper(letter)
		}
		return letter
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

// notLineStart reports whether the match at start has text before it on the same line.
// It keeps indentation, like a markdown code block leading into a dot, out of reach of
// the punctuation cleanup.
func notLineStart(text string, start int) bool {
	return start > 0 && text[start-1] != '\n' && text[start-1] != '\r'
}

// collapsibleRun reports whether a run of spaces should collapse. A run at the start of
// a line is indentation, and a run on a markdown table row is alignment padding, so
// both stay.
func collapsibleRun(text string, start int) bool {
	return notLineStart(text, start) && !inTableRow(text, start)
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
// the next word.
func splitSemicolon(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	last := r[len(r)-1]
	return ". " + string(unicode.ToUpper(last))
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
	rest := strings.ToLower(strings.TrimLeft(text[semi+1:end], " \t"))
	for _, conj := range semicolonConjunctions {
		if strings.HasPrefix(rest, conj) {
			return false
		}
	}
	return true
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
