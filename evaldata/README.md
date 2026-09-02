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
