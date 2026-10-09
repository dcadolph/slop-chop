# The benchmark

The engine ships with a labeled corpus and a benchmark that runs with the test suite on
every push. When a change drops detection or flags human writing, the build fails. This
page shows what is measured, the current numbers, and the limits of what they mean, so
the claim on the front page has its receipt.

Everything here reproduces from the repository:

```
go test ./sanitize/ -run TestBenchmark -v
```

## What the score claims, and what it does not

The slop score measures the density of patterns from the engine's ruleset: buzzwords,
stock phrases, sentence shapes, hedging, sentences that keep closing on an -ing clause, a
comma in nearly every sentence, and the characters models type. A high score
means the text carries many machine-writing patterns. It does not determine who wrote
the text. A person can write in the polished-assistant register, and a machine can be
prompted away from every pattern the rules know. The score is a lint result, not an
authorship verdict.

## The corpus

The corpus is one JSON object per line at `sanitize/testdata/corpus.jsonl`, embedded in
the engine and versioned with it. Each passage carries a label and a note naming the
tell or the trap it exercises.

| Label       | Passages | What it holds                                                          |
|-------------|----------|------------------------------------------------------------------------|
| `ai`        | 71       | Machine-register prose: each passage exercises specific tells, from buzzword density to the polished 2026 register with no lexical tells at all.&nbsp; |
| `human`     | 46       | Human prose chosen to trip a careless detector: poetry heavy with em-dashes, ornate academic writing, plain conversational notes, a graduation speech. |
| `technical` | 36       | Precision traps: RFC normative language, legal parallel structure, reference docs that repeat their subject, prose that has every right to sound formal. |

The passages were written and curated during adversarial audit rounds, on purpose, to
probe where the engine fails. That is also the corpus's main limitation, covered below.

There is a second development corpus beside it at `sanitize/testdata/longform.jsonl`,
holding 145 full-length passages: 80 human READMEs from repositories untouched since 2021,
and 65 machine passages across five local model families, seven genres, and three prompt
styles. It exists because the corpus above is two sentences long at the median, which is
the right size for pinning what a rule matches and too short for any signal that reads a
whole passage. Both files are held under the same lock that keeps the evaluation corpus
out of rule development.

## Current numbers

The benchmark reports these on every run and fails below the floors.

| Metric                                | Current | Floor |
|---------------------------------------|---------|-------|
| Tell recall (AI passages with a tell) | 0.99    | 0.95  |
| Technical precision (no false tell)   | 1.00    | 0.98  |
| Score recall (AI at 25 or higher)     | 0.99    | 0.95  |
| Score precision (at 25 or higher)     | 0.99    | 0.98  |
| Mean score, AI passages               | 83.4    |       |
| Mean score, human passages            | 2.5     |       |
| Score margin (AI mean minus human)    | 80.9    | 70    |

The floors sit below the current numbers so ordinary changes pass while a real
regression fails. They ratchet up as the engine and the corpus improve.

## What fires on what

122 distinct rules fire across the 71 AI passages. By class:

| Class      | Findings | Score weight                     |
|------------|----------|-----------------------------------|
| Structural | 90       | 2 per finding                     |
| Word       | 74       | 1                                 |
| Phrase     | 13       | 1                                 |
| Character  | 5        | 1 for the em-dash and invisibles&nbsp; |
| Tidy       | 2        | 0                                 |

Sentence shapes, not word lists, carry the most weight. That is deliberate: a stock
sentence shape is stronger evidence than one word, and word lists are the first thing a
model gets trained away from.

On the 46 human passages the engine finds typography almost exclusively: nineteen curly
quotes, en-dashes, and ellipses. Those carry zero score weight, which is why a
professionally typeset human page averages 1.5 rather than getting flagged for its
punctuation. One `that said,` and one semicolon round out the human findings. The 36
technical passages produce zero findings.

## Attacking it on purpose

`slop-chop attack` rewrites text to dodge the rules without improving the writing: a
listed buzzword becomes an unlisted one, a stock opener becomes an unstocked one, an
em-dash becomes punctuation no rule reads. Then it reports what survived. Every
replacement it reaches for is itself checked against the default profile by a test, so an
evasion that stops evading fails the build.

Run against all 71 AI passages, the attack tells the story the score weighting rests on:

