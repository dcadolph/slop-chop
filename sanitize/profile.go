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
	// WordReplace maps a whole word to its replacement, matched case-insensitively with
	// the match's capitalization carried onto the replacement. Unlike a block word it
	// rewrites, so it is the safe way to swap one word for another.
	WordReplace map[string]string `json:"wordReplace"`
	// RegexReplace maps a regular expression to its replacement. The pattern is used as
	// written, so the caller controls anchoring, and a reference like $1 in the
	// replacement expands against the match.
	RegexReplace map[string]string `json:"regexReplace"`
	// BlockWords are words flagged wherever they appear. They are reported but never
	// rewritten, since a safe replacement depends on context.
	BlockWords []string `json:"blockWords"`
	// FlagPatterns maps a rule name to a regular expression that only flags its matches,
	// never rewrites them. It catches structural tells a word list cannot, like the
	// "not just X, but Y" cadence, where the fix depends on the whole sentence and is
	// left to the rewrite pass.
	FlagPatterns map[string]string `json:"flagPatterns"`
	// Allow lists words a rule must never flag or rewrite, matched case-insensitively
	// against the exact text a rule matched. It silences false positives.
	Allow []string `json:"allow"`
	// CollapseSpaces collapses runs of two or more spaces into one and removes spaces
	// and stray commas left before closing punctuation, like the debris an em-dash swap
	// or a dropped word leaves behind. Runs at the start of a line are indentation and
	// stay as they are.
	CollapseSpaces bool `json:"collapseSpaces"`
	// SplitSemicolons turns "; " into ". " and capitalizes the next word.
	SplitSemicolons bool `json:"splitSemicolons"`
	// FixArticles corrects "a" and "an" to the sound of the word that follows, so a swap that
	// changes a word's first sound never leaves "an new". On by default.
	FixArticles bool `json:"fixArticles"`
	// ProtectQuotes leaves text inside double quotation marks unchanged, straight or smart,
	// so a quoted source is not reworded. Off by default, since cleaning your own draft
	// should reach inside your own quotes.
	ProtectQuotes bool `json:"protectQuotes"`
	// Tone holds optional notes on the voice to aim for. The rules pass ignores it.
	// The rewrite pass feeds it to the model so output sounds like you.
	Tone []string `json:"tone"`
	// Dialect enforces a spelling variant. "american" flags British spellings and
	// rewrites them, "british" does the reverse, and an empty value or "off" leaves
	// spelling alone.
	Dialect Dialect `json:"dialect"`
}

