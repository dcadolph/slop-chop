package sanitize

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	// BlockAlways are words flagged wherever and however they appear, with no
	// proper-noun exemption. A personal avoid list lands here: when you ban your own
	// word, a capitalized use is still the word you banned, not a brand to spare.
	BlockAlways []string `json:"blockAlways"`
	// ScoreWeights tunes how much a finding adds to the score's tell density. A key is
	// an exact rule name, like "char:—" or "word:robust", or a rule class, the part
	// before the colon, like "word", "phrase", "structural", or "char"; cleanup rules
	// with no colon share the class "tidy". An exact name wins over its class, and both
	// win over the built-in defaults: structural tells count 2, typography swaps 0,
	// cleanups 0, and everything else 1. Findings themselves are unaffected; only the
	// score moves.
	ScoreWeights map[string]float64 `json:"scoreWeights"`
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
	// Standalone makes this profile replace the built-in default outright instead of
	// extending it. Off by default, because the policy every team actually wants is
	// the default detection plus their own entries, and a two-line profile that
	// silently disabled two hundred rules certified slop through CI.
	Standalone bool `json:"standalone"`
}

// DefaultProfile returns the built-in profile that targets common AI tells.
func DefaultProfile() Profile {
	p := Profile{
		CharReplace: map[string]string{
			"\u00a0":   " ",   // non-breaking space to a normal space
			"\u202f":   " ",   // narrow non-breaking space to a normal space
			"\u200b":   "",    // zero-width space, usually paste cruft
			"\u2060":   "",    // word joiner, usually paste cruft
			"\ufeff":   "",    // zero-width no-break space or a stray byte-order mark
			"\u00ad":   "",    // soft hyphen, invisible and a word-smuggling channel
			"\u2010":   "-",   // unicode hyphen to the ASCII one
			"\u2011":   "-",   // non-breaking hyphen to the ASCII one
			"\u2012":   "-",   // figure dash to a hyphen
			"\u2015":   ", ",  // horizontal bar doing an em-dash's job
			"\u200c":   "",    // zero-width non-joiner smuggled between Latin letters
			"\u200d":   "",    // zero-width joiner smuggled between Latin letters
			"\ufe58":   ", ",  // small em dash doing an em-dash's job
			"\u2e3a":   ", ",  // two-em dash doing an em-dash's job
			"－":        "-",   // fullwidth hyphen-minus to the ASCII one
			"ſ":        "s",   // long s, a Latin homoglyph that dodges the word lists
			"&mdash;":  ", ",  // an em-dash that arrived as a raw HTML entity
			"&ndash;":  "-",   // an en-dash that arrived as a raw HTML entity
			"&hellip;": "...", // an ellipsis that arrived as a raw HTML entity
			"&#8212;":  ", ",  // the numeric em-dash entity
			"&#8211;":  "-",   // the numeric en-dash entity
			"—":        ", ",  // em-dash
			"–":        "-",   // en-dash
			"‘":        "'",   // left single quote
			"’":        "'",   // right single quote
			"“":        `"`,   // left double quote
			"”":        `"`,   // right double quote
			"…":        "...", // ellipsis
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
			"that said, ":                       "",
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
		WordReplace: map[string]string{
			// The essay-skeleton ordinals have one mechanical fix: the bare ordinal reads
			// the same and drops the -ly flourish, so these rewrite instead of only
			// flagging.
			"firstly":  "first",
			"secondly": "second",
			"thirdly":  "third",
			"fourthly": "fourth",
			"lastly":   "last",
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
			"its-not-x-its-y": `(?i)\b(?:it|this|that)(?:'?s|\s+(?:is|was|are|were))(?:\s+not|n'?t)\b[^.!?\n]{1,40}(?:[,;.]\s*|\s*[—–]\s*)(?:it'?s|that'?s|this is)\b`,
			// "not just X but also Y" and "not only X but also Y".
			"not-just-but-also": `(?i)\bnot (just|only)\b[^.!?\n]{1,60}\bbut\b[^.!?\n]{0,25}\balso\b`,
			// Throat-clearing openers that promise a payoff.
			"heres-the-thing": `(?i)\bhere'?s the (thing|kicker|deal|catch|secret|problem)\b`,
			// The "let's dive in" invitation and its "let's take a closer look" cousins.
			"lets-dive-in":     `(?i)\blet'?s (dive|delve|jump) in(to)?\b`,
			"lets-take-a-look": `(?i)\blet'?s (?:take a (?:closer )?look|explore|unpack|break (?:it|this) down)\b`,
			// Chatbot reply openers, anchored to a line start where an opener lives.
			"assistant-opener": `(?im)^\s{0,3}(?:certainly|absolutely|great question|great (?:point|catch|call)|good (?:point|catch|question)|that'?s a (?:great|good|fair|valid) (?:point|question|concern)|thanks for (?:flagging|raising|catching|sharing)|i'?d be happy to|happy to help)\b`,
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
			"bold-bullet-run": `(?m)(?:^[ \t]*[-*][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*.*\n(?:[ \t]*\n)*){2}[ \t]*[-*][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*`,
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
			"spaced-hyphen": `\p{Ll} -- \p{Ll}`,
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
			"contracted-not-just": `(?i)\b(?:is|are|was|were|do|does|did|has|have)n'?t (?:just |only |merely |simply )?(?:a|an|the|your|about)\b[^.!?\n]{1,60}(?:[.,]\s*|\s*[—–]\s*)(?:it'?s|they'?re|you'?re|we'?re|that'?s|this is)\b`,
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
			"bold-number-run": `(?m)(?:^[ \t]*\d+[.)][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*.*\n(?:[ \t]*\n)*){2}[ \t]*\d+[.)][ \t]+\*\*[\p{L}\p{N}][^*\n]{0,59}\*\*`,
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
			// The pasted-assistant-answer artifacts: a document that talks about its own
			// delivery the way a chat reply does.
			"below-is-requested": `(?im)^(?:\w+[.!] )?below (?:is|are) (?:the|a|an|your) (?:requested|updated|revised|complete|full|final)\b`,
			"ive-structured":     `(?i)\bi'?ve (?:structured|organized|formatted|arranged) (?:this|the|it|each)\b`,
			"would-you-like":     `(?i)\bwould you like me to\b`,
			"let-me-know-if":     `(?i)\blet me know if you(?:'?d like| want| need)\b`,
			// The AI pull-request register.
			"pr-introduces":   `(?im)^this (?:pr|pull request|mr|commit|change|patch) (?:introduces|implements|adds|enhances)\b`,
			"key-changes":     `(?i)\bkey (?:changes|updates|highlights|features) include\b`,
			"backward-compat": `(?i)\bwhile maintaining (?:full )?backwards? compatibility\b`,
			"no-breaking":     `(?i)\bno breaking changes (?:are|were) introduced\b`,
			"future-could":    `(?i)\bfuture (?:enhancements|improvements) could include\b`,
			// The outline-driven listicle skeleton.
			"in-this-section": `(?im)^in this section,? we\b`,
			"is-practice-of":  `(?i)\bis the (?:practice|process|technique|art) of\b`,
			// The balanced 2026 register: restating the question, and verdicts that
			// commit to nothing.
			"question-restate": `(?im)^you asked (?:whether|about|if|for)\b[^.\n]{5,80}, and\b`,
			"no-commitment":    `(?i)\b(?:depends? on (?:several|many|a number of) factors|the right choice will vary|a defensible choice|will vary based on your)\b`,
			// An emoji closing a heading, the decorated-section look.
			"emoji-heading-suffix": `(?m)^#{1,6}[ \t].{0,60}[\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{1F000}-\x{1FAFF}][ \t]*$`,
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
			"it's not you, it's me",
			"foster care", "foster child", "foster children", "foster family",
			"foster home", "foster parent", "foster parents",
		},
		CollapseSpaces:  true,
		SplitSemicolons: true,
		FixArticles:     true,
	}
	// Fullwidth Latin letters fold to ASCII so a tell spelled in fullwidth forms cannot
	// slip past the word lists. Only letters fold: fullwidth punctuation is ordinary CJK
	// typesetting and stays.
	for r := 'A'; r <= 'Z'; r++ {
		p.CharReplace[string(r+0xFEE0)] = string(r)
	}
	for r := 'a'; r <= 'z'; r++ {
		p.CharReplace[string(r+0xFEE0)] = string(r)
	}
	return p
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
