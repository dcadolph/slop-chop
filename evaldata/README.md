# The locked evaluation corpus

The benchmark in `sanitize/testdata/` is a regression guard: it proves the rules still
catch what they caught yesterday. It cannot prove the score tracks human judgment,
because the same process that wrote the rules wrote its passages. This directory holds
the corpus that can: samples collected after the rules were frozen, never used to tune
anything, rated blind by people, then scored once.

The question it answers is narrow and worth answering: does the slop score correlate
with how machine-written a text reads to a person who has never seen the ruleset?

## The lock

Three rules make this corpus evidence rather than more homework.

1. No sample here may be used to develop, tune, or debug a rule. A rule change motivated
   by an eval sample disqualifies that sample, permanently.
2. No sample text may appear in the development corpus. CI enforces the disjointness:
   `go run ./evaldata/harness -check` fails the build on any overlap.
3. Samples are added only against a named release tag, recorded in the sample's `rules`
   field, so every published result names the exact ruleset it measured.

Once a result is published, the samples behind it stay frozen. New samples extend the
corpus. They never replace what a published number rests on.

### These samples are burned, 2026-09-21

Rule one was broken, and by the person who wrote it down. On 2026-09-21 this corpus was
used to hunt for features that separate the halves, and then to choose the threshold and
the weight of the cadence penalty that came out of that hunt. That is development, done
against evaluation samples, which is the one thing the lock exists to prevent.

Every sample here is therefore disqualified for measuring the ruleset that hunt produced.
The separation figures published below are training numbers. They are kept rather than
deleted, because a corpus that quietly loses its embarrassing results is worth less than
one that keeps them, but none of them is held-out evidence and none should be quoted as
though it were.

The precision results are not affected. Those were measured on README prose collected
separately, never used to tune anything, and the cadence penalty was checked against a
fresh sample of it after the fact rather than fitted to it.

What a real separation number needs now is a corpus collected after the current ruleset
was frozen and never read while changing it. That corpus does not exist yet. Until it
does, the honest statement about how well the score separates machine prose from human
prose is that it is unmeasured.

### The cadence penalty replicated, on data collected afterward

The burn leaves the size of the effect unmeasured, not the existence of it. The long-form
development corpus described below was collected after the cadence weight was frozen and
was never used to adjust it. On it, machine prose runs at a coefficient of variation of
0.360 against 0.707 for human, a Cohen's d of -1.78, firing on 86 percent of machine
passages and 20 percent of human ones.

That is a replication and not a held-out result. It is development data by designation, so
the first time anything is tuned against it the number becomes a training number in exactly
the way the ones below did. It is recorded here so the penalty is not carried on nothing
while the real corpus is collected.

### Why it happened, and what changed so it cannot happen the same way

Calling the violation carelessness would be letting the repository off. The development
corpus at `sanitize/testdata/corpus.jsonl` is a rule exemplar file: every passage is the
smallest text that reproduces one tell. That is what makes it a precise regression net,
and it is two sentences long at the median. The rhythm signals need six sentences before
they act at all, so ten of its hundred and thirteen passages were eligible to trigger the
one under test.

The evaluation corpus was the only full-length prose in the repository. A signal that
could not be measured anywhere else was going to be measured there, and it was.

The fix is a second development corpus, `sanitize/testdata/longform.jsonl`, holding
full-length passages of both labels, collected the same way this corpus is and held under
the same lock. The lock now takes a list of development corpora rather than one path,
since a development corpus outside the lock is a corpus the rules can be tuned on.

```sh
# Human half: long README prose from repositories untouched since 2021, skipping every
# repository this corpus has already spent.
go run ./evaldata/harness -collect-longform 80

# Machine half: local models across six long-form genres and three prompt styles.
go run ./evaldata/harness -generate-longform 1
```

The first collection run pulled twenty-eight locked samples straight back in, because the
skip list read the false-positive manifests and nothing else, and the human half of this
corpus records its origins only on the samples themselves. The lock caught all twenty-eight
before anything was measured, which is the first time it has been the thing that stopped a
mistake rather than a rule that was merely agreed to. The skip list now reads all three
records.

## Collecting samples

Target at least 150 machine and 150 human samples before publishing anything. Each
sample is one JSON object on one line in `samples.jsonl`:

