package sanitize

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Diffing a draft against the version its writer shipped is the one voice signal that
// cannot be contaminated by the model: the words come from the draft, but every change
// to them came from a person. Learning from a model's own output teaches a tool to
// imitate the tool. Learning from what a person changed about that output teaches it to
// imitate the person.

// EditPair is one change a writer made to a draft. Was is the draft's wording and Now is
// what replaced it, empty when the writer cut the words outright.
type EditPair struct {
	// Was is the wording the draft used.
	Was string `json:"was"`
	// Now is the wording the writer put in its place, or empty for a cut.
	Now string `json:"now"`
}

// maxPairWords bounds how many words either side of a pair may hold. A longer change is
// a restructured sentence rather than a word choice, and mining a style rule from it
// produces noise.
const maxPairWords = 8

// maxDiffCells caps the LCS table so a pair of large, unrelated documents cannot allocate
// without bound. Past the cap no pairs are reported, since a diff that large is not two
// versions of one text.
const maxDiffCells = 1 << 20

// EditPairs returns the changes between a draft and the text its writer shipped, keeping
// only those that read as word choice rather than fact. A pair whose two sides carry
// different anchors, the numbers, URLs, and acronyms a claim rests on, is dropped: that
// is a corrected fact, not a preferred phrasing. Pure insertions are dropped too, since
// added words are the writer's new content rather than a view about the draft's wording.
func EditPairs(draft, final string) []EditPair {
	return filterPairs(alignedPairs(draft, final), final)
}

// alignedPairs aligns the two texts and returns the raw change pairs. When the differing
// region is too large for one table, both texts are split at paragraph breaks and each
// paragraph pair is aligned on its own, so a long document with many small edits still
// yields its pairs instead of silently nothing.
func alignedPairs(draft, final string) []EditPair {
	was, now := fields(draft), fields(final)
	head, tail := commonEnds(was, now)
	was, now = was[head:len(was)-tail], now[head:len(now)-tail]
	if len(was)*len(now) <= maxDiffCells {
		return pairsFromScript(was, now)
	}
	dp, fp := paragraphs(draft), paragraphs(final)
	if len(dp) != len(fp) || len(dp) < 2 {
		return nil
	}
	var out []EditPair
	for i := range dp {
		out = append(out, alignedPairs(dp[i], fp[i])...)
	}
	return out
}