// DefaultProfile returns the built-in profile that targets common AI tells.
func DefaultProfile() Profile {
	return Profile{
		CharReplace: map[string]string{
			"\u00a0": " ",   // non-breaking space to a normal space
			"\u202f": " ",   // narrow non-breaking space to a normal space
			"\u200b": "",    // zero-width space, usually paste cruft
			"\u2060": "",    // word joiner, usually paste cruft
			"\ufeff": "",    // zero-width no-break space or a stray byte-order mark
			"\u00ad": "",    // soft hyphen, invisible and a word-smuggling channel
			"\u2010": "-",   // unicode hyphen to the ASCII one
			"\u2011": "-",   // non-breaking hyphen to the ASCII one
			"\u2012": "-",   // figure dash to a hyphen
			"\u2015": ", ",  // horizontal bar doing an em-dash's job
			"—":      ", ",  // em-dash
			"–":      "-",   // en-dash
			"‘":      "'",   // left single quote
			"’":      "'",   // right single quote
			"“":      `"`,   // left double quote
			"”":      `"`,   // right double quote
			"…":      "...", // ellipsis
		},
		PhraseReplace: map[string]string{
			"additionally, ":                    "",
			"consequently, ":                    "",
			"furthermore, ":                     "",
			"importantly, ":                     "",
			"it is important to note that ":     "",
			"it's important to note that ":      "",
			"more importantly, ":                "",
			"moreover, ":                        "",
			"notably, ":                         "",
			"rest assured, ":                    "",
			"that being said, ":                 "",
			"with that said, ":                  "",
			"at its core, ":                     "",
			"at the end of the day, ":           "",
			"first and foremost, ":              "",
			"giving it to you honestly, ":       "",
			"in a nutshell, ":                   "",
			"in conclusion, ":                   "",
			"in essence, ":                      "",
			"in summary, ":                      "",
			"in today's digital age, ":          "",
			"in today's fast-paced world, ":     "",
			"in today's world, ":                "",
			"it goes without saying that ":      "",
			"it is important to remember that ": "",
			"it's important to remember that ":  "",
			"it is worth mentioning that ":      "",
			"it is worth noting that ":          "",
			"it's worth mentioning that ":       "",
			"it's worth noting that ":           "",
			"keep in mind that ":                "",
			"last but not least, ":              "",
			"needless to say, ":                 "",
			"overall, ":                         "",
			"simply put, ":                      "",
			"to be honest, ":                    "",
			"to put it simply, ":                "",
			"to recap, ":                        "",
			"to recap: ":                        "",
			"to summarize, ":                    "",
			"to summarize: ":                    "",
			"in summary: ":                      "",
			"in conclusion: ":                   "",
			"ultimately, ":                      "",
			"without further ado, ":             "",
		},
		BlockWords: []string{
			"best-in-class", "blast radius", "blazing fast", "blazingly fast",
			"boast", "boasted", "boasting", "boasts", "bustling",
			"comprehensive", "crucial", "cutting edge", "cutting-edge", "daunting",
			"delve", "delved", "delves", "delving", "dive deeper",
			"effortless", "effortlessly", "elegant", "elevate", "elevates", "elevating",
			"embark", "embarked", "embarking", "embarks", "empower", "empowering", "empowers",
			"ever-changing", "ever-evolving", "facilitate", "facilitates", "facilitating",
			"fascinating", "fast-paced", "foster", "fostering", "fosters", "frictionless",
			"future-proof", "future-proofing", "future-proofs",
			"game changer", "game changers", "game-changer", "game-changers", "game-changing",
			"groundbreaking", "harness the power", "has something for everyone", "holistic",
			"in the realm of", "in the world of", "innovative", "invaluable",
			"leverage", "leveraged", "leverages", "leveraging", "look no further",
			"meticulous", "meticulously", "more than ever", "must-see", "must-visit",
			"myriad", "nestled", "paradigm shift", "pivotal", "plethora",
			"powerful", "revolutionize", "revolutionized", "revolutionizes", "revolutionizing",
			"robust", "seamless", "seamlessly", "showcase", "showcased", "showcases",
			"showcasing", "state-of-the-art", "streamline", "streamlined", "streamlines",
			"streamlining", "supercharge", "supercharged", "synergies", "synergy",
			"stands as a", "tapestry", "testament to", "the possibilities are endless",
			"to the next level", "top-notch", "transformative", "treasure trove",
			"unleash", "unleashed", "unleashes", "unleashing",
			"unlock the full potential", "unlock the potential", "unparalleled",
			"unprecedented",
			"utilize", "utilized", "utilizes", "utilizing", "vibrant", "whopping",
			"world-class",
			// Stock metaphors and set phrases that carry no information of their own.
			"a beacon of", "actionable insights", "at the forefront",
			"bespoke", "deep dive", "double-edged sword",
			"navigate the complexities", "navigating the complexities",
			"pave the way", "paved the way", "paves the way", "paving the way",
			"peace of mind", "perfect blend", "shed light on", "sheds light on",
			"stark contrast", "stark reality", "stark reminder",
			"tailored to your", "the intersection of", "when it comes to",
			"a marathon, not a sprint", "multifaceted",
			"competitive landscape", "changing landscape", "current landscape",
			"digital landscape", "evolving landscape",
			// Worn metaphors a model reaches for when it has nothing specific to say.
			"tip of the iceberg", "scratching the surface", "the proof is in the pudding",
			"where the rubber meets the road", "the writing is on the wall",
			"breath of fresh air", "best of both worlds", "speak for themselves",
			"opens up a world of", "a world of possibilities", "time will tell",
			"the results speak", "we've got you covered", "you're in good hands",
		},
		FlagPatterns: map[string]string{
			// "It's not just X, it's Y" and its "this is not X, it's Y" cousin, matched in
			// the contracted "it's" and the spelled-out "this is not" forms alike. The join
			// may be a comma, a semicolon, or a full stop, so "That's not a tooling problem.
			// That's a visibility problem." is the same tell across a sentence break.
			"its-not-x-its-y": `(?i)\b(?:it|this|that)(?:'?s|\s+(?:is|was|are|were))(?:\s+not|n'?t)\b[^.!?\n]{1,40}[,;.]\s*(?:it'?s|that'?s|this is)\b`,
			// "not just X but also Y" and "not only X but also Y".
			"not-just-but-also": `(?i)\bnot (just|only)\b[^.!?\n]{1,60}\bbut\b[^.!?\n]{0,25}\balso\b`,
			// Throat-clearing openers that promise a payoff.
			"heres-the-thing": `(?i)\bhere'?s the (thing|kicker|deal|catch|secret|problem)\b`,
			// The "let's dive in" invitation and its "let's take a closer look" cousins.
			"lets-dive-in":     `(?i)\blet'?s (dive|delve|jump) in(to)?\b`,
			"lets-take-a-look": `(?i)\blet'?s (?:take a (?:closer )?look|explore|unpack|break (?:it|this) down)\b`,
			// Chatbot reply openers, anchored to a line start where an opener lives.
			"assistant-opener": `(?im)^\s{0,3}(?:certainly|absolutely|great question|i'?d be happy to|happy to help)\b`,
			// Chatbot sign-offs, unanchored since they land at the end of a paragraph.
			"assistant-signoff": `(?i)\b(?:i hope this helps|hope this helps|don'?t hesitate to|feel free to reach out)\b`,
			// A stack of hedges in one breath, the noncommittal AI register. Matched in
			// lower case and sentence case only: an all-caps MAY or SHOULD is an RFC 2119
			// normative keyword, the opposite of a hedge.
			"hedge-stack": `\b(?:[Mm]ay|[Mm]ight|[Cc]ould|[Pp]ossibly|[Pp]erhaps|[Aa]rguably|[Gg]enerally|[Pp]otentially|[Ss]omewhat|[Ss]eemingly|[Pp]resumably|[Cc]onceivably)\b[^.!?\n]{1,50}\b(?:[Mm]ay|[Mm]ight|[Cc]ould|[Pp]ossibly|[Pp]erhaps|[Aa]rguably|[Gg]enerally|[Pp]otentially|[Ss]omewhat|[Ss]eemingly|[Pp]resumably|[Cc]onceivably)\b`,
			// "That's where X comes in", the setup-and-reveal move.
			"thats-where-comes-in": `(?i)\bthat'?s where\b[^.!?\n]{1,30}\bcomes? in\b`,
			// The fragment-question reveal, "The best part? It's free."
			"fragment-reveal": `(?i)\bthe (?:best part|result|catch|takeaway|upshot|kicker|bottom line|good news|bad news|verdict|payoff)\?`,
			// "Here are five ways to ..." enumeration openers.
			"here-are-n": `(?i)\bhere are (?:\d+|a few|some|several|three|four|five|six|seven|eight|nine|ten) (?:key |simple |quick |easy |practical |common )?(?:reasons|ways|things|tips|steps|takeaways|strategies|examples|benefits|best practices)\b`,
			// Three or more consecutive bullets that each open with a bold label. The label
			// must open on a letter or digit, so a reference list of bolded flags or code
			// tokens like "**--json**" is left alone.
			"bold-bullet-run": `(?m)(?:^[ \t]*[-*][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*.*\n){2}[ \t]*[-*][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*`,
			// The audience-flattering "whether you're a novice or an expert" frame. Requiring
			// the articles keeps the ordinary "whether you're coming or not" out of reach.
			"whether-youre": `(?i)\bwhether you'?re an? [^.!?\n]{1,40}\bor an? \p{L}`,
			// "Think of it as ..." analogy opener.
			"think-of-it-as": `(?i)\bthink of (?:it|this|them) as\b`,
			// Negative parallelism split across a sentence break, "not just a tool. You're
			// investing", the form the comma version turned into once it became a known tell.
			"not-just-sentence-split": `(?i)\bnot just\b[^.!?\n]{1,60}\.[ \t]+(?:it'?s|you'?re|we'?re|they'?re|this is|that'?s)\b`,
			// A spaced hyphen or double hyphen doing an em-dash's job mid-sentence, the
			// swap models reached for once the em-dash itself became a tell. Lower-case
			// letters on both sides keep a range like "Monday - Friday" out of reach.
			"spaced-hyphen": `\p{Ll} -{1,2} \p{Ll}`,
			// "In an era where ..." grand-context opener. The "of" form is not listed:
			// "in an age of sail" is ordinary historical prose.
			"in-an-era": `(?i)\bin an? (?:era|age|world) (?:where|when)\b`,
			// The "not only does X" inversion, a formal flourish models overuse.
			"not-only-inversion": `(?i)\bnot only (?:does|do|did|is|are|was|were|can|could|will|would)\b`,
			// "Plays a crucial role in ..." importance-claiming filler.
			"plays-a-role": `(?i)\bplay(?:s|ed|ing)? an? (?:crucial|key|vital|pivotal|central|significant|essential) role\b`,
			// Stock closing headings on generated articles. A plain "Conclusion" heading is
			// not listed: it is near-mandatory in academic and report writing.
			"conclusion-heading": `(?im)^#{1,6}[ \t]+(?:final thoughts|wrapping up|in closing|key takeaways)\b`,
			// An emoji decorating a bullet or heading, the generated-listicle look.
			"emoji-decoration": `(?m)^[ \t]*(?:[-*][ \t]+|#{1,6}[ \t]+)\*{0,2}[\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{1F000}-\x{1FAFF}]`,
			// The contracted negative parallelism, "it isn't a perk. It's an expectation",
			// which the spelled-out "not just" patterns walk straight past.
			"contracted-not-just": `(?i)\b(?:is|are|was|were|do|does|did|has|have)n'?t\b[^.!?\n]{1,60}[.,]\s*(?:it'?s|they'?re|you'?re|we'?re|that'?s|this is)\b`,
			// "Let's be clear" and the other throat-clearing declaratives.
			"lets-be-clear": `(?i)\b(?:let'?s be clear|make no mistake|the truth is|here'?s the reality)\b`,
			// A rhetorical question aimed at the reader, the engagement-bait opener. The
			// ordinary "the question is:" is not listed; it is common human phrasing.
			"rhetorical-hook": `(?i)\b(?:ever wondered|have you ever wondered|what if i told you|sound familiar\?|why does (?:this|it) matter\?|worth asking (?:yourself|whether|if)\b)`,
			// "The answer lies in" and its cousins, the manufactured reveal.
			"the-answer-lies": `(?i)\bthe (?:answer|secret|key|difference|trick) (?:lies in|is (?:simple|straightforward)|comes down to)\b`,
			// "Has never been more important", the urgency claim with no evidence.
			"never-been-more": `(?i)\b(?:has|have) never been more (?:important|critical|relevant|urgent|clear|apparent|necessary)\b`,
			// A numbered list whose items each open with a bold label, the ordered twin of
			// bold-bullet-run, with the same letter-or-digit opener guard.
			"bold-number-run": `(?m)(?:^[ \t]*\d+[.)][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*.*\n){2}[ \t]*\d+[.)][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*`,
			// "The ultimate guide to", the stock generated-article title.
			"ultimate-guide": `(?i)\bthe (?:ultimate|complete|definitive|essential) guide to\b`,
			// "Underscores the importance", a claim verb standing in for evidence.
			"underscores-the": `(?i)\b(?:underscore|underscores|underscoring|highlights|underlines) the (?:importance|need|value|fact|reality|significance)\b`,
			// The narrator stepping in to announce what it is about to do, the register of a
			// reply rather than a piece of writing.
			"chatbot-scaffolding": `(?i)\blet me (?:explain|break (?:this|it) down|walk you through|show you|start by)\b|\bhere'?s how (?:it works|this works|you do it)\b`,
			// "You might be wondering", the question the writer puts in the reader's mouth.
			"reader-mind-reading": `(?i)\b(?:you (?:might|may|could) be (?:wondering|asking|thinking)|you'?re probably (?:wondering|thinking)|so,? what does this mean for you)\b`,
			// "But here's where it gets interesting", the manufactured turn.
			"manufactured-turn": `(?i)\b(?:here'?s where it gets (?:interesting|good|tricky|fun)|but here'?s the (?:twist|thing))\b`,
			// Sign-offs that close a chat reply rather than a piece of writing.
			"chat-signoff": `(?i)\b(?:happy (?:coding|building|shipping|writing)!|thanks for reading|until next time|let me know if you have any questions|if you have any questions,? (?:just )?ask|hopefully this (?:gives|helps))`,
			// The before-and-after pitch, "say goodbye to X" and "gone are the days".
			"marketing-reveal": `(?i)\b(?:say (?:goodbye|hello) to|gone are the days|no more (?:wrestling|fighting|struggling) with|imagine a world where|picture this:)`,
			// "Enter Foo, the tool that", the product introduced as the answer to a setup. The
			// appositive article is required, so the ordinary imperative "Enter Berlin, then
			// pick a date" is left alone.
			"enter-the-product": `(?:^|[.!?]\s+)Enter \p{Lu}[\p{L}-]+,\s+(?:the|a|an)\b`,
			// Certainty asserted instead of shown, the confident filler that carries no
			// evidence with it.
			"asserted-certainty": `(?i)\b(?:there'?s no denying|without a doubt|it'?s clear that|it is clear that|suffice it to say|needless to say|make no mistake about it)\b`,
			// The summary flourish that announces a conclusion rather than reaching one.
			"summary-flourish": `(?i)\b(?:the bottom line is|long story short|at its heart|the beauty of (?:it|this)|that'?s the beauty of|the best part is)\b`,
			// "What if there were a better way", the infomercial setup.
			"what-if-better-way": `(?i)\bwhat if (?:there (?:were|was)|i told you|you could)\b`,
			// Reader-instruction openers, the cousins of "it is important to note" that a
			// phrase delete cannot swallow because the sentence continues through them.
			"reader-instruction": `(?i)\b(?:it'?s important to (?:understand|remember|realize)|one thing to keep in mind|the (?:one )?thing to remember)\b`,
			// The "It worked. Until it didn't." snap, a full sentence of reversal after a
			// stop, the signature cadence of 2025-era model prose.
			"until-it-didnt": `(?i)[.!?]\s+until (?:it|they|we|that) (?:didn'?t|wasn'?t|weren'?t|doesn'?t|couldn'?t|stopped)\b`,
			// The reversal aphorism, "Tools don't fail teams. Blind spots do.": a negated
			// claim, then a short sentence closing on a bare auxiliary.
			"reversal-aphorism": `(?i)\b(?:don'?t|doesn'?t|isn'?t|aren'?t|never|not)\b[^.!?\n]{0,40}\.\s+\p{Lu}[^.!?\n]{2,35}\s(?:do(?:es)?|did|is|are|was|were|will|can)\.`,
			// "The question isn't whether X. The question is whether Y.", the parallel
			// reframe delivered as two sentences.
			"question-isnt-is": `(?i)\bthe (?:question|point|problem|issue|goal|answer) isn'?t\b[^.!?\n]{1,60}\.\s+the (?:question|point|problem|issue|goal|answer) is\b`,
			// "It's not about X. It's about Y.", the same reframe in its "about" form.
			"not-about-about": `(?i)\b(?:it|this)'?s not about\b[^.!?\n]{1,50}\.\s+it'?s about\b`,
			// A whole sentence of exactly three bare words, "Simple, boring, reliable.",
			// the triad fragment models drop in for punch. A human list nearly always takes
			// an "and" before its last item, so the bare form is the tell.
			"triad-fragment": `(?m)(?:^|[.!?][ \t]+)\p{Lu}\p{Ll}{2,14}, \p{Ll}{2,14}, \p{Ll}{2,14}\.`,
			// "Firstly, ... Secondly, ..." enumeration adverbs, the essay-skeleton tell.
			"ordinal-enumeration": `(?i)\b(?:firstly|secondly|thirdly|fourthly|lastly),`,
			// The canonical assistant self-reference.
			"as-an-ai": `(?i)\bas an ai(?: language)? model\b`,
			// The uncontracted "not just a library but a whole platform" form, which the
			// "but also" pattern walks past. Articles on both sides keep ordinary "not just
			// for fun but because" contrasts out of reach.
			"not-just-a-but": `(?i)\bnot (?:just|merely|simply) (?:a|an|the)\b[^.!?\n]{1,40}\bbut (?:a|an|the)\b`,
			// A Latin letter pressed against a Cyrillic or Greek one inside a word, the
			// homoglyph smuggling that hides a tell from every word list. Flag only: the
			// safe respelling is the author's call.
			"mixed-script": `(?:\p{Latin}[\p{Cyrillic}\p{Greek}]|[\p{Cyrillic}\p{Greek}]\p{Latin})`,
		},
		Allow: []string{
			// Technical collocations where a flagged word is a term of art, protected so a
			// swap never turns "robust regression" into "solid regression".
			"robust regression", "robust standard errors", "robust estimator",
			"robust estimation", "robust statistics", "robust control",
			"optimal substructure", "optimal control", "optimal transport",
			"optimal policy", "optimal stopping",
			"comprehensive exam", "comprehensive examination",
			"comprehensive income", "comprehensive incomes",
			"leverage ratio", "leverage ratios", "operating leverage",
			"financial leverage", "leveraged buyout", "leveraged buyouts",
			"foster care", "foster child", "foster children", "foster family",
			"foster home", "foster parent", "foster parents",
		},
		CollapseSpaces:  true,
		SplitSemicolons: true,
		FixArticles:     true,
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

	spelling, ok, err := spellingRule(p.Dialect)
	if err != nil {
		return nil, err
	}
	if ok {
		rules = append(rules, spelling)
	}

	swaps, drops := splitDrops(p.WordReplace)
	replace, ok, err := wordSwapRule("replace", lowerKeys(swaps))
	if err != nil {
		return nil, err
	}
	if ok {
		replace.keep = notProperNoun
		rules = append(rules, replace)
	}
	for _, w := range drops {
		r, err := deletionRule("drop:"+w, w)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	for _, pat := range slices.Sorted(maps.Keys(p.RegexReplace)) {
		r, err := regexRule(pat, p.RegexReplace[pat])
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	block, ok, err := blockWordRule(p.BlockWords)
	if err != nil {
		return nil, err
	}
	if ok {
		block.keep = notProperNoun
		rules = append(rules, block)
	}

	for _, name := range slices.Sorted(maps.Keys(p.FlagPatterns)) {
		re, err := regexp.Compile(flexPatternSpaces(p.FlagPatterns[name]))
		if err != nil {
			return nil, fmt.Errorf("%w: flag pattern %q: %w", ErrCompile, name, err)
		}
		rules = append(rules, Rule{
			Name:    "structural:" + name,
			re:      re,
			rewrite: false,
			unwrap:  true,
		})
	}

	if p.SplitSemicolons {
		rules = append(rules, Rule{
			// The pattern stays within one line, so a semicolon before a line break never
			// swallows the newline and reflows the paragraph.
			Name:     "semicolon",
			re:       regexp.MustCompile(`;[ \t]+(\p{L})`),
			replFunc: splitSemicolon,
			keep: func(text string, start, _ int) bool {
				return semicolonJoinsClauses(text, start)
			},
			rewrite: true,
			tidy:    true,
		})
	}

	if p.CollapseSpaces {
		rules = append(rules, Rule{
			// Runs before space-before-punct so the sentence's separating space is still
			// there to keep, not eaten as space before a comma.
			Name:     "orphan-comma",
			re:       regexp.MustCompile(`,[ \t]*(\p{L})`),
			replFunc: stripOrphanComma,
			keep:     commaOpensSentence,
			rewrite:  true,
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:     "space-before-punct",
			re:       regexp.MustCompile(`[ \t]+[,.!?;:]`),
			replFunc: trimLeadingSpace,
			keep:     spaceBeforePunctKeep,
			rewrite:  true,
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:     "comma-before-stop",
			re:       regexp.MustCompile(`,+[.!?;:]`),
			replFunc: keepFinalByte,
			rewrite:  true,
			tidy:     true,
		})
		rules = append(rules, Rule{
			Name:    "comma-run",
			re:      regexp.MustCompile(`,{2,}`),
			repl:    ",",
			rewrite: true,
			tidy:    true,
		})
		rules = append(rules, Rule{
			Name:    "double-space",
			re:      regexp.MustCompile(`  +`),
			repl:    " ",
			keep:    collapsibleRun,
			rewrite: true,
			tidy:    true,
		})
	}

	if p.FixArticles {
		rules = append(rules, Rule{Name: "article", re: articleRe, replFunc: fixArticle, keep: articleNeedsFix, rewrite: true, tidy: true})
	}

	if allow := allowSet(p.Allow); allow != nil {
		for i := range rules {
			rules[i].allow = allow
		}
	}

	return rules, nil
}

// allowPhraseRe compiles the multi-word entries of an allow list into one alternation whose
// matches are protected from every rule, so a term of art like "robust regression" keeps its
// word even when the bare word is a tell. Single-word entries are left to the per-rule allow
// set. It returns nil when there are no multi-word entries.
func allowPhraseRe(allow []string) (*regexp.Regexp, error) {
	var parts []string
	for _, a := range allow {
		fields := strings.Fields(a)
		if len(fields) < 2 {
			continue
		}
		parts = append(parts, flexSpaces(regexp.QuoteMeta(strings.Join(fields, " "))))
	}
	if len(parts) == 0 {
		return nil, nil
	}
	slices.Sort(parts)
	re, err := regexp.Compile(`(?i)\b(?:` + strings.Join(parts, "|") + `)\b`)
	if err != nil {
		return nil, fmt.Errorf("%w: allow phrases: %w", ErrCompile, err)
	}
	return re, nil
}

// allowSet turns the allow list into a lower-cased lookup, or nil when it is empty.
func allowSet(words []string) map[string]bool {
	if len(words) == 0 {
		return nil
	}
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[strings.ToLower(w)] = true
	}
	return set
}

// splitDrops separates word entries that swap in a new word from those that cut a word.
// A blank target marks a drop, which deletionRule handles so the cut leaves no double
// space or orphaned capital. The drops come back sorted for a stable rule order.
func splitDrops(m map[string]string) (swaps map[string]string, drops []string) {
	swaps = make(map[string]string, len(m))
	for k, v := range m {
		if k == "" {
			continue
		}
		if v == "" {
			drops = append(drops, k)
			continue
		}
		swaps[k] = v
	}
	slices.Sort(drops)
	return swaps, drops
}

// lowerKeys returns m with every key lower-cased and empty keys dropped, the shape
// wordSwapRule expects for case-insensitive matching. Values are left as written, so a
// replacement's intended capitalization, like "GitHub" or "iPhone", survives instead of
// being flattened to lower case. It returns nil for an empty map.
func lowerKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k == "" {
			continue
		}
		out[strings.ToLower(k)] = v
	}
	return out
}

