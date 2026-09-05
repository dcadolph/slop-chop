# slop-chop on slop-chop

A tool that judges writing should be willing to be judged by itself. This page is the
engine pointed at its own documentation, its own output, and its own rules. Every number
here is produced by the checked-in engine and re-checked in CI, so a change that breaks one
of these claims fails the build rather than quietly making this page a lie.

## Does the documentation pass its own check?

Yes, and CI enforces it. Every guide is run through `slop-chop check` on each push, and a
single finding fails the job.

| Page                                   | Score | Words |
|----------------------------------------|-------|-------|
| README.md                              | 0     | 2173&nbsp; |
| ENGINE.md                              | 0     | 2601  |
| docs/PROFILE.md                        | 0     | 1985  |
| docs/BENCHMARK.md                      | 0     | 1091  |
| docs/VOICE.md                          | 0     | 1045  |
| docs/MCP.md                            | 0     | 631   |
| docs/API.md                            | 0     | 187   |
| docs/quickstart.md                     | 0     | 189   |

The one exception is the front page of this site, and deliberately: its demo text is slop
on purpose, so it can be chopped in the box.

This is less impressive than it sounds and more useful than it looks. Writing prose that
passes is not hard once the rules exist. What it buys is that the rules stay honest,
because every false positive lands on the author first. Several rules in the engine were
narrowed because they fired on this documentation and were wrong to.

## Does chopping settle?

Running the engine on its own output changes nothing. Across all 115 labeled corpus
passages, `fix(fix(text))` equals `fix(text)`, and a test fails the build if that ever
stops holding.

That matters beyond tidiness. A pre-commit hook can run after an editor plugin already
chopped the file, a pipeline can chop twice, and a CI job can re-run, and none of them
produce drift. Text that has been chopped is a stable point, not a step in a sequence that
keeps moving.

Of the 115 passages, 101 are already at rest before the first pass and 14 change once and
then hold. None needs a third pass.

## What one pass actually does

Here is a paragraph written to carry as many tells as it can hold.

```
In today's fast-paced digital landscape, it's important to note that teams leverage a
myriad of robust tools to seamlessly streamline their workflows—often juggling five or six
apps just to ship one feature. Moreover, this creates friction. The best part? Our
comprehensive platform delves into the root causes. It's not just a tool, it's a paradigm
shift. Here are three ways it empowers your team to unlock synergy.
```

Score 80, with 18 tells in 67 words. After one pass with the `cleaver` preset:

```
In today's fast-paced digital landscape, teams use many solid tools to simplify their
workflows, often juggling five or six apps just to ship one feature. This creates friction.
The best part? Our thorough platform digs into the root causes. It's not just a tool, it's
a big change. Here are three ways it helps your team to unlock synergy.
```

Thirteen findings rewritten, six left. A second pass changes nothing.

And the score is still 80.

That is the honest part. Every tell the engine could safely rewrite is gone: the buzzwords,
the stock opener, the em-dash splice. What remains is structural, and the engine will not
touch it: the fragment reveal of `The best part?`, the `it's not just X, it's Y` contrast, the
listicle promise of `Here are three ways`. Fixing those means rewriting the sentences,
which changes what they say, which is the author's call and not a rule's.

So a chopped document is not a clean document. It is a document with the mechanical tells
removed and the judgment calls surfaced. The score stays high because the remaining
problems are real, and pretending otherwise would be the easiest possible lie for this tool
to tell.

## Can it evade itself?

`slop-chop attack` rewrites text to dodge the rules without improving it, then reports what
held. Word tells fall to a thesaurus. Sentence shapes do not, because escaping one means
rebuilding the sentence. Across the corpus the attack evades 33 word tells and 1 of 78
structural ones, and 54 of 58 machine passages still flag afterward.

That gap is why a structural tell counts twice what a word does in the score, and
[the benchmark page](BENCHMARK.md) has the full table along with the limits of what any of
it proves.

## What none of this shows

That the score tracks what a person would say about a text. The corpus was written
alongside the rules, so it measures rule coverage rather than generalization. The
experiment that would settle it, blind human raters over samples the rules never saw, is
scaffolded in the repository and has not been run. Until it has, the honest claim is the
narrow one: the same input gives the same output, and the patterns the rules know get
caught every time.
