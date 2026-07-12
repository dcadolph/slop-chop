package sanitize

import (
	"regexp"
	"strings"
)

// Structural protection covers the parts of Markdown and front matter that are syntax or
// machine-read data rather than prose: link and image destinations, autolinks, reference
// definitions, bare URLs, and a leading front-matter block. Rules only rewrite prose, so a
// link target or a YAML value is never touched, while the visible link text stays editable.

//nolint:gochecknoglobals // Compiled once, never modified.
var (
	// inlineDestRe matches an inline link or image destination, the "(...)" after "](", so
	// the URL and any title are protected while the bracketed text stays editable.
	inlineDestRe = regexp.MustCompile(`\]\(([^)\n]*)\)`)
	// autolinkRe matches a CommonMark autolink: a URI with a scheme, or a bare email, in
	// angle brackets. The scheme or the "@" keeps it from matching an ordinary HTML tag.
	autolinkRe = regexp.MustCompile(`<(?:[a-zA-Z][a-zA-Z0-9+.\-]*:[^>\s]*|[^>\s@]+@[^>\s]+)>`)
	// refDefRe matches a reference link definition line, "[label]: dest title", so both the
	// label and the destination are protected.
	refDefRe = regexp.MustCompile(`(?m)^ {0,3}\[[^\]\n]+\]:.*$`)
	// bareURLRe matches a bare http or https URL in prose. It ends on a URL character, so
	// trailing sentence punctuation is left outside the protected span.
	bareURLRe = regexp.MustCompile(`https?://[^\s<>()\[\]]*[A-Za-z0-9/=#&%_~+-]`)
)

// structuralRanges returns the byte ranges of Markdown structure and front matter that must
// not be rewritten. The ranges are unsorted and may overlap each other and code, which
// skipRanges merges before use.
func structuralRanges(text string) [][2]int {
	ranges := frontMatterRange(text)
	for _, loc := range inlineDestRe.FindAllStringSubmatchIndex(text, -1) {
		ranges = append(ranges, [2]int{loc[2], loc[3]})
	}
	for _, re := range []*regexp.Regexp{autolinkRe, refDefRe, bareURLRe} {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			ranges = append(ranges, [2]int{loc[0], loc[1]})
		}
	}
	return ranges
}

// frontMatterRange returns the range of a leading YAML or TOML front-matter block, or nil
// when the text has none. The block opens on a first line of only "---" or "+++" and closes
// on the next line that is only that same marker. An unclosed opener protects nothing, so a
// lone "---" thematic break does not swallow the document.
func frontMatterRange(text string) [][2]int {
	var marker string
	switch {
	case strings.HasPrefix(text, "---\n"), strings.HasPrefix(text, "---\r\n"):
		marker = "---"
	case strings.HasPrefix(text, "+++\n"), strings.HasPrefix(text, "+++\r\n"):
		marker = "+++"
	default:
		return nil
	}
	for _, lr := range lineRanges(text)[1:] {
		if strings.TrimRight(text[lr[0]:lr[1]], "\r") == marker {
			return [][2]int{{0, lr[1]}}
		}
	}
	return nil
}