// regexRule compiles a user regular expression into a rewriting rule. The pattern is used
// as written, so the caller controls anchoring and boundaries, and a reference like $1 in
// the replacement expands against the match. Zero-width matches are skipped so a pattern
// that can match nothing does not insert its replacement between every character.
func regexRule(pattern, repl string) (Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: regex %q: %w", ErrCompile, pattern, err)
	}
	return Rule{
		Name: "regex:" + pattern,
		re:   re,
		replFunc: func(text string, loc []int) string {
			// loc already holds the submatch indices from matching the full text, so the
			// replacement expands against the original with its surrounding context intact.
			// Re-running the pattern on the isolated span would drop the preceding character
			// a boundary anchor like \b or \B depends on, silently voiding the swap.
			return string(re.ExpandString(nil, repl, text, loc))
		},
		keep:    func(_ string, start, end int) bool { return end > start },
		rewrite: true,
	}, nil
}

// wsGap matches the whitespace between two words: spaces and tabs crossing at most one
// line break, LF or CRLF. It lets a phrase or a multi-word term match when a line wrap
// splits it, without ever reaching across a paragraph break.
const wsGap = `(?:[ \t]+(?:\r?\n[ \t]*)?|\r?\n[ \t]*)`

// flexSpaces widens each literal space in a quoted pattern into wsGap, so the words
// around it still match when a line wrap sits between them.
func flexSpaces(quoted string) string {
	return strings.ReplaceAll(quoted, " ", wsGap)
}

