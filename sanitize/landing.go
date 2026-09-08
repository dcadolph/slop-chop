package sanitize

import "strings"

// Landing detection covers the copula-abstraction landing: a concrete or demonstrative
// subject equated to an abstraction with a bare "is", delivered as a short payoff at the
// close of a sentence. "That number is the rest of the day." carries no banned word, no
// em-dash, and no stock opener, so every lexical rule walks past it; the tell is the
// shape. A regular expression cannot express "the subject is concrete and the complement
// is abstract", so this walks sentences and reads the closing clause directly.

// The clause must be short enough to read as a payoff and long enough to hold a subject,
// a copula, and a complement. A long closing clause is ordinary exposition.
const (
	landingMinWords = 4
	landingMaxWords = 10
)

// landingMaxSubjectWords is the longest subject that still reads as a landing. The tell
// puts a bare noun phrase against the copula; a subject carrying its own modifiers is a
// sentence doing work.
const landingMaxSubjectWords = 3

// landingMaxComplementWords is the longest complement that still reads as a payoff: a
// determiner, one modifier, and the abstraction, plus an "of" tail of up to three words.
const landingMaxComplementWords = 6

//nolint:gochecknoglobals // Immutable lookups.
var (
	// landingCopulas are the bare copulas a landing turns on. A modal or a perfect
	// ("has been", "would be") is a sentence reasoning, not a sentence landing.
	landingCopulas = map[string]bool{"is": true, "are": true, "was": true, "were": true}

	// landingConjunctions open the closing clause after a comma, so the clause is read
	// from its subject rather than from the join.
	landingConjunctions = map[string]bool{
		"and": true, "but": true, "or": true, "so": true, "yet": true, "then": true,
	}

	// landingSubjectDeterminers may open a subject noun phrase. The demonstratives are in
	// here because "that number", pointing back at something already said, is the
	// strongest form of the concrete subject the tell equates away. On their own they are
	// pronouns instead, which landingPronouns rules out.
	landingSubjectDeterminers = map[string]bool{
		"the": true, "a": true, "an": true, "our": true, "your": true, "its": true,
		"their": true, "his": true, "her": true, "my": true, "every": true, "each": true,
		"that": true, "this": true, "these": true, "those": true,
	}

	// landingComplementDeterminers may open the complement. Only definite determiners
	// qualify: "a constraint" describes, while "the constraint" declares, and the
	// declaration is the landing.
	landingComplementDeterminers = map[string]bool{
		"the": true, "our": true, "your": true, "its": true, "their": true,
		"his": true, "her": true, "my": true,
	}

	// landingModifiers may sit between the complement's determiner and its abstraction,
	// the intensifiers a landing reaches for.
	landingModifiers = map[string]bool{
		"whole": true, "real": true, "only": true, "entire": true, "actual": true,
	}

	// landingPronouns cannot stand as the bare subject of a landing. "It is the point"
	// is ordinary reference, and a bare demonstrative is the "That's the story" idiom
	// people have always written.
	landingPronouns = map[string]bool{
		"it": true, "he": true, "she": true, "they": true, "we": true, "you": true,
		"i": true, "one": true, "there": true, "who": true, "what": true, "which": true,
		"that": true, "this": true, "these": true, "those": true, "everything": true,
		"nothing": true, "something": true, "anything": true, "all": true, "none": true,
	}

	// landingAbstractions are the abstraction nouns a landing equates its subject to.
	// The list is deliberately short and stays that way: each entry is a noun that
	// reframes rather than describes, so a concrete complement like "the primary key"
	// never reads as a payoff. A noun that ordinary technical prose predicates on
	// something real, "the cost" or "the bottleneck", is left off on purpose.
	landingAbstractions = map[string]bool{
		"point": true, "points": true, "constraint": true, "constraints": true,
		"tell": true, "difference": true, "catch": true, "trade-off": true,
		"tradeoff": true, "story": true, "game": true, "bet": true, "moat": true,
		"lesson": true, "takeaway": true, "upshot": true, "rest": true,
		"currency": true,
	}
)