```json
{"id": "a001", "source": "ai", "rules": "v0.36.0", "meta": {"model": "claude-opus-5",
 "prompt": "plain", "genre": "email"}, "text": "..."}
```

| Field    | What it holds                                                              |
|----------|-----------------------------------------------------------------------------|
| `id`     | Stable unique id. Prefix `a` for machine samples and `h` for human ones.&nbsp; |
| `source` | Ground truth: `ai` or `human`. Raters never see it.                         |
| `rules`  | The release tag whose rules were frozen before this sample was collected.   |
| `meta`   | Provenance: model and prompt style for `ai`, origin and genre for `human`.  |
| `text`   | The sample, 60 to 400 words.                                                |

Machine samples: at least five model families, and for each a spread of prompt styles,
including the plain ask, a styled ask, and the adversarial ask that tells the model to
avoid AI cliches, since that is the text the ruleset is most likely to miss. Spread the
genres: email, README, blog post, report, chat reply.

Human samples: text with provenance that predates the model era where possible, and a
spread of registers: technical writing, journalism, casual notes, academic prose. Human
text that shares vocabulary with the machine register is wanted, not avoided. That is
the false-positive frontier.

## Rating protocol

Raters see the text and nothing else: no labels, no scores, no slop-chop output, no
other raters' answers. Ask one question per sample:

> How machine-written does this read to you? 1 means certainly a person, 7 means
> certainly a machine.

At least three raters per sample. Each answer is one line in `ratings.jsonl`:

```json
{"sample": "a001", "rater": "r01", "machine": 6}
```

Raters are identified by opaque ids. A rater who has read the slop-chop ruleset or
docs is a worse rater for this purpose, so recruit outside the project.

## Analysis

Once ratings exist, the harness does the rest:

```
go run ./evaldata/harness
```

It scores every sample with the default profile and reports:

- Spearman correlation between the slop score and the mean human rating. This is the
  headline number: does the score order texts the way people do?
- How well the score separates the `ai` and `human` labels, as the probability that a
  random machine sample outscores a random human one.
- Rater consistency, as the mean pairwise correlation between raters, so a weak headline
  number can be traced to raters who disagree with each other rather than with the tool.
- The disagreements, both directions: samples the score calls heavy slop that people
  read as human, and samples people read as machine that the score waves through. These
  are the finding, whatever the headline number says, and they get published either way.

## What the result means

A strong correlation says the ruleset's patterns track the thing people actually
perceive, on text it never trained against. A weak one says the score measures rule
compliance and the product language must keep saying so. Either result is worth having
before rule two hundred, and either result gets published, failures included.

## Rating the corpus

The protocol asks for people to read samples without knowing which are machine-written,
and until now nothing presented them that way, so the corpus sat collected and unrated.

```sh
go run ./evaldata/harness -rate your-id
go run ./evaldata/harness -rate other-id -rate-seed 42
```

Samples come one at a time in an order derived from the seed, so two raters see different
orders and either order can be reproduced. The label is never shown, the score is never
shown, and nothing on screen hints at the answer. Answers are appended as they are given,
so a session can be stopped with `q` and resumed later without repeating a sample.

A rater is somebody doing a favor, not somebody setting up a development environment, so
there is a second path that asks nothing of them but reading and typing a number:

```sh
go run ./evaldata/harness -rate-sheet sheet.csv -rate-seed 42   # send this file out
go run ./evaldata/harness -rate-import filled.csv -rate their-id
```

The sheet is a CSV with an id, the text, and an empty answer column. It carries no label,
no model name, and no origin, because a spreadsheet is exactly the kind of file somebody
scrolls sideways in. On the way back a blank is a skip and an answer outside one to seven
is refused out loud, since a corpus quietly holding a rating nobody gave is worse than one
missing a rating. The columns are matched by header name, so a rater who reorders them in
a spreadsheet does not shift every answer onto the wrong sample.

The blocker this leaves is the real one. The tool no longer stands in the way. Independent
raters do, and a project nobody has heard of has no obvious source of them.

## Generating the machine half

```sh
go run ./evaldata/harness -generate-ai 3 -generate-tag v0.39.2
```