// flexPatternSpaces widens each literal space in a hand-written pattern into a run of one
// or more spaces and tabs, so a flag pattern still matches where a wrap left extra
// indentation between two words. Unlike flexSpaces the input is a real pattern, not quoted
// text, so a space inside a character class is left alone: widening the one in "[ \t]+"
// would break the class it belongs to. A space after a backslash is an escaped literal and
// is widened like any other.
func flexPatternSpaces(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern))
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			if pattern[i+1] == ' ' && !inClass {
				b.WriteString(`[ \t]+`)
			} else {
				b.WriteByte(c)
				b.WriteByte(pattern[i+1])
			}
			i++
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		case c == ' ' && !inClass:
			b.WriteString(`[ \t]+`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// blockWordRule compiles every block word into one flag-only rule. Folding them into a
// single alternation turns a full-text scan per word into one scan, and nameByMatch keeps
// each finding named for the word it caught. Longer words sort first so a longer term wins
// over a shorter one it contains at the same spot. It returns ok false for an empty list.
func blockWordRule(words []string) (Rule, bool, error) {
	alts := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			alts = append(alts, w)
		}
	}
	if len(alts) == 0 {
		return Rule{}, false, nil
	}
	slices.SortFunc(alts, func(a, b string) int {
		if d := len(b) - len(a); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	parts := make([]string, len(alts))
	for i, w := range alts {
		parts[i] = `\b` + flexSpaces(regexp.QuoteMeta(w)) + `\b`
	}
	re, err := regexp.Compile(`(?i)(?:` + strings.Join(parts, "|") + `)`)
	if err != nil {
		return Rule{}, false, fmt.Errorf("%w: block words: %w", ErrCompile, err)
	}
	return Rule{Name: "word", re: re, rewrite: false, nameByMatch: true}, true, nil
}

// endsWithWordChar reports whether s ends in an ASCII word character, so a closing \b
// boundary is added only where it would hold.
func endsWithWordChar(s string) bool {
	return s != "" && isWordByte(s[len(s)-1])
}

// phraseRule builds the rule for one phrase swap. A leading word boundary keeps the
// phrase from matching inside another word. A deletion is handled by deletionRule, which
// restores a sentence's opening capital. A non-empty replacement is a plain swap.
func phraseRule(phrase, repl string) (Rule, error) {
	if repl == "" {
		return deletionRule("phrase:"+strings.TrimSpace(phrase), phrase)
	}
	trimmed := strings.TrimRight(phrase, " ")
	core := `(?i)\b` + flexSpaces(regexp.QuoteMeta(trimmed))
	// A phrase ending in a word character gets a closing boundary so a key like "cat"
	// never fires inside "category". A phrase ending in punctuation, like the trailing
	// comma on "in summary,", is already bounded and takes no extra anchor.
	if endsWithWordChar(trimmed) {
		core += `\b`
	}
	re, err := regexp.Compile(core)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: phrase %q: %w", ErrCompile, phrase, err)
	}
	return Rule{Name: "phrase:" + strings.TrimSpace(phrase), re: re, replFunc: phraseSwap(repl), rewrite: true}, nil
}