// paragraphs splits text at blank lines into its paragraphs, dropping empty ones.
func paragraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// filterPairs keeps the pairs that read as word choice. Matching trailing punctuation is
// trimmed from both sides first, so "robust." against "solid." proposes the words and a
// punctuation-only edit vanishes, and a cut whose words still appear in the shipped text
// is dropped as a move rather than a deletion preference.
func filterPairs(pairs []EditPair, final string) []EditPair {
	lowerFinal := " " + strings.ToLower(strings.Join(strings.Fields(final), " ")) + " "
	var out []EditPair
	for _, p := range pairs {
		p.Was = strings.TrimRight(p.Was, ".,;:!?")
		p.Now = strings.TrimRight(p.Now, ".,;:!?")
		if !keepPair(p) {
			continue
		}
		if p.Now == "" && strings.Contains(lowerFinal, " "+strings.ToLower(p.Was)+" ") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// CandidateKind says what one change tells you about the profile.
type CandidateKind string

const (
	// CandidateConfirms marks a change that removed something the rules already flag.
	// It adds no rule; it is evidence the existing one matches how you write.
	CandidateConfirms CandidateKind = "confirms"
	// CandidateNew marks a change the rules said nothing about, which is where a
	// profile grows: a word you always swap that nothing yet catches.
	CandidateNew CandidateKind = "new"
	// CandidateKeep marks a tell you read and shipped anyway. It is a false positive
	// report against the rules rather than a fact about your voice, and it is the most
	// valuable of the three, since it says the engine is wrong about this word.
	CandidateKeep CandidateKind = "keep"
)

// Candidate is one proposal drawn from a draft and the text its writer shipped. Nothing
// here is applied: a voice file the writer stopped trusting is worse than none, so these
// are meant to be read and accepted one at a time.
type Candidate struct {
	// Kind says which of the three readings this is.
	Kind CandidateKind `json:"kind"`
	// Pair is the change the writer made, empty for a keep, which is a thing they
	// deliberately did not change.
	Pair EditPair `json:"pair"`
	// Match is the text at issue: the wording that changed, or the tell that survived.
	Match string `json:"match"`
	// Rule names the rule involved for a confirms or a keep, and is empty for a new
	// candidate, which by definition no rule caught.
	Rule string `json:"rule,omitempty"`
}

// Candidates reads a draft against the text its writer shipped and reports what the pair
// says about the profile. Changes that removed a flagged tell confirm existing rules,
// changes the rules missed are where new ones come from, and tells the writer read and
// shipped are reported against the rules themselves. A document with no changes yields
// nothing, since a draft nobody edited is not evidence of anything.
func (s *Sanitizer) Candidates(draft, final string) []Candidate {
	pairs := EditPairs(draft, final)
	if len(pairs) == 0 {
		return nil
	}
	flagged := make(map[string]string)
	for _, f := range s.Check(draft) {
		if !TidyRule(f.Rule) {
			flagged[strings.ToLower(f.Match)] = f.Rule
		}
	}
	var out []Candidate
	for _, p := range pairs {
		c := Candidate{Kind: CandidateNew, Pair: p, Match: p.Was}
		if rule, ok := matchedRule(flagged, p.Was); ok {
			c.Kind, c.Rule = CandidateConfirms, rule
		} else if !viableRule(p) {
			// A new candidate exists to become a rule. A pair built on a stop word, a
			// markdown fragment, or punctuation would compile into a rule that is dead
			// or destructive, so it is not proposed at all.
			continue
		}
		out = append(out, c)
	}
	return append(out, s.keepCandidates(draft, final)...)
}

// pairStopWords are words too common to ever become a personal rule. A suggested swap
// keyed on one of these, the residue of a restructured clause aligning positionally,
// would rewrite half of every future document.
//
//nolint:gochecknoglobals // Immutable lookup.
var pairStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "am": true, "and": true, "or": true,
	"but": true, "to": true, "of": true, "in": true, "on": true, "at": true,
	"it": true, "its": true, "it's": true, "this": true, "that": true,
	"these": true, "those": true, "we": true, "you": true, "they": true,
	"he": true, "she": true, "i": true, "as": true, "for": true, "with": true,
	"by": true, "from": true, "not": true, "no": true, "so": true, "if": true,
	"then": true, "than": true, "there": true, "here": true, "will": true,
	"would": true, "can": true, "could": true, "has": true, "have": true,
	"had": true, "do": true, "does": true, "did": true,
}

// viableRule reports whether a pair can compile into a live, safe voice rule: no stop
// word on the changed side, and both sides made of plain words, since a side carrying
// markdown markers, code, or sentence punctuation produces a rule that never matches or
// matches what it should not.
func viableRule(p EditPair) bool {
	if pairStopWords[strings.ToLower(strings.TrimSpace(p.Was))] {
		return false
	}
	return plainWords(p.Was) && plainWords(p.Now)
}

// plainWords reports whether s is empty or words joined by single spaces, built from
// letters, digits, apostrophes, and hyphens only.
func plainWords(s string) bool {
	for _, f := range strings.Fields(s) {
		for _, r := range f {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' && r != '’' && r != '-' {
				return false
			}
		}
	}
	return true
}