| Class      | Evaded | Held | What that means                                                    |
|------------|--------|------|---------------------------------------------------------------------|
| Word       | 33     | 41   | A word list is a lookup, and a lookup loses to a thesaurus.&nbsp;    |
| Structural | 1      | 89   | No swap reaches a sentence shape, so this bounds the attacker.&nbsp; |

67 of the 71 passages still carry a tell after the attack. That gap is why a structural
tell counts two and a word counts one, and the test that measures it fails the build if
the classes ever invert.

The asymmetry is what the score weighting rests on. It says a published word list is a
depreciating asset, and that the structural rules are worth more per finding.

Read both rows as facts about this one attacker rather than about evasion in general.
The word row is a floor: the evasion table covers a few dozen entries against a block
list of 161, so a fuller thesaurus would evade more.

The structural row carries a sharper limit, and it is the one to be careful with.
`slop-chop attack` substitutes, and a substitution does not rebuild a sentence, so the
structural rules are measured here against an attacker that cannot reach them. The row
shows the attacker's reach rather than the rules' durability. Nothing in this run tried
to escape a sentence shape, so nothing in this run says a sentence shape is hard to
escape.

Testing that needs an attacker that rewrites rather than swaps, which means a model told
to keep the meaning and change the shape. The published paraphrase attacks on neural
detectors do exactly that, with a separate control for reordering, and they report large
drops in detection. Whether a fixed rule set holds up better than a classifier is a fair
question and an open one. Until that run happens, treat the durability of the structural
half as unmeasured.

[slop-chop on slop-chop](ITSELF.md) turns the engine on its own documentation and output:
whether the docs pass, whether chopping settles, and what one pass actually leaves behind.

## Prose the rules never saw

The corpus above shares an author with the rules, which is the limitation its own section
admits. One question it cannot answer is whether the engine fires on ordinary professional
writing nobody involved here has read. That one has an answer that needs no raters, because
ground truth can come from the calendar: a README in a repository with no push after 2021
was written before a general writing model existed.

| | Abandoned projects | Maintained projects&nbsp; |
| --- | --- | --- |
| READMEs scored | 618 | 268 |
| Ecosystems | 7 | 7 |
| Median prose | 369 words | 573 words |
| Median score | 1 | 4 |
| p90 / p99 | 13 / 22 | 15 / 26 |
| Worst sample | 35 | 36 |
| **Scoring 25 or higher** | **4** | **4** |

886 documents across Go, Python, Rust, JavaScript, Java, Ruby, and C++, of which eight
reach the reads-clean line: 0.9 percent. The maintained column read three until the
participle tail signal of 2026-10-05 moved one document from 24 to 25, which the recheck
that day missed because it ran the abandoned column only. The clause habit and the three
rules added beside it on 2026-10-08 moved no document in either column, measured on the
engine before and after. The second column is the check on the first: a
repository untouched since 2021 is an abandoned one, and abandoned projects might write
differently. They do, in the direction that makes the test harder rather than easier, since
maintained projects carry half again as much prose per README for the engine to trip on.

This measurement read zero for a long time, across a larger draw of 1270 documents. The
seven are the price of the evidence component, which is what lets the engine see a long
machine document at all, and subtracting that component back out returns every one of them
to clean. The section on full-length prose above carries the other half of that trade. The
numbers here come from a fresh collection under the current ruleset rather than the older
one, so the sample is smaller than the draw that reported zero.

The rules that do fire on human prose are worth naming: `word:powerful` leads at six
percent of documents, then curly quotes, `structural:bold-bullet-run`, and
`structural:template-stem`. Nothing reaches seven percent. The character rules dominate the
raw counts and contribute nothing to the score, which is the separation between a finding
and a score weight doing its job on writing it has never seen.

This measures quiet, not detection. There are no machine samples in it, so it says nothing
about recall, and a clean sweep is evidence the engine stays out of the way rather than
evidence it works. It is also one genre, and a README is terse and list-heavy, so the
number describes README prose and is quoted that way. `evaldata/README.md` carries the
method, and the two manifests beside it carry a row per sample with the commit it was read
at, so the numbers can be rechecked rather than believed.

## Full-length prose, both halves

