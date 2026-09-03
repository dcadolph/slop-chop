package sanitize

import (
	"sort"
	"strings"
)

// Attacking the engine is how its blind spots get found on purpose rather than by a
// reader noticing. The attack rewrites text to dodge the rules while keeping the machine
// register intact: it swaps a listed buzzword for an unlisted cliche, a stock opener for
// an unstocked one, an em-dash for punctuation no rule reads. Nothing about the writing
// improves. Only the score moves.
//
// What survives is the finding. A word list is a lookup and loses to a thesaurus, which
// is why a word tell counts one. A sentence shape has to be rebuilt to escape, which no
// substitution can do, which is why a structural tell counts two. The attack is the
// experiment behind that weighting, and it reports honestly when the weighting is wrong.

// evasions maps a flagged word or phrase to a replacement that reads the same and hides
// from the rules. Every entry is checked against the default profile by the tests, so an
// evasion that stops evading is a failing build rather than a quiet lie in the report.
//
//nolint:gochecknoglobals // Immutable lookup.
var evasions = map[string]string{
	// Buzzwords, swapped for synonyms no list carries yet.
	"delve":          "dig",
	"leverage":       "tap",
	"robust":         "sturdy",
	"comprehensive":  "full",
	"seamlessly":     "smoothly",
	"utilize":        "employ",
	"myriad":         "countless",
	"synergy":        "alignment",
	"vibrant":        "lively",
	"crucial":        "central",
	"pivotal":        "central",
	"landscape":      "terrain",
	"realm":          "sphere",
	"navigate":       "work through",
	"foster":         "grow",
	"underscore":     "stress",
	"testament":      "proof",
	"tapestry":       "weave",
	"beacon":         "signal",
	"paradigm":       "model",
	"holistic":       "whole",
	"cutting-edge":   "advanced",
	"game-changer":   "turning point",
	"unlock":         "open up",
	"empower":        "enable",
	"streamline":     "tighten",
	"elevate":        "lift",
	"transformative": "sweeping",
	// Stock openers, swapped for openers nobody has listed.
	"in summary, ":              "to sum up, ",
	"in conclusion, ":           "to close, ",
	"moreover, ":                "on top of that, ",
	"furthermore, ":             "beyond that, ",
	"additionally, ":            "on top of that, ",
	"notably, ":                 "of note, ",
	"importantly, ":             "of note, ",
	"it is worth noting that ":  "one thing to flag is that ",
	"it's worth noting that ":   "one thing to flag is that ",
	"at the end of the day, ":   "when all is said, ",
	"first and foremost, ":      "before anything else, ",
	"that being said, ":         "with that in mind, ",
	"that said, ":               "with that in mind, ",
	"in essence, ":              "put simply, ",
	"at its core, ":             "put simply, ",
	"it goes without saying ":   "needless to add ",
	"in today's digital age, ":  "in the current era, ",
	"in today's world, ":        "in the current era, ",
	"navigating the landscape ": "working the terrain ",
}

// charEvasions maps a flagged character to punctuation that carries the same pause and
// trips nothing. The em-dash becomes an ellipsis rather than the comma a fix would use,
// since the point is to hide the tell, not to write the sentence properly.
//
//nolint:gochecknoglobals // Immutable lookup.
var charEvasions = map[string]string{
	"—": "...",
	"–": "...",
}

// Evasion records one substitution the attack made and the rule it dodged.
type Evasion struct {
	// Rule is the rule that flagged the text before the swap.
	Rule string `json:"rule"`
	// Was is the text the rule matched.
	Was string `json:"was"`
	// Now is what replaced it.
	Now string `json:"now"`
}

// AttackResult is what an attack found: the rewritten text, the swaps that dodged a rule,
// and the findings that survived every swap.
type AttackResult struct {
	// Text is the attacked text, which reads the same and scores lower.
	Text string `json:"text"`
	// Evasions are the substitutions that removed a finding, in document order.
	Evasions []Evasion `json:"evasions"`
	// Survived are the findings still present after the attack, the rules a thesaurus
	// cannot beat.
	Survived []Finding `json:"survived"`
	// ScoreBefore and ScoreAfter are the scores of the original and attacked text.
	ScoreBefore int `json:"scoreBefore"`
	ScoreAfter  int `json:"scoreAfter"`
	// ByClass counts, per rule class, how many findings the attack evaded and how many
	// resisted, which is the number the score weights rest on.
	ByClass map[string]ClassResult `json:"byClass"`
}

