package sanitize

import "strings"

// Anaphora detection covers the repeated-opener drumbeat, "We needed retries. We needed
// observability. We needed ownership.", a signature cadence of model prose. A regular
// expression cannot express "the same two words again", so this walks sentences directly.

// anaphoraMaxWords is the longest sentence that still counts toward an anaphora run. The
// tell is a run of short, punchy sentences; long parallel sentences are the ordinary
// register of legal and technical writing.
const anaphoraMaxWords = 12

// anaphoraOrder places anaphora findings after every compiled rule in the dedupe order.
// The finding only flags, so the position never affects which rewrite is reported.
const anaphoraOrder = 1 << 30

// identicalRun reports whether every sentence in the run says the same thing word for
// word, ignoring case and closing punctuation. "It's not fair. It's not fair. It's not
// fair." is epizeuxis, deliberate emphasis and an old human device, while the anaphora
// tell is parallel openers with varying tails. Verbatim repetition is the author leaning
// on a line, so it is exempt.
func identicalRun(text string, spans []sentSpan) bool {
	first := sentenceWords(text[spans[0].start:spans[0].end])
	for _, sp := range spans[1:] {
		if sentenceWords(text[sp.start:sp.end]) != first {
			return false
		}
	}
	return true
}

// sentenceWords normalizes a sentence to its lower-cased words with sentence punctuation
// stripped, so "No way!" and "no way." compare equal.
func sentenceWords(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimRight(strings.TrimSpace(s), ".!?")), " "))
}

// sentSpan is one sentence located in the text, with the opener key anaphora runs are
// grouped by.
type sentSpan struct {
	// start and end are the byte range of the sentence.
	start, end int
	// key is the sentence's first two words, lower-cased, or empty when the sentence is
	// too short or too long to count toward a run.
	key string
	// adjacent reports that no paragraph break separates this sentence from the one
	// before it, so a run never reaches across paragraphs.
	adjacent bool
}

// anaphoraFindings reports every run of three or more consecutive short sentences in one
// paragraph that open with the same two words. The finding flags the whole run and is
// never rewritten: collapsing a drumbeat is a judgment call the author makes.
func anaphoraFindings(text string, protected [][2]int) []Finding {
	spans := sentenceSpans(text)
	// A markdown table is not prose. Its rows open alike by construction, which is
	// alignment rather than a drumbeat, so they neither seed a run nor extend one.
	for i := range spans {
		if inTableRow(text, spans[i].start) {
			spans[i].key = ""
		}
	}
	var out []Finding
	i := 0
	for i < len(spans) {
		j := i + 1
		for j < len(spans) && spans[j].adjacent && spans[j].key != "" && spans[j].key == spans[i].key {
			j++
		}
		if spans[i].key != "" && j-i >= 3 && !identicalRun(text, spans[i:j]) {
			start, end := spans[i].start, spans[j-1].end
			if !overlapsAny(protected, start, end) {
				out = append(out, Finding{
					Rule:   "structural:anaphora-run",
					Match:  text[start:end],
					Offset: start,
					order:  anaphoraOrder,
				})
			}
		}
		if j > i+1 {
			i = j
		} else {
			i++
		}
	}
	return out
}

// sentenceSpans splits text into located sentences. A sentence ends at sentence
// punctuation that is not an abbreviation, or at a paragraph break. A heading or a list
// item without closing punctuation runs to the next blank line, which keeps it from
// seeding a run of prose sentences.
func sentenceSpans(text string) []sentSpan {
	var spans []sentSpan
	i := 0
	adjacent := false
	for i < len(text) {
		newlines := 0
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			if text[i] == '\n' {
				newlines++
			}
			i++
		}
		if newlines > 1 {
			adjacent = false
		}
		if i >= len(text) {
			break
		}
		start := i
		end := -1
		for i < len(text) {
			c := text[i]
			if c == '.' || c == '!' || c == '?' {
				if c == '.' && abbreviationEnd(text, i) {
					i++
					continue
				}
				for i < len(text) && (text[i] == '.' || text[i] == '!' || text[i] == '?') {
					i++
				}
				end = i
				break
			}
			if c == '\n' && blankFrom(text, i+1) {
				end = i
				break
			}
			i++
		}
		if end < 0 {
			end = len(text)
		}
		spans = append(spans, sentSpan{start: start, end: end, key: anaphoraKey(text[start:end]), adjacent: adjacent})
		adjacent = true
	}
	return spans
}

// anaphoraKey returns the lower-cased first two words of sentence, or empty when the
// sentence is too short or too long to count toward an anaphora run.
func anaphoraKey(sentence string) string {
	fields := strings.Fields(sentence)
	if len(fields) < 2 || len(fields) > anaphoraMaxWords {
		return ""
	}
	second := strings.TrimRight(fields[1], ".!?,;:")
	if second == "" {
		return ""
	}
	return strings.ToLower(fields[0] + " " + second)
}
