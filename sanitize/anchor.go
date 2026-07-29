package sanitize

import "regexp"

// anchorPatterns are the load-bearing token kinds pulled from prose to compare an
// original against its rewrite. Order matters: each pattern masks its matches so a later
// one cannot re-capture the same text, so a URL's digits are never also counted as a
// bare number.
var anchorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s)]*[A-Za-z0-9/=#&%_~+-]`),
	regexp.MustCompile(`[\w.+-]+@[\w-]+(?:\.[\w-]+)+`),
	regexp.MustCompile(`[$£€]\d[\d.,]*[KkMmBbTt]?`),
	regexp.MustCompile(`\d[\d.,]*%`),
	regexp.MustCompile(`\d[\d.,:/-]*\d|\d`),
	regexp.MustCompile(`\b[A-Z][A-Z0-9]+\b`),
}

// Anchors returns the load-bearing tokens in the prose of text: URLs, emails, money,
// percentages, numbers, and all-caps acronyms. Code is masked first, since it is
// compared verbatim elsewhere, so its digits and identifiers never count as anchors. A
// rewrite pass diffs the anchors of its input and output to catch a changed fact.
func Anchors(text string) []string {
	b := []byte(maskCode(text))
	var out []string
	for _, re := range anchorPatterns {
		for _, loc := range re.FindAllIndex(b, -1) {
			out = append(out, string(b[loc[0]:loc[1]]))
			for i := loc[0]; i < loc[1]; i++ {
				b[i] = ' '
			}
		}
	}
	return out
}

// maskCode returns text with every code range blanked to spaces, keeping newlines and
// byte offsets intact, so a prose scan skips code without shifting positions.
func maskCode(text string) string {
	ranges := codeRanges(text)
	if len(ranges) == 0 {
		return text
	}
	b := []byte(text)
	for _, r := range ranges {
		for i := r[0]; i < r[1] && i < len(b); i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	return string(b)
}