// keepCandidates reports the tells that were in the draft and are still in the shipped
// text. The writer went through this document and left them, so each one is the rules
// disagreeing with a person who read the sentence.
func (s *Sanitizer) keepCandidates(draft, final string) []Candidate {
	lowerDraft, lowerFinal := strings.ToLower(draft), strings.ToLower(final)
	seen := make(map[string]bool)
	var out []Candidate
	for _, f := range s.Check(final) {
		if TidyRule(f.Rule) {
			continue
		}
		m := strings.ToLower(f.Match)
		if seen[m] || !strings.Contains(lowerDraft, m) || !strings.Contains(lowerFinal, m) {
			continue
		}
		seen[m] = true
		out = append(out, Candidate{Kind: CandidateKeep, Match: f.Match, Rule: f.Rule})
	}
	return out
}

// minReverseOverlap is the shortest changed text allowed to claim a rule by sitting
// inside that rule's match. Below it a common short word would attach itself to any
// finding that happens to contain those letters.
const minReverseOverlap = 4

// matchedRule reports the rule that flagged the text a change touched, so cutting a
// flagged word is read as agreeing with the rule that flags it. The overlap is checked
// both ways: a phrase deletion reports one letter past its phrase, so the rule's match
// can be slightly longer than the change that removed it.
func matchedRule(flagged map[string]string, was string) (string, bool) {
	lower := strings.ToLower(was)
	if rule, ok := flagged[lower]; ok {
		return rule, true
	}
	for match, rule := range flagged {
		if strings.Contains(lower, match) {
			return rule, true
		}
		if len(lower) >= minReverseOverlap && !pairStopWords[lower] && strings.Contains(match, lower) {
			return rule, true
		}
	}
	return "", false
}

// keepPair reports whether a raw pair survives the filters: it must change something,
// stay short enough to be a word choice, hold no anchor difference, and not be a pure
// insertion.
func keepPair(p EditPair) bool {
	if p.Was == "" {
		return false
	}
	if strings.EqualFold(p.Was, p.Now) {
		return false
	}
	if len(strings.Fields(p.Was)) > maxPairWords || len(strings.Fields(p.Now)) > maxPairWords {
		return false
	}
	// The word cap counts space-separated tokens, which an unsegmented script never
	// has: a wholly rewritten CJK sentence is one "word" on each side. A single token
	// longer than any real word marks text the word diff cannot serve.
	const maxTokenRunes = 24
	for _, side := range []string{p.Was, p.Now} {
		if !strings.Contains(side, " ") && utf8.RuneCountInString(side) > maxTokenRunes {
			return false
		}
	}
	return sameAnchors(p.Was, p.Now)
}

