# Investigation: what the long-form misses carry

Opened 2026-10-08. Design record in the style of INVESTIGATION-cadence-tells.md. Every number
below was measured on the development half of the long-form corpus first, then reported on
the holdout half and the pre-2022 false positive corpora without adjustment. The temporary
test that produced them was deleted once the signal shipped. This file is the record.

## What set this off

The long-form corpus scored 41 of 65 machine passages at or over the reads-clean line and 1
of 80 human. As a ranking the engine was fine, at a Cohen's d of 2.16. As a verdict it
missed more than a third of full-length machine prose. On the development half 17 of 41
machine passages sat under the line, and 13 of those came from the adversarial prompt, the
one that names the cliches and tells the model to avoid them. They carried no lexical tells
to speak of. Cadence fired on most of them at four to ten points, and cadence is capped so
that it cannot carry a verdict alone, which is correct and is also why they stayed under.

Three scored zero. One was a release note that opens `We are thrilled to announce` and runs
on `significant update`, `designed to optimize`, `enhance overall system security`, and
`ensures that your critical data is saved more effectively`. Every one of those lives in the
`marketing` preset, which is not the default, so the default profile read the most obvious
machine prose in the corpus as clean. The other two were adversarial-prompt READMEs that
read like competent human instructions, and the engine was right to be nearly quiet.

## What the engine already reads

The first draft of this plan named three candidates from the owner's own diction notes:
copula landings, "not X but Y", and uniform punchiness. All three were already in the
engine. `landing.go` flags the copula-abstraction landing, `drumbeat.go` scores the bare
"That's X." snap by rate, `punchiness` is measured and deliberately unweighted because terse
is a voice, and the negation reframes are a family of regex rules in the profile. The gap
was not those. The gap had to be found by measuring.

## What was measured

Fifty features, each a rate or a share over one passage, computed on the development half
(41 machine, 42 human) and reported as Cohen's d, then recomputed on the holdout half (24
machine, 38 human). Both halves are the engine's own long-form corpus, flattened to one
paragraph per passage by the collection filter.

| Feature | Machine | Human | Dev d | Holdout d |
| --- | --- | --- | --- | --- |
| Adjacent-sentence length change, over mean | 0.40 | 0.71 | -2.08 | -1.64 |
| Share of sentences with no comma | 0.25 | 0.55 | -2.02 | -2.27 |
| Cadence coefficient of variation | 0.35 | 0.72 | -1.95 | -1.56 |
| Share of sentences at ten words or under | 0.10 | 0.33 | -1.77 | -1.41 |
| Share of sentences at six words or under | 0.02 | 0.16 | -1.49 | -1.00 |
| Mid-sentence capitalized words per 100 | 1.6 | 5.5 | -1.38 | -0.91 |
| `ensure` forms per 100 words | 0.37 | 0.02 | 1.28 | 1.71 |
| Parentheses per 100 words | 0.07 | 1.04 | -1.26 | -1.33 |
| Words of ten letters or more, share | 0.10 | 0.07 | 1.20 | 1.67 |
| Nominalizations per 100 words | 5.4 | 3.0 | 1.15 | 2.10 |
| Punchiness (runs of short sentences) | 0.00 | 0.09 | -0.91 | -0.65 |
| Sentence-initial transition words per 100 | 0.38 | 0.07 | 0.86 | 1.08 |
| Commas per sentence | 1.22 | 0.87 | 0.80 | 1.27 |
| "if you" per 100 words | 0.12 | 0.37 | -0.72 | -1.00 |
| Digits per 100 words | 0.78 | 2.28 | -0.72 | -0.75 |
| First person singular per 100 words | 0.04 | 0.56 | -0.64 | -0.56 |
| Colons mid-line per 100 words | 0.46 | 0.96 | -0.60 | -0.83 |
| "X, not Y" sentence endings per 100 | 0.00 | 0.01 | -0.38 | 0.11 |
| "rather than" per 100 words | 0.02 | 0.00 | 0.32 | 0.68 |
| Paragraph-level shape (any of five) | 1 paragraph | 1 paragraph | 0 | 0 |

Everything under 0.6 on both halves is left out of the table and was left out of the work.

Four things came out of this.