// phraseSwap returns a replFunc that swaps a matched phrase for repl, carrying the
// match's capitalization onto it. A phrase opening a sentence keeps the opening capital,
// so "In order to ship" becomes "To ship" and not "to ship".
func phraseSwap(repl string) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		return matchCase(text[loc[0]:loc[1]], repl)
	}
}

// deletionRule builds a rule that cuts text and restores the sentence's opening capital.
// It eats the horizontal space after the match and captures the letter that follows, so
// the letter becomes a capital when the cut opened a sentence. It crosses a line break
// only when a word follows on the next line, so a cut never merges prose into a code
// fence or an indented block. Used for both stock-phrase openers and dropped words.
func deletionRule(name, text string) (Rule, error) {
	trimmed := strings.TrimRight(text, " ")
	core := `(?i)\b` + flexSpaces(regexp.QuoteMeta(trimmed))
	if endsWithWordChar(trimmed) {
		core += `\b`
	}
	re, err := regexp.Compile(core + `[ \t]*(?:\n[ \t]*(\p{L})|(\p{L})?)`)
	if err != nil {
		return Rule{}, fmt.Errorf("%w: %s: %w", ErrCompile, name, err)
	}
	return Rule{Name: name, re: re, replFunc: deleteWithRecap(re), rewrite: true}, nil
}