The measurement above has no machine samples in it, so it reports quiet rather than
detection. The long-form corpus is the pair to it: human and machine prose at the same
length, in the same genres, scored by the same ruleset.

Two things come out of it, and they point in opposite directions.

**The engine separates full-length prose very well.** Machine passages mean 38.1 against
5.4 for human, which is a Cohen's d of 2.66. As a ranking instrument on real documents it
works, and the gap is not an artifact of the adversarial prompt style: passages written
under an instruction to avoid every cliche of AI writing score about the same as the plain
ones.

**The verdict did not follow it there, and the reason was the score itself.** Before the
evidence component, 13 of 65 machine passages reached the line a reader acts on, against 0
of 80 human. The score was a pure density, so the same evidence spread across four hundred
words read thinner than across twenty. The exemplar corpus reports a mean machine score in
the eighties because its passages average twenty words. Real machine prose at three hundred
and fifty words scored 17 and was called clean.

Moving the line was the obvious fix and the wrong one, since it changes every verdict the
tool has ever given and does nothing about the cause. The cause is visible in what a rate
discards. Counting total weighted evidence rather than only its rate against length:

| | Median weighted tells | Maximum&nbsp; |
| --- | --- | --- |
| Machine, long-form | 4.0 | 17.2 |
| Human, long-form | 1.0 | 5.5 |

A human document accumulates about one weighted tell however long it runs. A machine one
accumulates four. The score now adds up to fifteen points for accumulated evidence, at two
points per weighted tell, capped low enough that a count on its own cannot reach a verdict.
The multiplier was read off a flat region rather than a peak: every value between 1.5 and 3
recovers between 46 and 51 percent of long machine prose at the same cost.

The corpus is split in half by a hash of the passage text, and the shape was chosen on one
half only. On the half it was not chosen on, machine passages reaching the line went from 7
of 24 to 15 of 24, with human documents staying at 0 of 38.

**It is not free.** Scored against the abandoned-repository half of the false positive
corpus, the evidence component takes that measurement from 0 human documents at the line to
4 of 663. All four are caused by this component and none by the rhythm penalty, which was
checked by subtracting each one back out. Density does not separate them from machine
prose either: they run between 1.8 and 4.0 tells per hundred words, and machine long-form
averages 2.05, so there is no filter hiding here that keeps the recall and drops the cost.
The trade is two thirds of long machine prose caught against six human documents in a
thousand flagged, and it is a trade rather than a free improvement.

**Cadence replicates here.** The rhythm penalty was set on the evaluation corpus and burned
it, which left the size of its effect unmeasured. On this corpus, collected afterward and
never used to adjust it, machine prose runs at a coefficient of variation of 0.360 against
0.707 for human, a Cohen's d of -1.78. It fires on 86 percent of machine passages and 20
percent of human ones. That is a replication rather than a held-out result, since this is
development data and tuning anything against it would burn it too.

**It is also the one signal a thesaurus cannot reach.** Running the substitution attack
across the machine half removes 43 percent of the tells and 2 percent of the cadence
penalty. The 2 percent is real rather than zero: an evasion that swaps one word for two
changes a sentence length. But escaping a rhythm means rewriting sentences, and no lookup
table does that.

**Participle tails catch more without flagging more.** A sentence closing on a comma and
an -ing clause, "the fleet sat offshore, deploying its guns", is ordinary writing once and
the model register when it keeps happening. On this corpus 85 percent of machine passages
carry one against 36 percent of human passages, and machine prose runs at about eight times
the human rate. The score treats it like hedging: the rate counts and the instance never
does, the first tail is free, and the habit adds at most ten points.

Its settings were chosen while the whole corpus was in view, before the split was noticed.
Choosing again on the development half alone lands on the same settings, so the holdout did
not pick them, but it was seen, and these figures carry that. On the holdout half, machine
passages reaching the line went from 15 of 24 to 17 of 24, with human documents staying at
0 of 38. Separation on that half did not improve: 0.959 before and 0.957 after. So the tail
moves borderline machine prose over the line without ranking documents any better. Scored
against the 618 abandoned-repository READMEs of the false positive corpus, which nothing here
was tuned on, the change moves no document at all.

