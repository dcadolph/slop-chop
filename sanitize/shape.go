package sanitize

import "strings"

// Shape detection covers the outline-driven register: prose whose paragraphs share one
// template. No word list catches it, because its vocabulary is clean; the tell is the
// sameness itself. Two shapes are read: paragraphs that all hold the same number of
// sentences, and a sentence stem repeated across paragraphs.

// shapeMinParagraphs is the fewest body paragraphs that can establish a shape. Below it
// sameness is coincidence.
const shapeMinParagraphs = 5

// shapeUniformShare is the share of body paragraphs that must share one sentence count
// before the sameness reads as a template.
const shapeUniformShare = 0.8

// stemWords is how many opening words make a sentence stem, and stemMinRepeats is how
// many paragraphs must open a sentence with the same stem before it reads as a template.
const (
	stemWords      = 4
	stemMinRepeats = 3
)

// shapeFindings reports the template shapes in text: a run of same-length paragraphs and
// repeated sentence stems. Both only flag, since reshaping a document is nobody's rule.
func shapeFindings(text string, protected [][2]int) []Finding {
	paras := paragraphSpans(text)
	var out []Finding
	if f, ok := uniformParagraphs(text, paras); ok && !overlapsAny(protected, f.Offset, f.Offset+len(f.Match)) {
		out = append(out, f)
	}
	out = append(out, templateStems(text, paras, protected)...)
	return out
}

// paraSpan is one paragraph located in the text with its sentence count and word count.
type paraSpan struct {
	// start and end are the byte range of the paragraph.
	start, end int
	// sentences is how many sentences the paragraph holds.
	sentences int
	// words is the paragraph's word count.
	words int
}

// paragraphSpans locates every paragraph, a run of non-blank lines, with its sentence
// and word counts.
func paragraphSpans(text string) []paraSpan {
	var out []paraSpan
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		body := text[start:end]
		n := 0
		for _, sp := range sentenceSpans(body) {
			if sp.end > sp.start {
				n++
			}
		}
		out = append(out, paraSpan{start: start, end: end, sentences: n, words: len(strings.Fields(body))})
		start = -1
	}
	pos := 0
	for pos <= len(text) {
		lineEnd := len(text)
		if i := strings.IndexByte(text[pos:], '\n'); i >= 0 {
			lineEnd = pos + i
		}
		if strings.TrimSpace(text[pos:lineEnd]) == "" {
			flush(pos)
		} else if start < 0 {
			start = pos
		}
		if lineEnd == len(text) {
			break
		}
		pos = lineEnd + 1
	}
	flush(len(text))
	return out
}

// uniformParagraphs reports one finding when enough body paragraphs share one exact
// sentence count. Body means prose-sized: at least two sentences and fifteen words, so
// headings and list stubs stay out of the sample.
func uniformParagraphs(text string, paras []paraSpan) (Finding, bool) {
	var body []paraSpan
	for _, p := range paras {
		if p.sentences >= 2 && p.words >= 15 {
			body = append(body, p)
		}
	}
	if len(body) < shapeMinParagraphs {
		return Finding{}, false
	}
	counts := map[int]int{}
	for _, p := range body {
		counts[p.sentences]++
	}
	for n, c := range counts {
		if float64(c) < shapeUniformShare*float64(len(body)) {
			continue
		}
		for _, p := range body {
			if p.sentences == n {
				return Finding{
					Rule:   "structural:uniform-paragraphs",
					Match:  text[p.start:p.end],
					Offset: p.start,
					order:  anaphoraOrder,
				}, true
			}
		}
	}
	return Finding{}, false
}

// templateStems reports a finding for each opening stem, the first few words of a
// sentence, that recurs across enough distinct paragraphs. "In this section, we" five
// times is an outline talking, not a writer.
func templateStems(text string, paras []paraSpan, protected [][2]int) []Finding {
	type site struct{ start, end, para int }
	stems := map[string][]site{}
	for pi, p := range paras {
		for _, sp := range sentenceSpans(text[p.start:p.end]) {
			words := strings.Fields(text[p.start+sp.start : p.start+sp.end])
			if len(words) < stemWords+1 {
				continue
			}
			key := strings.ToLower(strings.Join(words[:stemWords], " "))
			if !plainWords(strings.Join(words[:stemWords], " ")) {
				continue
			}
			stems[key] = append(stems[key], site{p.start + sp.start, p.start + sp.end, pi})
		}
	}
	var out []Finding
	for _, sites := range stems {
		paraSet := map[int]bool{}
		for _, s := range sites {
			paraSet[s.para] = true
		}
		if len(paraSet) < stemMinRepeats {
			continue
		}
		first := sites[0]
		end := first.start + stemEnd(text[first.start:first.end], stemWords)
		if end <= first.start || overlapsAny(protected, first.start, end) {
			continue
		}
		out = append(out, Finding{
			Rule:   "structural:template-stem",
			Match:  text[first.start:end],
			Offset: first.start,
			order:  anaphoraOrder,
		})
	}
	return out
}

// stemEnd returns the byte offset just past the nth whitespace-separated word of s, or 0
// when s holds fewer than n words.
func stemEnd(s string, n int) int {
	inWord := false
	count := 0
	for i := 0; i < len(s); i++ {
		space := s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r'
		switch {
		case !space && !inWord:
			inWord = true
			count++
		case space && inWord:
			inWord = false
			if count == n {
				return i
			}
		}
	}
	if inWord && count == n {
		return len(s)
	}
	return 0
}