// deleteWithRecap returns a replFunc that drops a phrase match, keeping the letter
// captured after it. The letter turns into a capital when the phrase opened a sentence,
// so deleting "In summary, it works." leaves "It works." and not "it works.". The letter
// may sit on the next line, which the match pulled up.
func deleteWithRecap(re *regexp.Regexp) func(text string, loc []int) string {
	return func(text string, loc []int) string {
		start, end := recapLetter(re.FindStringSubmatchIndex(text[loc[0]:loc[1]]))
		if start < 0 {
			return ""
		}
		letter := text[loc[0]+start : loc[0]+end]
		if sentenceStart(text, loc[0]) {
			return strings.ToUpper(letter)
		}
		return letter
	}
}

// recapLetter returns the byte range of the recaptured letter within a submatch, taking
// whichever of the two capture groups matched, or -1, -1 when neither did.
func recapLetter(sub []int) (start, end int) {
	switch {
	case sub == nil:
		return -1, -1
	case sub[2] >= 0:
		return sub[2], sub[3]
	case len(sub) >= 6 && sub[4] >= 0:
		return sub[4], sub[5]
	default:
		return -1, -1
	}
}

// sentenceStart reports whether offset sits at the start of a sentence: at the start of
// the text, or after sentence-ending punctuation or a line break, with any spaces in
// between ignored. A period that closes an abbreviation or an ellipsis does not end a
// sentence, so "e.g., a hammer" is never read as a sentence opening on a comma.
func sentenceStart(text string, offset int) bool {
	i := offset - 1
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i < 0 {
		return true
	}
	switch text[i] {
	case '\n', '\r', '!', '?':
		return true
	case '.':
		return !abbreviationEnd(text, i)
	}
	return false
}

