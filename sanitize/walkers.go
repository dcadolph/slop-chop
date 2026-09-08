package sanitize

// Walker names one structural detector that runs as a pass over sentences rather than as
// a compiled pattern. A shape like "the same two words again" or "this stem turned
// against itself" has no regular expression, so these walk the text directly. They flag
// and never rewrite, the same as the compiled structural patterns.
type Walker struct {
	// Name is the finding name without its "structural:" prefix.
	Name string
	// Reads says in one line what the walker looks for.
	Reads string
}

// Walkers returns the structural walkers in the order their findings sort, so the
// published catalog lists what the engine actually runs rather than a hand-kept copy.
func Walkers() []Walker {
	return []Walker{
		{
			Name:  "anaphora-run",
			Reads: "Three or more short sentences in a row, in one paragraph, opening with the same two words. A run that repeats one sentence word for word is emphasis and stays.",
		},
		{
			Name:  "uniform-paragraphs",
			Reads: "Most body paragraphs holding the same number of sentences, the shape of prose stamped from a template.",
		},
		{
			Name:  "template-stem",
			Reads: "One opening stem repeated across paragraphs, an outline talking rather than a writer.",
		},
		{
			Name:  "copula-landing",
			Reads: "A concrete or demonstrative subject equated to an abstraction by a bare copula, closing the sentence as a payoff: `That number is the rest of the day.` Both halves have to hold, so `That column is the primary key` and `The problem is the cost` are left alone.",
		},
		{
			Name:  "polyptoton",
			Reads: "A stem turned against itself in one sentence, with a pivot and a negation between the two forms: `verification that does not verify`. Bare repetition is ordinary prose, so `the parser parses` is left alone.",
		},
	}
}