// sameAnchors reports whether two spans carry the same load-bearing tokens. It is the
// guard that separates a rewording from a fact correction: changing "robust" to "solid"
// keeps every anchor, while changing "a 5s delay" to "a 30s delay" does not.
func sameAnchors(was, now string) bool {
	a, b := Anchors(was), Anchors(now)
	if len(a) != len(b) {
		return false
	}
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// fields splits text into whitespace-separated tokens, the unit the diff aligns on.
func fields(text string) []string {
	return strings.Fields(text)
}

// commonEnds returns how many tokens the two token runs share at the start and at the
// end. Trimming them keeps the LCS table to the part that actually differs, which is what
// makes diffing two versions of one document cheap.
func commonEnds(a, b []string) (head, tail int) {
	for head < len(a) && head < len(b) && eqToken(a[head], b[head]) {
		head++
	}
	for tail < len(a)-head && tail < len(b)-head &&
		eqToken(a[len(a)-1-tail], b[len(b)-1-tail]) {
		tail++
	}
	return head, tail
}

// eqToken reports whether two tokens are the same word, ignoring case so a capital
// restored after a cut opener does not read as a change of its own.
func eqToken(a, b string) bool {
	return strings.EqualFold(a, b)
}

// pairsFromScript walks the longest common subsequence of the two token runs and groups
// each run of removals and insertions into one pair, so "we utilize robust tools" against
// "we use solid tools" yields two pairs rather than four scattered edits.
func pairsFromScript(was, now []string) []EditPair {
	lcs := lcsTable(was, now)
	var out []EditPair
	var dropped, added []string
	emit := func(was, now []string) {
		switch {
		case len(was) == 0 && len(now) == 0:
			return
		case len(was) == len(now):
			// The run swapped word for word, so each position is its own choice:
			// "utilize robust" against "use solid" is two swaps a rule can use, not one
			// two-word phrase that would never match anything again.
			for i := range was {
				out = append(out, EditPair{Was: was[i], Now: now[i]})
			}
		default:
			out = append(out, EditPair{Was: strings.Join(was, " "), Now: strings.Join(now, " ")})
		}
	}
	flush := func() {
		if len(dropped) == 0 && len(added) == 0 {
			return
		}
		// A run of changes that crosses a sentence end holds two unrelated edits, so it
		// is cut at that boundary first. Without this, ending one sentence and cutting
		// the opener of the next merge into a single pair that describes neither.
		wasSegs, nowSegs := splitAtSentenceEnds(dropped), splitAtSentenceEnds(added)
		n := min(len(wasSegs), len(nowSegs))
		cleanTail := true
		for i := 0; i < n; i++ {
			if i == n-1 && len(wasSegs[i]) != len(nowSegs[i]) {
				cleanTail = false
			}
			emit(wasSegs[i], nowSegs[i])
		}
		// Segments past the shorter side read as cuts or insertions only when the last
		// aligned segment was a clean word-for-word swap. When it was not, the rewrite
		// absorbed the trailing words, and reporting them as a cut fabricates an edit
		// of words the writer never singled out.
		if cleanTail {
			for i := n; i < len(wasSegs); i++ {
				emit(wasSegs[i], nil)
			}
			for i := n; i < len(nowSegs); i++ {
				emit(nil, nowSegs[i])
			}
		}
		dropped, added = nil, nil
	}
	i, j := 0, 0
	for i < len(was) && j < len(now) {
		switch {
		case eqToken(was[i], now[j]):
			flush()
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			dropped = append(dropped, was[i])
			i++
		default:
			added = append(added, now[j])
			j++
		}
	}
	dropped = append(dropped, was[i:]...)
	added = append(added, now[j:]...)
	flush()
	return out
}

// lcsTable builds the longest-common-subsequence lengths for the two token runs, where
// cell [i][j] is the best match available from was[i:] against now[j:].
func lcsTable(was, now []string) [][]int {
	table := make([][]int, len(was)+1)
	for i := range table {
		table[i] = make([]int, len(now)+1)
	}
	for i := len(was) - 1; i >= 0; i-- {
		for j := len(now) - 1; j >= 0; j-- {
			if eqToken(was[i], now[j]) {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			table[i][j] = max(table[i+1][j], table[i][j+1])
		}
	}
	return table
}

// splitAtSentenceEnds breaks a run of tokens after each one that closes a sentence, so a
// change spanning two sentences becomes one segment per sentence. A run holding no
// sentence end comes back as a single segment, and an empty run as no segments.
func splitAtSentenceEnds(tokens []string) [][]string {
	var out [][]string
	start := 0
	for i, t := range tokens {
		if !endsSentence(t) {
			continue
		}
		out = append(out, tokens[start:i+1])
		start = i + 1
	}
	if start < len(tokens) {
		out = append(out, tokens[start:])
	}
	return out
}

// endsSentence reports whether a token closes a sentence: it ends in sentence
// punctuation, allowing a closing quote or bracket after it, and is not an abbreviation
// by the same tests abbreviationEnd applies, so the two sentence detectors cannot
// disagree about "e.g." or "U.S." and fabricate a split.
func endsSentence(token string) bool {
	t := strings.TrimRight(token, `"')]}`)
	if t == "" {
		return false
	}
	switch t[len(t)-1] {
	case '!', '?':
		return true
	case '.':
		body := strings.TrimRight(t, ".")
		if body == "" || strings.Contains(body, ".") || len(body) == 1 {
			return false
		}
		return !abbreviations[strings.ToLower(body)]
	}
	return false
}