// abbreviations are dotted shortenings whose period ends the word rather than the
// sentence, lower-cased without their final dot. Dotted forms like "e.g." and "i.e."
// are recognized by their internal dot and need no entry.
//
//nolint:gochecknoglobals // Immutable lookup.
var abbreviations = map[string]bool{
	"etc": true, "vs": true, "cf": true, "ca": true, "al": true, "st": true,
	"no": true, "nos": true, "dr": true, "mr": true, "mrs": true, "ms": true,
	"prof": true, "jr": true, "sr": true, "rev": true, "hon": true, "gen": true,
	"sgt": true, "capt": true, "lt": true, "col": true, "inc": true, "ltd": true,
	"co": true, "corp": true, "dept": true, "est": true, "fig": true, "figs": true,
	"vol": true, "vols": true, "pp": true, "ed": true, "eds": true, "misc": true,
	"approx": true, "appt": true, "apt": true, "ave": true, "blvd": true, "rd": true,
	"ft": true, "oz": true, "lb": true, "lbs": true, "hr": true, "hrs": true,
	"sec": true, "min": true, "yr": true, "yrs": true, "mo": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true,
	"aug": true, "sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
	"mon": true, "tue": true, "tues": true, "wed": true, "thu": true, "thurs": true,
	"fri": true, "sat": true, "sun": true,
}

// abbreviationEnd reports whether the period at i closes an abbreviation, an initial, or
// an ellipsis rather than a sentence. The destructive cleanups key off sentence starts, so
// an ambiguous period is read as an abbreviation: leaving a comma in place is recoverable,
// while capitalizing mid-sentence is corruption.
func abbreviationEnd(text string, i int) bool {
	if i > 0 && text[i-1] == '.' {
		return true
	}
	j := i - 1
	for j >= 0 {
		c := text[j]
		if c == '.' || c == '_' || ('0' <= c && c <= '9') ||
			('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') {
			j--
			continue
		}
		break
	}
	token := text[j+1 : i]
	if token == "" {
		return false
	}
	if strings.Contains(token, ".") {
		return true
	}
	if len(token) == 1 && unicode.IsLetter(rune(token[0])) {
		return true
	}
	return abbreviations[strings.ToLower(token)]
}

// trimLeadingSpace returns the match without its leading spaces and tabs, leaving just
// the punctuation.
func trimLeadingSpace(text string, loc []int) string {
	return strings.TrimLeft(text[loc[0]:loc[1]], " \t")
}

// keepFinalByte rewrites a match to its final byte. It drops a comma run pressed
// against closing punctuation, the debris left when a word between them is cut.
func keepFinalByte(text string, loc []int) string {
	return text[loc[1]-1 : loc[1]]
}

// commaOpensSentence reports whether the comma at start begins a sentence, which marks
// it as debris left when an opening word was cut. A comma anywhere else is ordinary.
func commaOpensSentence(text string, start, _ int) bool {
	return sentenceStart(text, start)
}

// stripOrphanComma drops a sentence-opening comma and the spaces after it, keeping the
// next letter as a capital, so cutting an opener like "Seamlessly," leaves a clean start.
func stripOrphanComma(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	return string(unicode.ToUpper(r[len(r)-1]))
}

// notLineStart reports whether the match at start has text before it on the same line.
// It keeps indentation, like a markdown code block leading into a dot, out of reach of
// the punctuation cleanup.
func notLineStart(text string, start, _ int) bool {
	return start > 0 && text[start-1] != '\n' && text[start-1] != '\r'
}

// spaceBeforePunctKeep reports whether a space-before-punctuation match is real cleanup and
// not Markdown structure. It keeps indentation out of reach like notLineStart, and skips the
// "!" that opens an inline image, where the space belongs before the image.
func spaceBeforePunctKeep(text string, start, end int) bool {
	if !notLineStart(text, start, end) {
		return false
	}
	return text[end-1] != '!' || end >= len(text) || text[end] != '['
}

// articleRe matches an "a" or "an" article and the word that follows, so the article can be
// corrected to the sound of that word.
//
//nolint:gochecknoglobals // Compiled once, never modified.
var articleRe = regexp.MustCompile(`\b([Aa]n?)\b([ \t]+)([A-Za-z][A-Za-z.'-]*)`)

// silentH lists words whose leading h is silent, so they take "an" despite the consonant.
//
//nolint:gochecknoglobals // Immutable lookup.
var silentH = []string{"honest", "honor", "honour", "hour", "heir"}

// consonantVowel lists vowel-spelled prefixes that open on a consonant sound, the "you" of
// "user" and the "wun" of "one", so they take "a" despite the leading vowel.
//
//nolint:gochecknoglobals // Immutable lookup.
var consonantVowel = []string{
	"use", "user", "usu", "uni", "unit", "uniqu", "unif", "unio", "util",
	"euro", "eu", "ubiq", "ukulele", "one", "once", "ewe",
}

