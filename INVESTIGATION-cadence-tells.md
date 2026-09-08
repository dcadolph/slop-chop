# Investigation: cadence and landing tells

Opened 2026-09-07. Design record in the style of slop-chop-audit.md.
Everything about the current engine below is read from the source, not assumed.

## What set this off

A live services page carried this sentence:

> It is usually about a third, and that number is the rest of the day.

A reader flagged it as obviously machine-written. It is clean ASCII, has no banned word,
no em-dash, no semicolon, and no stock opener, so the lexical rules pass through it. The
tell is the shape: a concrete noun equated to an abstraction with a bare "is", delivered
as a short closing landing. The engine does not catch it. This investigation scopes
whether it should, and what else in the same family is missing.

## What the engine already does (corrected from a wrong first assumption)

The first assumption was that slop-chop is purely lexical. That is wrong. It already runs
a structural track alongside the regex rules, and this investigation should build on it,
not around it.

Existing structural detection:

- Repeated-opener drumbeat: `anaphoraFindings` flags three or more consecutive short
  same-paragraph sentences that open with the same two words (`sanitize/anaphora.go`).
- Uniform paragraph templating: `uniformParagraphs` and `templateStems` in
  `sanitize/shape.go`, emitting `structural:uniform-paragraphs` and
  `structural:template-stem`.
- A large family of negation reframes already in `DefaultProfile.FlagPatterns`
  (`sanitize/profile.go` ~214-359): `its-not-x-its-y`, `not-just-but-also`,
  `not-only-inversion`, `reversal-aphorism` ("Tools don't fail teams. Blind spots do."),
  `until-it-didnt`, and more. All require an explicit `not`/`n't` trigger.
- A bare-fragment tricolon: `triad-fragment` catches "Simple, boring, reliable." with an
  epizeuxis guard.
- Cadence variance is measured. `CadenceCV`, the coefficient of variation of sentence
  length, is computed in `sanitize/score.go` (~277-304) and reported, but it carries zero
  score weight, on the deliberate reasoning that modern models vary sentence length on
  purpose and penalizing low variance would be noise.

So the engine has the machinery, the sentence-span primitives, the scoring hook, and the
`structural:*` finding convention. The gap is specific tells, not a missing capability.

## The gap

None of these are detected today:

1. Copula-abstraction landing. A short sentence, often closing a paragraph, of the shape
   NOUN PHRASE + is/are + ABSTRACTION, delivered as a payoff. "That number is the rest of
   the day." "The spread is the point." "Reach is the constraint." This is the flagged
   case and the highest-value target.
2. "X rather than Y" and "X instead of Y" corrective constructions. The negation family is
   well covered, but these non-negated cousins have no pattern.
3. Colon-drumroll reveal: a clause, a colon, then a short payoff phrase. The existing
   reveal patterns are question-mark or appositive based, not colon based.
4. "And"-joined tricolon: "green, lint-clean, and CI-passing." Only the bare-fragment form
   is caught today.
5. Polyptoton: a stem turned against itself inside one sentence, "verification that does
   not verify," "measures rather than asserts."
6. Uniform punchiness as a scored signal. Distinct from CadenceCV: not just low length
   variance, but a run of sentences that each land. Currently unmeasured as a penalty.

## Proposed design

The engine gives two plug-in paths. Use each where it fits; do not force one tell into the
wrong path.

### Regex-tractable tells go in FlagPatterns (data, no code)

Items 2, 3, and 4 are surface-shaped and belong as `FlagPatterns` entries in
`DefaultProfile` (`sanitize/profile.go`), compiled to flag-only `structural:*` rules at
`sanitize/compile.go` (~127). Flag-only means `rewrite: false`; the reword is the model
pass's job. Sketches, to be tuned against the corpus:

