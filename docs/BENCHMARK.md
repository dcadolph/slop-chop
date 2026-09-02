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
stock phrases, sentence shapes, hedging, and the characters models type. A high score
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
| `ai`        | 58       | Machine-register prose: each passage exercises specific tells, from buzzword density to the polished 2026 register with no lexical tells at all.&nbsp; |
| `human`     | 35       | Human prose chosen to trip a careless detector: poetry heavy with em-dashes, ornate academic writing, plain conversational notes, a graduation speech. |
| `technical` | 22       | Precision traps: RFC normative language, legal parallel structure, reference docs that repeat their subject, prose that has every right to sound formal. |

The passages were written and curated during adversarial audit rounds, on purpose, to
probe where the engine fails. That is also the corpus's main limitation, covered below.

## Current numbers

The benchmark reports these on every run and fails below the floors.

| Metric                                | Current | Floor |
|---------------------------------------|---------|-------|
| Tell recall (AI passages with a tell) | 0.98    | 0.95  |
| Technical precision (no false tell)   | 1.00    | 0.98  |
| Score recall (AI at 25 or higher)     | 0.98    | 0.95  |
| Score precision (at 25 or higher)     | 0.98    | 0.98  |
| Mean score, AI passages               | 77.8    |       |
| Mean score, human passages            | 1.9     |       |
| Score margin (AI mean minus human)    | 75.9    | 70    |

The floors sit below the current numbers so ordinary changes pass while a real
regression fails. They ratchet up as the engine and the corpus improve.

## What fires on what

116 distinct rules fire across the 58 AI passages. By class:

| Class      | Findings | Score weight                     |
|------------|----------|-----------------------------------|
| Structural | 78       | 2 per finding                     |
| Word       | 71       | 1                                 |
| Phrase     | 13       | 1                                 |
| Character  | 5        | 1 for the em-dash and invisibles&nbsp; |
| Tidy       | 2        | 0                                 |

Sentence shapes, not word lists, carry the most weight. That is deliberate: a stock
sentence shape is stronger evidence than one word, and word lists are the first thing a
model gets trained away from.

On the 35 human passages the engine finds typography almost exclusively: curly quotes,
en-dashes, an ellipsis. Those carry zero score weight, which is why a professionally
typeset human page averages 1.9 rather than getting flagged for its punctuation. One
`that said,` and one semicolon round out the human findings. The 22 technical passages
produce zero findings.

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
authors, and there are no confidence intervals on 115 passages. It is a regression
guard with teeth, not a study.

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
