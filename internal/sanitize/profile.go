package sanitize

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
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
	// CollapseSpaces collapses runs of two or more spaces into one.
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
			"in summary, ":                "",
			"in conclusion, ":             "",
			"to recap, ":                  "",
			"overall, ":                   "",
			"it's worth noting that ":     "",
			"it is worth noting that ":    "",
			"giving it to you honestly, ": "",
			"to be honest, ":              "",
		},
		BlockWords: []string{
			"comprehensive", "robust", "seamless", "seamlessly",
			"elegant", "powerful", "cutting-edge", "delve",
			"blast radius", "substrate", "tapestry", "pivotal", "showcase",
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

	for _, from := range sortedKeys(p.CharReplace) {
		rules = append(rules, Rule{
			Name:    "char:" + from,
			re:      regexp.MustCompile(regexp.QuoteMeta(from)),
			repl:    p.CharReplace[from],
			rewrite: true,
		})
	}

	for _, phrase := range sortedKeys(p.PhraseReplace) {
		rules = append(rules, Rule{
			Name:    "phrase:" + strings.TrimSpace(phrase),
			re:      regexp.MustCompile("(?i)" + regexp.QuoteMeta(phrase)),
			repl:    p.PhraseReplace[phrase],
			rewrite: true,
		})
	}

	for _, w := range p.BlockWords {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(w) + `\b`)
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
			Name:     "semicolon",
			re:       regexp.MustCompile(`;\s+(\p{L})`),
			replFunc: splitSemicolon,
			keep:     semicolonJoinsClauses,
			rewrite:  true,
		})
	}

	if p.CollapseSpaces {
		rules = append(rules, Rule{
			Name:    "double-space",
			re:      regexp.MustCompile(`  +`),
			repl:    " ",
			rewrite: true,
		})
	}

	return rules, nil
}

// sortedKeys returns the keys of m in ascending order for deterministic rule ordering.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// splitSemicolon rewrites a "; x" match into ". X", ending the clause and capitalizing
// the next word.
func splitSemicolon(match string) string {
	r := []rune(match)
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