// fixArticle rewrites an "a"/"an" match so the article matches the sound of the next word,
// keeping the article's capitalization and the original spacing.
func fixArticle(text string, loc []int) string {
	m := articleRe.FindStringSubmatch(text[loc[0]:loc[1]])
	if m == nil {
		return text[loc[0]:loc[1]]
	}
	article, gap, word := m[1], m[2], m[3]
	corrected := "a"
	if startsWithVowelSound(word) {
		corrected = "an"
	}
	if article[0] == 'A' {
		corrected = "A" + corrected[1:]
	}
	return corrected + gap + word
}

// notProperNoun reports whether a match should be acted on rather than skipped as a likely
// proper noun. A Title-case word mid-sentence, like a brand name, is skipped, while the same
// word at a sentence start is ordinary capitalization and still counts, unless the document
// itself proves the word is a name by using it Title-cased mid-sentence somewhere else. A
// lower-case or an all-caps match always counts.
func notProperNoun(text string, start, end int) bool {
	match := text[start:end]
	r := []rune(match)
	if len(r) == 0 || !unicode.IsUpper(r[0]) {
		return true
	}
	if match == strings.ToUpper(match) {
		return true
	}
	if !sentenceStart(text, start) {
		return false
	}
	return !titleCaseMidSentence(text, match, start)
}

// titleCaseMidSentence reports whether match, kept in its exact casing, appears as a whole
// word at a non-sentence-start position anywhere else in text. One such occurrence proves
// the word is a proper noun in this document, so its sentence-start occurrences are names
// too, like "Delve is a debugger" in a document that later says "attach Delve".
func titleCaseMidSentence(text, match string, self int) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], match)
		if j < 0 {
			return false
		}
		pos := i + j
		i = pos + 1
		if pos == self {
			continue
		}
		if pos > 0 && isWordByte(text[pos-1]) {
			continue
		}
		if e := pos + len(match); e < len(text) && isWordByte(text[e]) {
			continue
		}
		if !sentenceStart(text, pos) {
			return true
		}
	}
}

// isWordByte reports whether c is an ASCII word character, the set the \b boundary
// recognizes.
func isWordByte(c byte) bool {
	return c == '_' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// articleNeedsFix reports whether the article in the match disagrees with the sound of the
// word that follows, so the rule fires, and reports a finding, only when a correction is
// actually needed and never on an already-correct "a" or "an".
func articleNeedsFix(text string, start, end int) bool {
	m := articleRe.FindStringSubmatch(text[start:end])
	if m == nil {
		return false
	}
	return startsWithVowelSound(m[3]) != (len(m[1]) == 2)
}

// startsWithVowelSound reports whether word begins with a vowel sound, which decides between
// "a" and "an". It handles the common exceptions: silent-h words take "an", "you"-sound and
// "one"-sound words take "a" despite a leading vowel, and an all-caps acronym opening on a
// letter whose name starts with a vowel (A, E, F, H, I, L, M, N, O, R, S, X) takes "an".
func startsWithVowelSound(word string) bool {
	lw := strings.ToLower(strings.Trim(word, "'"))
	if lw == "" {
		return false
	}
	if word == strings.ToUpper(word) && word != lw && len(word) > 1 {
		return strings.ContainsRune("AEFHILMNORSX", rune(word[0]))
	}
	for _, p := range silentH {
		if strings.HasPrefix(lw, p) {
			return true
		}
	}
	for _, p := range consonantVowel {
		if strings.HasPrefix(lw, p) {
			return false
		}
	}
	switch lw[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// collapsibleRun reports whether a run of spaces should collapse. A run at the start of
// a line is indentation, a run that reaches the end of a line can be a markdown hard
// break, and a run on a table row is alignment padding, so all three stay.
func collapsibleRun(text string, start, end int) bool {
	if !notLineStart(text, start, end) || inTableRow(text, start) {
		return false
	}
	return end < len(text) && text[end] != '\n' && text[end] != '\r'
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
// the next word. When the clause already ends in sentence punctuation, the semicolon is
// dropped without adding a second period, so "2.; the" does not become "2.. The".
func splitSemicolon(text string, loc []int) string {
	r := []rune(text[loc[0]:loc[1]])
	last := string(unicode.ToUpper(r[len(r)-1]))
	if loc[0] > 0 {
		switch text[loc[0]-1] {
		case '.', '!', '?':
			return " " + last
		}
	}
	return ". " + last
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
	if inTableRow(text, semi) || inParens(text[start:semi]) {
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

// inParens reports whether prefix, the text from the sentence start up to a semicolon,
// leaves a parenthesis open, which means the semicolon sits inside a parenthetical and is
// almost always a list separator rather than a clause join.
func inParens(prefix string) bool {
	return strings.Count(prefix, "(") > strings.Count(prefix, ")")
}

// sentenceBounds returns the byte range of the sentence around offset, bounded by
// sentence-ending punctuation or a block break. A newline that is only a soft line wrap
// does not end the sentence, so a hard-wrapped sentence keeps its full extent and the
// semicolon list guard sees every semicolon it holds, not just the ones on one line. A
// period that closes an abbreviation does not end the sentence either.
func sentenceBounds(text string, offset int) (start, end int) {
	for i := offset - 1; i >= 0; i-- {
		if sentenceBoundary(text, i) {
			start = i + 1
			break
		}
	}
	end = len(text)
	for i := offset + 1; i < len(text); i++ {
		if sentenceBoundary(text, i) {
			end = i
			break
		}
	}
	return start, end
}

// sentenceBoundary reports whether the byte at i ends a sentence: sentence punctuation
// that is not an abbreviation, or a newline that breaks a block rather than wrapping a
// line.
func sentenceBoundary(text string, i int) bool {
	switch text[i] {
	case '!', '?':
		return true
	case '.':
		return !abbreviationEnd(text, i)
	case '\n':
		return !softWrap(text, i)
	}
	return false
}