Samples are generated locally through ollama, across genres and across three ways of
asking: the plain ask, the styled ask, and the adversarial ask that names the cliches and
tells the model to avoid them. The adversarial third matters most, since that is the prose
a word list is least likely to catch. A reply outside the sixty to four hundred word band
is discarded rather than trimmed, because trimming would edit the model's writing and the
sample is supposed to be what it actually wrote.

A model name beginning with `claude-` goes through the Messages API instead of the local
daemon, so one flag mixes both in a run:

```sh
export ANTHROPIC_API_KEY=...
go run ./evaldata/harness -generate-ai 2 \
  -generate-models claude-opus-4-8,llama3.2:3b -generate-tag vX.Y.Z
```

A reply the model did not finish and one it declined to write are both discarded rather
than stored, since either would put a passage in the corpus that nobody actually produced.

The limitation is worth stating before anyone quotes a number off this corpus. The
protocol asks for at least five model families. What is here comes from small local open
models, because they cost nothing and need no key, and small local models do not write the
way the frontier models do. That makes this half of the corpus a weaker test than it looks:
it measures whether the score tracks human perception of machine prose in general, not
whether it tracks perception of the prose people actually mean when they say AI slop.
Replacing it with frontier samples is the first thing to do when a key is available.

## Matching the register, and why it nearly went wrong

The two halves have to be comparable or the experiment measures the wrong thing. The human
samples started as nineteenth century essays and letters, which was fine while there was
nothing to compare them against. Filling the machine half with modern work email, READMEs,
and status reports broke that: a rater set to separate Emerson from a generated
maintenance notice can score a perfect result on century and genre alone, without once
judging whether anything reads machine-written.

That would not have looked like a failure. It would have looked like a strong correlation,
which is the worst kind of wrong result to publish.

So the human half now takes README prose from repositories untouched since 2021, the one
modern human register available in volume with a hard date guarantee. Repositories already
spent on the false positive measurement are excluded, since those were used to weigh a
candidate rule change and the lock keeps them out.

The markup is stripped rather than kept: fences, headings, badges, tables, and lists go,
and so do link syntax and code spans inside the lines that survive. A human sample still
carrying a backtick is separable from a generated one on markup alone, which hands over
the answer as surely as the label would.

Four of the five machine genres still have no human counterpart. Email, blog, report, and
chat written before 2022 are not available in volume under a license that allows
redistribution, so the corpus is honest about covering one genre on both sides rather than
pretending to cover five. A number read off it describes README prose.

## The pre-2022 false positive measurement

The rated corpus above needs people. This one needs nobody, and it is available today.

Ground truth here is the calendar. A README written before 2022 predates any general
writing model in ordinary use, so the prose is human by construction. That gives a large
sample of real professional writing, drawn from repositories nobody involved with this
project has ever seen, at no cost beyond the API calls.

Two samples, because one of them has a bias worth removing:

```sh
# Repositories with no push since 2021. Cheap, and the guarantee is one field.
go run ./evaldata/harness -collect-pre2022 1400 -pre2022-file evaldata/pre2022.jsonl

# Maintained repositories, README read at the last commit before the cutoff. Costs two
# more API calls per repository and removes the abandonment bias.
go run ./evaldata/harness -collect-pinned 600 -pre2022-file evaldata/pinned.jsonl

go run ./evaldata/harness -pre2022 -pre2022-file evaldata/pre2022.jsonl
```

A sample of repositories untouched since 2021 is a sample of abandoned projects, and
abandoned projects may write differently from maintained ones. That bias runs in the
direction that flatters the engine, so the pinned sample exists to check it. Agreement
between the two is the result worth reporting.

A README is scored only when it carries at least a hundred words of prose once code and
fences are masked, and only when nine tenths of its letters are ASCII. The rest are badge
walls, stubs, and READMEs in other languages, where a score reads nothing. The skip count
is reported alongside the result, because it is large.

### What this measures, and what it does not

It measures one thing: how often the engine fires on professional human writing it has
never seen. The report names the rate at the reads-clean threshold and, more usefully,
ranks the rules by how many distinct human documents they fired on. A rule near the top
of that list is either a real tell people use anyway or a rule that wants a guard.

It cannot measure detection. There are no machine samples here, so the run says nothing
about recall, and a low false positive rate is not evidence that the engine works. It is
evidence that the engine is quiet where it should be quiet. Those are different claims
and only the rated corpus above can make the other one.