// landingFindings reports every copula-abstraction landing in text. The finding only
// flags: rewording a landing means deciding what the sentence was trying to say, which is
// the author's call or the rewrite pass's.
func landingFindings(text string, protected [][2]int) []Finding {
	var out []Finding
	for _, sp := range sentenceSpans(text) {
		if inTableRow(text, sp.start) {
			continue
		}
		off, clause := closingClause(text[sp.start:sp.end])
		if !isCopulaLanding(clause) {
			continue
		}
		start, end := sp.start+off, sp.start+off+len(clause)
		if overlapsAny(protected, start, end) {
			continue
		}
		out = append(out, Finding{
			Rule:   "structural:copula-landing",
			Match:  text[start:end],
			Offset: start,
			order:  anaphoraOrder,
		})
	}
	return out
}

// closingClause returns the clause that closes sentence along with its byte offset in it.
// The clause opens after the last comma or semicolon that separates clauses, past any
// coordinating conjunction, so "It is usually about a third, and that number is the rest
// of the day." is read from "that number" on. Trailing sentence punctuation is dropped.
func closingClause(sentence string) (offset int, clause string) {
	body := strings.TrimRight(sentence, " \t\r\n.!?")
	start := 0
	if i := strings.LastIndexAny(body, ",;"); i >= 0 {
		start = i + 1
	}
	for start < len(body) && (body[start] == ' ' || body[start] == '\t' || body[start] == '\n' || body[start] == '\r') {
		start++
	}
	if word, tail, ok := strings.Cut(body[start:], " "); ok && landingConjunctions[strings.ToLower(word)] {
		start = len(body) - len(tail)
	}
	for start < len(body) && (body[start] == ' ' || body[start] == '\t') {
		start++
	}
	return start, strings.TrimSpace(body[start:])
}

// isCopulaLanding reports whether clause is a concrete or demonstrative subject equated to
// an abstraction by a bare copula. Both halves must hold: an abstraction complement alone
// is ordinary definition, "The problem is the cost", and a demonstrative subject alone is
// ordinary reference, "That column is the primary key".
func isCopulaLanding(clause string) bool {
	words := strings.Fields(clause)
	if len(words) < landingMinWords || len(words) > landingMaxWords {
		return false
	}
	verb := -1
	for i, w := range words {
		if landingCopulas[landingWord(w)] {
			verb = i
			break
		}
	}
	if verb < 1 || verb > landingMaxSubjectWords {
		return false
	}
	return landingSubject(words[:verb]) && landingComplement(words[verb+1:])
}

// landingSubject reports whether words form the concrete or demonstrative noun phrase a
// landing equates away. Its head must not itself be an abstraction, since an abstraction
// on both sides is a definition rather than a landing.
func landingSubject(words []string) bool {
	head := landingWord(words[len(words)-1])
	if head == "" || landingAbstractions[head] || landingPronouns[head] {
		return false
	}
	if len(words) == 1 {
		return plainWords(words[0])
	}
	if !landingSubjectDeterminers[landingWord(words[0])] {
		return false
	}
	return plainWords(strings.Join(words, " "))
}

// landingComplement reports whether words form the abstraction a landing equates its
// subject to: a definite determiner, an optional intensifier, an abstraction noun, and
// nothing after it but a short "of" tail. Anything else after the noun, a relative clause
// or a second preposition, means the sentence went on to say something.
func landingComplement(words []string) bool {
	if len(words) < 2 || len(words) > landingMaxComplementWords {
		return false
	}
	if !landingComplementDeterminers[landingWord(words[0])] {
		return false
	}
	rest := words[1:]
	if landingModifiers[landingWord(rest[0])] {
		rest = rest[1:]
	}
	if len(rest) == 0 || !landingAbstractions[landingWord(rest[0])] {
		return false
	}
	tail := rest[1:]
	return len(tail) == 0 || landingWord(tail[0]) == "of"
}

// landingWord normalizes one token to its bare lower-cased word, so a token carrying
// sentence punctuation compares against the word lists.
func landingWord(word string) string {
	return strings.ToLower(strings.Trim(word, `,.!?;:"'()[]`))
}