The rhythm family is one signal measured five ways. Adjacent change, cadence variation,
short-sentence share, sentence-length spread, and punchiness all read the same flatness,
and cadence already carries it. Adding a second rhythm measure would charge the same prose
twice.

Half of the strongest features are absences. Human READMEs name things, cite numbers,
use parentheses, say "I", and write "if you". Machine prose does none of that. Scoring an
absence punishes every human who writes plain prose with none of them, and the exemplar
corpus holds poetry, a graduation speech, and Moby-Dick for exactly that reason. None of
the absences went forward.

"X, not Y" and "rather than" do not appear in small-model prose. The corrective reframe the
owner's notes list as a tell is a frontier-model habit, and this corpus has no frontier
model in it. It cannot be measured here.

Paragraph shape cannot be measured here either. The collection filter joins every passage
into one paragraph, on both halves, so the five paragraph features read zero on every
passage. The `shape.go` walkers are unmeasured on this corpus for the same reason.

That left three candidates with real separation on both halves that the engine did not
already read: the comma-bearing share, nominalization density, and the word `ensure`.

## Distributions, since precision is the claim

| | Machine p50 | Human p50 | Human p90 | Human p99 | Pre-2022 p90 | Pre-2022 p99 |
| --- | --- | --- | --- | --- | --- | --- |
| Comma-bearing share, dev | 0.75 | 0.50 | 0.68 | 0.84 | 0.56 | 0.75 |
| Comma-bearing share, holdout | 0.82 | 0.46 | 0.67 | 0.85 | | |
| Nominalizations per 100, dev | 6.1 | 3.8 | 6.1 | 8.6 | 6.1 | 9.1 |
| Nominalizations per 100, holdout | 8.0 | 3.4 | 5.9 | 7.9 | | |

The pre-2022 columns are the 618 abandoned-repository READMEs, 590 of which hold six or
more prose sentences. `ensure` is present in 66 percent of development machine passages and
5 percent of human ones, 62 and 3 percent on the holdout.

Pearson correlation on the pooled development half: comma share against cadence variation
-0.50, comma share against nominalizations 0.58, nominalizations against cadence -0.35.

## The verdict simulation

Each candidate was added to the existing score as a component and the verdict counts
recomputed, with the pre-2022 corpus scored under each configuration. "Touched" is how many
human READMEs gained any points at all.

| Configuration | Dev machine | Dev human | Holdout machine | Holdout human | Pre-2022 at 25 | Touched | AUC dev | AUC holdout |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Baseline | 24/41 | 1/42 | 17/24 | 0/38 | 4/618 | 0 | 0.878 | 0.957 |
| Comma share, 0.70 threshold, cap 8 | 24/41 | 1/42 | 19/24 | 0/38 | 4/618 | 10 | 0.898 | 0.957 |
| Comma share, 0.70 threshold, cap 10 | 24/41 | 1/42 | 19/24 | 0/38 | 5/618 | 10 | 0.899 | 0.956 |
| Nominalizations, 5 to 9 per 100, cap 6 | 24/41 | 1/42 | 19/24 | 0/38 | 4/618 | 109 | 0.888 | 0.957 |
| `ensure` as a word tell | 26/41 | 1/42 | 20/24 | 0/38 | 4/618 | 39 | 0.918 | 0.990 |
| Comma share and `ensure` | 27/41 | 1/42 | 20/24 | 0/38 | 4/618 | 49 | 0.924 | 0.989 |
| All three | 27/41 | 1/42 | 20/24 | 0/38 | 4/618 | 153 | 0.924 | 0.992 |

The comma share touches three human passages on each half. Nominalizations touch eight on
each half and 109 of the 618 READMEs, and they move the same two holdout passages the comma
share moves, which is what a 0.58 correlation looks like in verdicts. The substitution
attack removes none of the comma share and five percent of the nominalization points.

The six machine passages the full configuration carries over the line are four adversarial
or plain README, postmortem, guide, and essay passages between 16 and 24, and two styled
ones at 23. Four of the six carry `ensure` more than once.

## What was chosen

