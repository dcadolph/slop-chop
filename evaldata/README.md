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
the line. The abandonment bias the pinned sample exists to check runs the opposite way
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