It is also one genre. READMEs are terse, technical, and heavy on lists and headings. A
rate that holds across seven language ecosystems is worth more than one measured on a
single community, which is why the collection spreads across them, but none of it
generalizes to essays, marketing copy, or long-form argument. The number describes
README prose and should be quoted that way.

### Result, 2026-09-15, ruleset v0.39.2

Both samples, scored once and published whatever they said.

| | Push-date | Pinned |
| --- | --- | --- |
| Scored | 852 | 418 |
| Ecosystems | 7 | 7 |
| Median prose | 371 words | 543 words&nbsp; |
| Median score | 1 | 1 |
| p90 / p99 | 5 / 14 | 5 / 10 |
| Worst sample | 20 | 21 |
| **At or above 25** | **0 (0.0%)** | **0 (0.0%)** |

1270 human documents, no false positives, and the worst of them sat four points under
the line. That result belongs to v0.39.2 and no longer describes the engine: the evidence
component added on 2026-09-21 takes a fresh collection of 886 documents to seven at the
line, which is the price of being able to see a long machine document at all. The
measurement is redone in docs/BENCHMARK.md and the trade is set out there. The numbers
below are kept as the record of what v0.39.2 measured. The abandonment bias the pinned sample exists to check runs the opposite way
from the flattering one: maintained projects write 46 percent more prose per README and
are skipped far less often, so that sample hands the engine more surface to trip on and
it still found nothing.

`pre2022-manifest.jsonl` and `pinned-manifest.jsonl` carry one row per scored sample:
repository, commit, ecosystem, word count, score, and the rules that fired. No README
text is redistributed, and the commit makes every row re-fetchable, so the numbers above
can be checked rather than believed.

### What the result did not justify

The rules that fire most on human prose are the lexical ones, and the attack command
shows the lexical rules are also the evadable ones: 35 of 76 word tells and 8 of 12
phrase tells fall to a thesaurus, against 2 of 66 structural tells. Two lines of evidence
pointing the same way made a case for weighing the lexical half less.

Measuring it did not support the change. Dropping word and phrase weight to 0.75 removes
the one human false positive in the development corpus and loses one machine passage, and
reading the two shows why: `That said, there are real tradeoffs` and a sentence of stacked
corporate buzzwords both move from 32 to 24. The weight moves everything uniformly and
separates nothing, so the trade is one true positive for one false positive.

No rule in the 1270 documents fires on more than 6 percent of them, and the false positive
that does exist comes from a single ordinary finding in an eighteen-word passage rather
than from any rule being weighed too heavily. The evidence says the weighting is already
where it should be, so it stays there. The lexical rules remain the evadable half, which
is a fact about durability rather than about precision, and those are different claims.

### First result off the locked corpus, 2026-09-16, ruleset v0.39.2

See the burn notice above. This number was held-out when published and is not any more,
because the samples behind it were later used to tune the cadence penalty.

No ratings yet, so this answers the narrower question the labels alone can answer: does
the score tell the halves apart on prose the rules never saw?

| | Machine | Human |
| --- | --- | --- |
| Samples | 89 | 98 |
| Mean score | 9.7 | 1.9 |
| At or above 25 | 8 | 1 |

Separation by score is 0.752, where 0.5 is chance and 1.0 is perfect.

Put that beside the development corpus, where the machine half means 78.0 and 99 percent
of it clears 25. Here 9 percent clears 25. The same engine, the same threshold, and a
different answer, which is the whole reason a corpus collected after the rules were frozen
is worth the trouble.

By prompt style, machine samples at or above 25:

| Ask | Samples | Mean | Cleared 25&nbsp; |
| --- | --- | --- | --- |
| styled | 27 | 11.9 | 4 |
| plain | 31 | 9.4 | 3 |
| adversarial | 31 | 8.0 | 1 |

The gradient runs the way it should. A model told to sound polished writes the most
catchable prose, and a model told to avoid the cliches by name writes prose the ruleset
almost never flags. That is the ruleset working exactly as advertised against the tells it
lists, and it is also the ceiling on what a list of tells can do.

### Second result, 2026-09-20, five model families

See the burn notice above. Same samples, same disqualification.

The machine half was extended from two families to five. The detection rules did not
change between the two tags, so the numbers are measured by the same engine and the
corpora combine.