**The clause habit**, `sanitize/clauses.go`. The share of prose sentences carrying a comma,
read over sentences of five words or more outside code, tables, headings, and block quotes.
Nothing under six prose sentences. Nothing until the share clears 0.70, which is the human
ninetieth percentile on the development half. A gradient from there to eight points at a
comma in every sentence, so a human passage at the ninety-ninth percentile gains under four.
The measurement skipped sentences opening on a list marker. The shipped rule counts a list
item that is a sentence, because `sentenceSpans` splits a numbered item off its own marker
and the word floor already drops the stubs. The pre-2022 figures below are from the shipped
rule.

**`ensure`, `ensured`, `ensures`, `ensuring`** as block words. Flag only, weight one, like
every other block word. It is the evadable kind, and it is the widest single-word gap
measured on this corpus.

**`announce-flourish`**, a flag pattern for `thrilled to announce` and its cousins. Zero of
852 pre-2022 READMEs, zero human long-form passages, three machine ones, and the release
note that scored zero.

**`chat-signoff` widened** to `happy` plus any verb plus `ing!`. Four machine long-form
passages close that way and no human one. Two of 852 pre-2022 READMEs do, `happy coding!`
and `Happy reading!`, and the first of those was already in the rule.

## What was rejected, and why

Nominalization density, for the reasons the simulation shows: the same passages as the
comma share, no ranking gain beyond it, and eighteen percent of human READMEs touched,
because technical prose is made of those nouns.

"designed to", which runs 23 of 65 machine long-form passages against 2 of 80 human, but
appears in 25 of 852 pre-2022 READMEs and is ordinary English.

Every absence feature, on principle, as above.

## Result

Shipped, measured by the tests that now gate it.

| | Before | After |
| --- | --- | --- |
| Long-form machine at the line, all | 41/65 | 48/65 |
| Long-form machine at the line, dev | 24/41 | 28/41 |
| Long-form machine at the line, holdout | 17/24 | 20/24 |
| Long-form human at the line | 1/80 | 1/80 |
| Long-form score, Cohen's d | 2.16 | 2.66 |
| Long-form mean, machine / human | 30.1 / 5.1 | 38.1 / 5.4 |
| Clause points the attack removes | | 1 of 170 |
| Pre-2022 abandoned at the line | 4/618 | 4/618 |
| Pre-2022 maintained at the line | 4/268 | 4/268 |
| `ensure` firing, abandoned / maintained | | 5% / 13% |
| Exemplar corpus, machine / human / technical | 68 / 45 / 36 | 71 / 46 / 36 |
| Attack, word tells evaded / held | 33 / 38 | 33 / 41 |
| Attack, structural tells evaded / held | 1 / 88 | 1 / 89 |

The maintained-corpus count was recorded as three in BENCHMARK.md and measured four on the
engine before this change, since the participle tail signal of 2026-10-05 had moved one
document from 24 to 25 and the recheck that day ran the abandoned corpus only. Both corpora
were scored on the engine before and after in a scratch worktree. No document moved.

The development gain is larger than the simulation's 27, because the simulation modeled
`ensure` as a plain weight and the shipped word rule halves a repeated occurrence, which
interacts with the evidence cap differently, and because the sign-off rule was not
simulated at all.

## What this does not say

Nothing here is measured on frontier prose. Every machine passage in the long-form corpus
comes from a small local model. The clause habit is the kind of thing a frontier model does
more of, not less, but that is a belief and not a number until the corpus has a frontier
half.

The verdict gain is on prose that was already ranked as machine. These signals lift
passages from the high teens into the twenties and thirties. A passage with a human-shaped
rhythm and no comma habit, like the two adversarial READMEs that scored zero, still scores
near zero, and should.

`ensure` is a word, and a word list depreciates. Its thirteen percent on maintained-project
READMEs is the highest rate of any rule on human prose, above `powerful` at nine. That is a
finding cost, not a verdict cost, and it is the one number in this change to watch. Pulling
the four entries from the block list is one line if it turns out to be the wrong trade.

The web app's score popover does not show the new component, though the JSON carries it.

## What would come next

A frontier half for the long-form corpus, through `-generate-longform` with a `claude-`
model, the moment a key with credit exists. That is where "X, not Y", "rather than", and
whatever else the frontier register carries can be measured, and it is the first thing the
evaluation protocol asks for.

An unflattened development corpus, so paragraph shape can be measured at all. Five
features and two shipped walkers are blind on the current one.