// ClassResult is one rule class's tally in an attack.
type ClassResult struct {
	// Evaded is how many findings of this class a substitution removed.
	Evaded int `json:"evaded"`
	// Resisted is how many survived every substitution the attack knows.
	Resisted int `json:"resisted"`
}

// Attack rewrites text to dodge as many rules as it can without improving the writing,
// and reports what still fires. It changes no rule and writes no file: the result is a
// measurement of how much of the ruleset a determined evader can walk past.
func (s *Sanitizer) Attack(text string) AttackResult {
	before := s.Score(text)
	out, evaded := s.evade(text)
	after := s.Score(out)
	survived := s.Check(out)

	byClass := map[string]ClassResult{}
	for _, e := range evaded {
		c := attackClass(e.Rule)
		r := byClass[c]
		r.Evaded++
		byClass[c] = r
	}
	for _, f := range survived {
		c := attackClass(f.Rule)
		r := byClass[c]
		r.Resisted++
		byClass[c] = r
	}
	return AttackResult{
		Text:        out,
		Evasions:    evaded,
		Survived:    survived,
		ScoreBefore: before.Value,
		ScoreAfter:  after.Value,
		ByClass:     byClass,
	}
}

// evade applies every substitution it has to the findings in text, latest match first so
// each rewrite leaves the earlier offsets valid.
func (s *Sanitizer) evade(text string) (string, []Evasion) {
	findings := s.Check(text)
	// Work back to front, so replacing a span never shifts the offset of one not yet
	// handled.
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Offset > findings[j].Offset })

	var applied []Evasion
	out := text
	used := map[int]bool{}
	for _, f := range findings {
		if used[f.Offset] {
			continue
		}
		repl, ok := evasionFor(f)
		if !ok {
			continue
		}
		start, end := f.Offset, f.Offset+len(f.Match)
		// A dash fenced by spaces becomes " ..." unless the leading space goes with it,
		// which would leave a space before punctuation: a new finding, not an evasion.
		if _, isChar := charEvasions[f.Match]; isChar && start > 0 && out[start-1] == ' ' {
			start--
		}
		if end > len(out) || out[f.Offset:end] != f.Match {
			// The text moved under a previous swap, so this offset is no longer the
			// match. Skipping is right: the report counts it as resisted rather than
			// claiming an evasion that never happened.
			continue
		}
		out = out[:start] + repl + out[end:]
		used[f.Offset] = true
		applied = append(applied, Evasion{Rule: f.Rule, Was: f.Match, Now: repl})
	}
	// Report in document order, which is how a reader follows the text.
	sort.SliceStable(applied, func(i, j int) bool { return applied[i].Was < applied[j].Was })
	return out, applied
}

// evasionFor returns the substitution for one finding, carrying the match's capitalization
// so an evaded sentence still opens with a capital.
func evasionFor(f Finding) (string, bool) {
	if repl, ok := charEvasions[f.Match]; ok {
		return repl, true
	}
	key := strings.ToLower(f.Match)
	if repl, ok := evasions[key]; ok {
		return matchCase(f.Match, repl), true
	}
	// A phrase finding carries the letter after it, since deleting the phrase would
	// recapitalize that word. Swap the longest evasion that opens the match and keep
	// whatever trailed it, so "In summary, w" becomes "To sum up, w".
	best := ""
	for k := range evasions {
		if len(k) > len(best) && strings.HasPrefix(key, k) {
			best = k
		}
	}
	if best == "" {
		return "", false
	}
	return matchCase(f.Match, evasions[best]) + f.Match[len(best):], true
}

// attackClass names the class an attack report groups a rule under: the part before the
// colon, or "tidy" for the cleanup rules that carry no score weight.
func attackClass(rule string) string {
	if c, _, ok := strings.Cut(rule, ":"); ok {
		return c
	}
	return "tidy"
}