**The clause habit is the signal the attack cannot reach at all.** Half of human prose
sentences carry a comma. Three quarters of machine sentences do: a qualifier, an aside, a
consequence clause, on every sentence, so that nothing is ever stated flat. On the
development half the comma-bearing share runs 0.75 for machine prose against 0.49 for
human, a Cohen's d of 1.74, and on the holdout half 0.81 against 0.44, a d of 2.26. The
score charges it the way it charges cadence: nothing until the share clears 0.70, the human
ninetieth percentile on the development half, then a gradient to eight points at a comma in
every sentence, and nothing at all under six prose sentences. It touches three human
passages in each half and ten of the 618 abandoned-repository READMEs, moves none of them
to the line, and the substitution attack removes one percent of it, against two percent of
the cadence penalty and thirty-five percent of the tells.

Three lexical rules came out of the same measurement, and they are the evadable kind.
`ensure` appears in two thirds of machine long-form passages and one human passage in
twenty, the widest gap of any single word measured. It now fires on five percent of
abandoned-repository READMEs and thirteen percent of maintained-project ones, the highest
rate of any rule on human prose, and that cost is stated here rather than hidden. The
release-note fanfare, `we are thrilled to announce`, appears in no README of 852 written
before 2022 and in the one machine release note that scored zero. The chat sign-off now
takes any verb, since `Happy automating!` is the same sign-off as `Happy coding!`.

Together, on the holdout half, machine passages reaching the line went from 17 of 24 to 20
of 24 and on the development half from 24 of 41 to 28 of 41, with human documents staying
at 0 of 38 and 1 of 42. The probability that a random machine passage outscores a random
human one went from 0.878 to 0.924 on the development half and from 0.957 to 0.992 on the
holdout. In the simulation that chose these, the clause habit alone moved two holdout
passages and `ensure` alone moved three. The word did more of the verdict work and the
habit did the durable part.

One candidate was measured and left out. Nominalization density, the share of words ending
in -tion, -ment, -ness, and the like, separates the halves about as well on the holdout (d
2.12) and runs at 0.58 correlation with the clause habit, so it moved the same passages and
no others, and it touches eighteen percent of human READMEs, since technical prose is built
from those nouns. The record of the whole measurement, with every feature tried, is
`INVESTIGATION-clause-habit.md` in the repository.

## Limitations, plainly

**The corpus grades the engine on its own homework.** The passages were written during
the audits that also shaped the rules, so recall of 0.98 here demonstrates rule
coverage of known registers, not generalization to arbitrary text in the wild. The
numbers are honest about what they measure, and what they measure is narrower than "any
AI text anywhere."

**A low score is not a human verdict.** Text written to dodge every known pattern will
score low and may still read machine-made to a person. The score measures compliance
with the ruleset. The ruleset chases the current registers, and the registers move.

**No stratification.** The corpus is not sampled across models, prompts, genres, or
authors, and there are no confidence intervals on 153 passages. It is a regression
guard with teeth, not a study.

**A rate is not a verdict.** The score is tell density, so its headline numbers move with
passage length. The exemplar corpus reports a machine mean of 85 on twenty-word passages
and the long-form corpus reports 30 on three-hundred-word ones, from the same engine and
the same ruleset. Both are true and neither is the number. Any figure quoted off this page
has a passage length attached to it, stated or not.

**The gate was blind to anything longer than a paragraph.** Until the long-form corpus
existed, ten of the exemplar corpus's hundred and thirteen passages were long enough to
trigger a signal that reads sentence rhythm, so the benchmark could not have caught a
regression in one. That is how the rhythm work ended up measured against the evaluation
corpus, which burned it. The second corpus closes the hole for signals at that scale, and
the same hole will open again for any signal that needs more context than a single
document.

**What would settle it.** Blind human raters over a corpus the rules never saw: does
the score order texts the way people do? The scaffolding for that experiment lives at
`evaldata/` in the repository, with the collection protocol, the rating protocol, a CI
lock that keeps eval samples out of rule development, and a harness that computes the
correlation once ratings exist. The experiment has not been run. Until it has, the
honest claim stays: same input, same output, and the patterns the rules know get caught
every time.

## Growing it

Add a line to `corpus.jsonl` with a `label`, the `text`, and a `note` naming what it
exercises, then run the benchmark. A passage that exposes a miss is worth more than ten
that pass. When the numbers rise and hold, raise the floors so the gain is locked in.