- `rather-than-reframe`: a comma-free clause, then ` rather than ` or ` instead of `, then
  a short tail. Guard against legitimate comparative usage ("chose Postgres rather than
  MySQL" is fine; the tell is abstract-versus-abstract).
- `colon-reveal`: sentence-internal `: ` followed by a payoff of roughly two to six words
  ending the sentence. High false-positive risk against ordinary lists and code; needs a
  `keep` guard using `structuralRanges` and a length/word-class check.
- `tricolon-and`: three comma-separated adjectives or short noun phrases joined by a final
  "and", as a full clause. Reuse the `triadVaries` epizeuxis guard from `compile.go`.

Each regex-only tell that needs a context check registers a guard in `flagPatternKeeps`
(`sanitize/compile.go` ~332), signature `func(text string, start, end int) bool`.

### Semantic tells need a Go walker (Path B)

Items 1, 5, and 6 cannot be a single regex. They need sentence walking and a light
word-class judgment, so they follow the `anaphoraFindings` / `shapeFindings` template:
a function `func(text string, protected [][2]int) []Finding` appended in
`Sanitizer.Check` (`sanitize/sanitize.go` ~116), emitting `structural:*` findings with the
`anaphoraOrder` sentinel so they sort after compiled rules.

- `structural:copula-landing`. Walk sentences with `sentenceSpans`. Fire when a sentence
  is short (say under about twelve words), its main verb is a copula (is/are/was/were),
  the subject is concrete or a demonstrative ("that number", "the spread"), and the
  complement is abstract. The abstract-complement test is the hard part. A pragmatic first
  cut: a small closed list of abstraction nouns plus a demonstrative-subject trigger, which
  catches "that number is the rest of the day" and "the spread is the point" without a POS
  tagger. Flag-only; the model rewrites. Bias toward precision over recall; a missed
  landing is cheaper than a flagged honest sentence.
- `structural:polyptoton`. Within a sentence, stem each word (a crude suffix stripper is
  enough for a first cut) and fire when one stem appears twice in different surface forms,
  excluding stopwords and exact repetition (which anaphora and `identicalRun` already
  handle). Guard technical terms via the existing collocation allow-list machinery.
- Uniform punchiness. Do not ship as a hard finding first. Add it as a reported score
  signal beside CadenceCV, watch it on the corpus, and only give it weight if the
  benchmark separation margin holds. The existing choice to leave CadenceCV unweighted is
  the right prior and this should clear the same bar before it earns a penalty.

### Scoring

No new wiring needed. Any finding whose `Rule` begins with `structural:` is picked up by
`weightTells` at weight 2 (`sanitize/score.go` ~229), with overlap dedup and repeat
halving already applied. A profile author can retune via `scoreWeights["structural"]` or an
exact-name key. The copula-landing walker should confirm it does not double-count against
`reversal-aphorism` on the same span; the overlap dedup at `score.go` ~176 should handle it,
but the corpus test must prove it.

## The real risk

Structural detection false-positives hurt more than lexical ones, because they fire on
sentence shapes that good human writing also uses. A copywriter lands one sentence per
page on purpose. The goal is not zero landings, it is catching the machine rate of one per
paragraph. Two consequences:

- Every new detector ships flag-only and precision-first. Recall is secondary.
- The benchmark is the gate, not intuition. `internal/sanitize/bench_test.go` against
  `testdata/corpus.jsonl` must keep its separation margin. Each tell needs positive
  examples (the flagged sentence and its cousins) and hard negatives (honest sentences of
  the same surface shape) added to the corpus before the detector is weighted.

## Suggested order

1. Corpus first. Add positives and hard negatives for all six tells to `corpus.jsonl` so
   every later step is measured.
2. `rather-than-reframe` and `tricolon-and` as `FlagPatterns`. Cheapest, lowest risk,
   extends a family that already works.
3. `structural:copula-landing` walker with the demonstrative-plus-abstraction-list first
   cut. This is the one that motivated the investigation.
4. `colon-reveal` `FlagPattern`, only if its hard negatives stay clean; it is the most
   false-positive-prone.
5. `structural:polyptoton` walker.
6. Revisit uniform-punchiness as a reported signal, weight it only if the margin holds.

## Note on TELLS.md

TELLS.md is the published catalog. Anything shipped from this investigation gets an entry
there in the same voice, so the public tell list stays in step with what the engine
actually detects.

## Outcome, measured 2026-09-07

Built in the order above. The corpus went first, so every decision below is a reading off
`sanitize/testdata/corpus.jsonl` rather than a judgment about how a sentence feels. Corpus
grew from 58 ai / 35 human / 22 technical to 66 / 43 / 36.

Benchmark, before and after:

| Metric | Before | After | Floor |
| ------ | ------ | ----- | ----- |
| Tell recall | 0.98 (57/58) | 0.98 (65/66) | 0.95 |
| Technical precision | 1.00 (22/22) | 1.00 (36/36) | 0.98 |
| Score recall | 0.98 (57/58) | 0.98 (65/66) | 0.95 |
| Score precision | 0.98 | 0.98 | 0.98 |
| Score margin | 75.9 | 76.5 | 70 |

### Shipped

- `structural:copula-landing`, a walker in `sanitize/landing.go`. Fires when the closing
  clause of a sentence puts a concrete or demonstrative subject against a bare copula and
  an abstraction from a short closed list, with nothing after the noun but an "of" tail.
  Both halves are required, which is what keeps `That column is the primary key` and
  `The problem is the cost` out. Catches the sentence that opened this investigation.
- `structural:polyptoton`, a walker in `sanitize/polyptoton.go`. Fires on a stem turned
  against itself with a pivot and a negation standing between the two forms. The turn is
  the tell, not the repetition, which is what leaves `the parser parses` and
  `The scheduler does not schedule jobs` alone. The crude prefix stemmer the doc suggested
  did not work: it cannot separate `measurement` and `measuring` from `release` and
  `relearn`, since the false pair shares more of its shorter word. A one-suffix stripper
  plus a trailing "e" drop does separate them, and that is what shipped.
- Uniform punchiness, reported on `Score.Punchiness` and given no weight. See below.

### Rejected on the corpus

Three of the six could not be separated from honest prose at the surface. Each was built,
measured, and dropped, and the hard negatives stay in the corpus so a future looser rule
runs into them.

- `rather-than-reframe`. Every candidate caught one honest technical sentence for every
  two machine ones. `A missing field is a warning rather than an error` and
  `It is a discipline rather than a habit` are the same shape; the difference is whether
  the two nouns are abstract, which needs a second closed word list built only from the
  positives. The parallel third-person form was worse: it caught
  `The scheduler retries rather than fails` and none of the positives.
- `tricolon-and`. The plain sentence-final and-tricolon hit 3 ai and 4 technical passages.
  Requiring a coined hyphenated compound narrowed it to 2 ai and 3 technical, and
  requiring two compounds to 2 ai and 1 technical, still catching
  `The keys are rotated, read-only, and write-once`. The and-joined triad is the ordinary
  English list, which is the reason `triad-fragment` only takes the bare form, and this
  measurement confirms that prior rather than overturning it.
- `colon-reveal`. Caught 5 honest sentences for 2 real drumrolls, including
  `The rule was simple: never trust the client` and `Run it in two steps: build, then push`.
  Worse, every ai passage it did catch was already caught by a lexical rule on the lead-in
  phrase, so it added a large false-positive surface and no unique recall. The machine
  quality of the colon drumroll lives in the phrase before the colon, and those are
  already covered.

### Uniform punchiness stays unweighted

Reported as the share of sentences sitting in a run of three or more short ones, which is
a different reading from `CadenceCV`: one measures spread, the other measures runs. It
separates directionally, 0.47 mean on ai against 0.10 on human. It does not earn weight.
Trialed at four weights on the corpus: at 10 and 15 points it buys zero additional recall
and only pads passages already over the line, and 25 is the first weight that catches the
one flat-cadence passage the engine still misses. At exactly that weight it also flips a
human passage over the threshold, the terse plumber anecdote, taking score precision from
0.98 to 0.97 and under its floor. Terse is a voice, so the signal is reported and left at
zero weight, which is where `CadenceCV` already sits for the same reason.
