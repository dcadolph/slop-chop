# What slop-chop is, on the evidence

Opened 2026-09-16. A decision record, not a decision. Three reviewers, reading the
repository independently, each arrived at the same recommendation: stop calling this an
AI detector and start calling it a prose linter. That is a judgment about identity and it
belongs to the owner. What this file adds is the part that is not a judgment, which is
what the engine's own measurements already say.

## The claim on the tin

The README opens on "AI writing leaves fingerprints" and offers to give back something
"that reads like a person wrote it." The engine underneath does something narrower and
more defensible: it finds patterns a profile lists and reports them, deterministically,
with no model and no network.

Those are not the same promise. The first is a claim about authorship, which the docs
then spend paragraphs walking back. The second is a claim about pattern density, which
the engine actually delivers and the benchmark actually measures.

## What the measurements say

Three of them, none of which is an opinion about positioning.

**The word list is a depreciating asset.** `slop-chop attack` rewrites text to dodge the
rules without improving it, then reports what survived. By class: 35 of 76 word tells and
8 of 12 phrase tells fall to a thesaurus. 2 of 66 structural tells do. A lookup loses to
a lookup. A sentence shape has to be rebuilt.

**The engine is already quiet on writing it has never seen.** 1270 README files from
repositories with no push after 2021, across seven language ecosystems, scored once: no
false positives, worst sample four points under the reads-clean line. The rules that do
fire on human prose are the lexical ones, and none reaches seven percent of documents.

**The score does not yet claim what people think it claims.** The locked corpus has never
been rated, so there is no evidence the number tracks what a reader perceives. Until
there is, the honest description of a score is pattern density under a named profile, not
a verdict about who wrote something.

## What follows, and what does not

What follows is narrow. The durable half of the engine is the structural half, and the
public wording leans on the half that erodes. A page that led with determinism, the
profile system, and the structural shapes would be describing the part that survives
contact with a model trying to evade it.

What does not follow is a rewrite of the product. The word list still earns its place: it
is what makes the tool useful on the first run, before anyone writes a profile. The
finding is that it should not be the headline, not that it should go.

Nor does it follow that the AI framing is wrong. It is the reason the default profile
exists and the reason anyone arrives. The narrower reading is that "these patterns
currently correlate with AI-assisted writing" is a claim the repository can defend, and
"this text was written by AI" is not.

## The three options

Leave it as it stands. The docs already disclaim the strong claim in several places. The
cost is that the strongest evidence in the repository, the attack asymmetry and the 1270
documents, stays behind a framing that invites a different argument.

Reframe the wording and keep the product. Lead on deterministic, local, programmable, and
keep the AI-tells profile as the default and the reason to show up. Costs an afternoon of
copy and nothing else.

Reframe the project. Make the profile system the center, ship the AI tells as one profile
among several, and rename the score to something no reader can take for a probability.
Costs real work and changes what the project is.

The measurements support the second more than the first and do not by themselves require
the third. Nothing here is a recommendation about which to take.
