# Analysis plan

This plan was fixed on 2026-10-04, before a single rating was collected and before any
score was read on the corpus it describes. The commit that adds this file is the record
of that. Everything below decides in advance what gets measured, what each result would
mean, and what gets published, so the numbers cannot choose the analysis after the fact.

The last corpus burned because someone looked first and decided what to measure second.
This file exists so that the order cannot be reversed again.

## What is being measured

The corpus is `evaldata/samples.jsonl`: 84 samples, 50 human and 34 machine, pinned to
the ruleset frozen at v0.41.0. Its composition and collection are described in
[README.md](README.md).

This plan covers those 84 samples and no others. Samples added later, such as machine
samples from frontier models, are a separate measurement with a plan of their own. They do
not join this scored run, and this run does not wait for them. They go in a file of their
own rather than `samples.jsonl`, since the harness scores only once every sample in that
file has enough raters.

The question is whether the slop score orders texts the way blind readers do. The
question is not whether the score detects machine authorship, and no result here will be
described as if it were.

## Who may rate

A rater must not have read the slop-chop rules, the docs, the tell list, or any
announcement of the project. Someone who knows what the rules look for is rating against
the rules, which measures nothing. Raters are recruited before any public announcement,
and anyone who has seen one is not recruited afterward.

Raters are identified by opaque ids. The instrument is the one in the README: one passage
at a time, a 1 to 7 answer to "how machine-written does this read to you", with no label,
score, or other rater's answer shown.

## Stage one: the pilot

The pilot asks whether readers agree with each other at all. If they do not, no score can
agree with them either, and finding that out costs twenty passages per person rather than
eighty-four.

- Twenty samples, ten machine and ten human, chosen by a hash of each sample id. The
  choice reads no text and no score. Export with
  `go run ./evaldata/harness -rate-sheet sheet.csv -rate-pilot 20 -rate-seed N`, with a
  different seed for each rater so each sees a different order.
- About ten raters.
- The pilot reports rater agreement only, from a plain
  `go run ./evaldata/harness`, which never loads the engine.

The decision on the mean pairwise Spearman correlation between raters:

| Agreement      | Decision                                                            |
|----------------|---------------------------------------------------------------------|
| 0.40 and above | Go on to the full corpus.                                           |
| 0.20 to 0.40   | Go on, and the final report states that agreement was weak.&nbsp;   |
| Below 0.20     | Stop. Publish the pilot agreement as the result.                    |

These thresholds are judgment calls. They are written down now so they cannot be moved
once the pilot number is known.

Pilot ratings are kept and count toward the full corpus, since the instrument and the
samples are the same. A pilot rater who goes on to rate the full sheet keeps their first
answer on the twenty pilot samples. The harness enforces this on import, and
`-check` fails on any rater who answered one sample twice.

## Stage two: the full corpus

Every sample needs at least three raters, with ten as the target. The harness refuses to
score until every sample has three:

```sh
go run ./evaldata/harness -score
```

That run happens once. Its output is committed verbatim, alongside the ratings it read.

## What the scored run reports

All of this is fixed in the harness as of this commit.

**Primary.** The Spearman correlation between the slop score and the mean human rating
across all 84 samples, with a 95 percent bootstrap interval (2000 resamples, seed
20261004).

**Required companion.** The same correlation computed within the machine samples alone
(34) and within the human samples alone (50). Across the whole corpus, a score that only
tells the two halves apart will correlate with ratings that also tell them apart, without
tracking anything about how machine-written a given text reads. Within one half the true
answer is held constant, so a correlation there is the score following perception. With
34 and 50 samples these figures are read for direction, not as proof.

**Secondary.** Separation by score between the labels, as the probability that a random
machine sample outscores a random human one. Rater agreement across the whole corpus.

**Exploratory.** The correlation of each score component (density, hedging, cadence,
evidence, drumbeat) with the mean rating. No claim rests on these. Several components are
zero on most samples, and the list was fixed before scoring so it cannot be trimmed to
the ones that look good.

**Always.** Both disagreement lists, in full: samples the score calls heavy slop that
readers took as human, and samples readers took as machine that the score let through.

## What each result would mean

| Result | What may be said |
|--------|------------------|
| Interval above zero, both within-half correlations 0.20 or more | The score tracks how machine-written a text reads, beyond telling the halves apart. |
| Interval above zero, either within-half correlation under 0.20 | The score agrees with readers on which texts are machine-written. It is not shown to track degree, and the report names the half where it failed to. |
| Interval includes zero | No evidence the score agrees with readers. It is described as rule compliance and nothing more. |
| Interval below zero | The score runs against reader judgment, and that is the headline. |

Whichever row the result lands in decides the language on the site and in the README,
and that language changes to match before anything else does.

## What will not be done

- No per-model or per-prompt breakdowns. 34 machine samples across five model families
  is about seven per family, and a number from seven samples is noise.
- No second scored run with a different analysis. If the harness turns out to have a bug
  after scoring, the fix is committed, the run is repeated, and both outputs are
  published along with the fix.
- No rater is dropped after scores are seen. The only exclusions are fixed now and are
  applied from the ratings alone, before scoring: a rater who gave one answer to every
  sample, and a rater who answered fewer than ten samples.
- Nobody working on the rules reads the ratings next to the sample texts before scoring.
  That pairing tells you which texts readers found machine-like, which is exactly the
  signal rule one of the lock forbids tuning on.

## The ruleset that gets measured

The scored run measures the rules at the commit it runs from, not the tag the samples were
pinned to. The rules may keep changing in the meantime, as long as no change is motivated
by this corpus. At scoring time, `git diff v0.41.0 -- sanitize/` is recorded with the
output, and the report names the commit it measured.

When this plan was written, the only change under `sanitize/` since v0.41.0 was
`band.go`, which adds a lookup for the score bands and changes no score.

## Limits, stated before the result

- The raters are volunteers known to the author. They are not a sample of anyone in
  particular, and the report says so.
- Both halves are READMEs. Markup was flattened out of both, but the human half predates
  2022 and the machine half was generated in 2026, so differences of era in the prose
  itself remain. That is the price of a human half that cannot contain machine text.
- The machine half came from local models, not frontier ones. Prose from a frontier model
  may carry different patterns.
- 84 samples is a small corpus. The interval on the primary figure is the honest measure
  of how small.

## Publication

Everything above is published, whatever the result: the agreement figure, the primary
figure and its interval, both within-half figures, the components, the disagreement
lists, and the ratings themselves. A null result is published as prominently as a good
one would have been.