| | First | Second |
| --- | --- | --- |
| Machine samples | 89 | 151 |
| Families | 2 | 5 |
| Human samples | 98 | 98 |
| Separation | 0.752 | 0.785 |
| Machine at or above 25 | 8 (9%) | 21 (14%) |
| Human at or above 25 | 1 | 1 |

The aggregate moved the right way. The breakdown says not to trust it.

| Model | Samples | Mean | Cleared 25&nbsp; |
| --- | --- | --- | --- |
| gemma2:2b | 16 | 19.5 | 38% |
| mistral:7b | 30 | 14.2 | 23% |
| qwen2.5-coder:14b | 40 | 12.1 | 15% |
| llama3.2:3b | 49 | 7.7 | 4% |
| phi3:mini | 16 | 6.4 | 0% |

Thirty-eight points of spread across five models, none of them frontier, all given the
same prompts. Whether a passage trips the rules depends more on which model wrote it than
on anything the corpus was built to measure, and the headline rate is therefore a fact
about the mix of models in the sample rather than a fact about machine writing. Change the
mix and the number moves, which is why the first result's nine percent and this one's
fourteen are the same finding rather than an improvement.

The prompt gradient held across the larger sample: styled 22 percent, plain 14, and the
adversarial ask that names the cliches 6. A model told to sound polished still writes the
most catchable prose, and a model told to dodge the tells still mostly dodges them.

This makes the frontier gap worse, not better. If five small models span nothing to
thirty-eight percent, a number measured without any frontier model in the sample says very
little about the prose people mean when they say AI slop. Nothing here was tuned after the
result, and the samples stay frozen.

### Third result, 2026-09-21: the cadence finding

This is the hunt that burned the corpus. The separation figures in it are training
numbers and the precision figures are not, which is the only reason the change it produced
was kept.

The corpus was asked, for the first time, what actually separates the halves rather than
whether a rule fires. Ten features measured on both, no hypothesis in front of it.

| Feature | Machine | Human | Effect |
| --- | --- | --- | --- |
| **cadence variation** | **0.342** | **0.587** | **d = -1.31** |
| short copula rate | 0.004 | 0.034 | -0.70 |
| colon rate | 0.089 | 0.428 | -0.67 |

Every feature ran the same direction, and machine prose had less of all of them. The
largest by a distance is sentence length that barely moves, and it holds with the genre
fixed: measured against human README prose alone the gap widens rather than closing.

That contradicted the reasoning the score had carried for months, which was that modern
model prose varies its rhythm on purpose and a flat cadence would only punish plain
competent human writing. The first half is wrong on this sample. The second half is not,
and the shape of the change is built around it.

| | Separation | Machine at or above 25 | Human at or above 25&nbsp; |
| --- | --- | --- | --- |
| Before | 0.785 | 21 of 151 (14%) | 1 of 98 |
| After | 0.845 | 30 of 151 (20%) | 1 of 98 |

Detection rose by half and the human half did not move. Against 311 passages of human
prose pulled fresh, 26 percent take some penalty and the rate at the threshold stays at
one, with the ninety-ninth percentile at 18 and seven points of headroom under the line.

Two choices keep it honest. The penalty scales with how flat the rhythm is rather than
switching on at a boundary, so prose near the line is nudged instead of condemned. And it
does nothing below six sentences, because a coefficient of variation over three is noise
and three short even sentences are a note rather than a document.

A larger cap was available. Twenty-five points reached 0.911 separation and 32 percent
detection with the same single false positive, and it was not taken: it left five points
of headroom where fifteen leaves seven, and precision is the claim this project can
actually defend.

### What this result does not say

It does not say the engine fails. The separation is well above chance, the human half is
nearly silent at 1 of 98, and that silence matches the 1270 README measurement. On the
question of not bothering honest writing, two independent corpora now agree.

It does not say the engine detects machine writing in general. Nine percent at the
threshold is what it says, on this corpus.

And the corpus has a limitation that cuts toward the engine's defense rather than against
it. The machine samples come from two small local models, and small models do not write
the polished register the ruleset was built from. A frontier model asked for a confident
professional voice produces far more of the prose these rules target. Replacing the
machine half with frontier samples is the first thing to do when a key is available, and
the number above should be read as provisional until that happens.

No rule was changed after seeing this. The lock holds, the samples stay frozen, and the
result is published as it came out.
