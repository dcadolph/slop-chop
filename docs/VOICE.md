# Your voice

A voice makes the chop sound like you. It is a small JSON file of three lists laid over your
profile and presets, so the cleaned text keeps your words, swaps buzzwords for the ones you
would pick, and flags the words you never want to see.

## The three lists

| List     | What it does                                                           | Maps to                         |
|----------|------------------------------------------------------------------------|---------------------------------|
| `keep`   | Words and phrases to never flag or cut, so your signatures survive.    | `allow`                         |
| `prefer` | Your own swap, from a word or phrase to the one you want. An empty target drops the word. | `wordReplace` / `phraseReplace` |
| `avoid`  | Your own words to flag wherever they appear, however capitalized.      | `blockAlways`                   |

A starter `~/.slop-chop/voice.json`:

```json
{
  "keep":   ["ship it", "gnarly"],
  "prefer": { "utilize": "use", "a myriad of": "a bunch of" },
  "avoid":  ["synergy", "circle back"]
}
```

With this, a preset that would swap a kept word leaves it alone, `utilize` becomes `use`
instead of whatever a preset picked, and `synergy` is flagged wherever it shows up.

## When your voice needs the profile instead

The three lists cover vocabulary. Two voice traits live in the profile file rather than
the voice file: if you write with semicolons, `"splitSemicolons": false` in a
`.slop-chop.json` keeps them. And if you want a tell to count more or less against the
score, that is the profile's `scoreWeights`. Both are one line. See
[the profile reference](PROFILE.md). Everything a keep entry protects is protected from
every rule, so `"keep": ["moreover,"]` preserves a sincere connective exactly as the
findings display it.

## Where it lives

- The personal default is `~/.slop-chop/voice.json`. Once it exists it applies to every
  run, and each run says so on stderr, since a voice changes what the rules report.
- `--voice path.json` points at a different file for a single run, and `--voice off`
  disables the voice entirely, which is what a CI script wants for reproducible output.
- A project's `.slop-chop.json` still outranks your voice, so a repo can pin its house style.

Precedence, highest to lowest: the project profile, then your voice, then a preset, then the
built-in default. Your `keep` and `prefer` win over any preset, and a project profile wins
over your voice.

## On the command line

```
slop-chop voice init      # write a starter ~/.slop-chop/voice.json
slop-chop voice show      # print the resolved voice and where it came from
slop-chop fix draft.md    # your voice applies with no extra flags
```

`voice init [path]` writes somewhere else, and `--force` overwrites an existing file.

## Learn from your own edits

`voice diff` reads a draft against the version you shipped and proposes entries from what
you changed.

```
slop-chop voice diff draft.md final.md
```

Your edits are the one voice signal a model cannot contaminate. The draft's words are its
own, but every change to them is yours, so nothing here can teach the tool to imitate
itself. A change whose two sides carry different numbers, links, or acronyms is read as a
corrected fact and ignored, which keeps "waits 5s" becoming "waits 30s" out of your style
rules while `utilize` becoming `use` stays in.

The report groups what it found three ways:

| Group      | What it means                                                            |
| ---------- | ------------------------------------------------------------------------ |
| `keep`     | The rules flagged it and you shipped it anyway, so the rules are wrong about this word. |
| `prefer`   | You changed it and no rule caught it, which is where your profile grows.  |
| `confirms` | You cut what the rules already flag, so an existing rule matches how you write. |

It proposes and never writes, and it declines to propose what cannot become a safe rule:
moves, case-only and punctuation-only edits, insertions, fact corrections, restructures
too large to read as word choice, common words left over from a rewrite, and markdown
fragments are all filtered out. A word you edited two different ways is reported as a
conflict instead of silently resolved. Merge the entries you agree with into your voice
file by hand, and let a habit show up more than once before you promote it: a single
edit is a mood, not a rule.

Add `--json` for the full candidate list plus the suggested voice, for wiring into
something else.

## Measure how you write

The three lists hold your words. A model that reads them can hand them straight back, and
a voice built only from words has that hole in it. So slop-chop also measures the part of
your writing nobody quotes: sentence rhythm, punctuation habits, and register, as numbers.

```
slop-chop voice fingerprint posts/*.md notes.md
```

That reads your own finished writing and stores eleven measurements in your voice file.

```
  sentence-length       13.71  give or take 4.00  words per sentence
  sentence-variation     0.72  give or take 0.18  spread of sentence length
  commas                 0.93  give or take 0.50  commas per sentence
  contractions           0.00  give or take 1.50  contractions per 100 words
  long-words             6.22  give or take 4.00  percent of words over eight letters
```

The give-or-take is the band, how far a text may sit from the measurement before the
difference means anything. Each band starts wide enough to absorb the ordinary variation
between two things one person wrote, and widens further when your own samples disagree, so
a habit that swings from piece to piece is not read as drift later.

Feed it your finished writing, several pieces of it, and no machine drafts. A fingerprint
taken from a model's prose measures the model. Code, headings, tables, front matter, and
list markers are stripped before anything is counted, so only prose you composed reaches
the numbers.

## Find the paragraphs that stopped sounding like you

```
slop-chop voice drift draft.md
```

```
draft.md reads unlike you on 4 of 11 traits
  a heavier vocabulary                         48.57 against your 6.02 (percent of words over eight letters)
  a flatter sentence rhythm                    0.41 against your 0.70 (spread of sentence length)
  fewer lowercase sentence openings            0.00 against your 24.42 (percent of sentences)
  longer sentences                             20.00 against your 14.02 (words per sentence)
```

Drift is not slop. A trait outside your range can be good writing that belongs to somebody
else, which is exactly what a model hands you. The report names the difference and leaves
the verdict to you, which is why nothing here is rewritten and no score changes.

`--bands 2` fails the run when a trait lands more than two bands out, so a team can gate
its own house voice in CI the way `score --max` gates slop. `--json` gives the same report
as data.

## Teach it your voice

The three lists shape the deterministic pass. A fourth, `tone`, shapes the optional model
rewrite: short notes on how you write, sent to the model as "Match this voice" whenever
`fix --rewrite` runs. Write them by hand:

```json
{ "tone": ["short, blunt sentences", "dry humor, no hype"] }
```

Or derive them from your own writing:

```
slop-chop voice learn notes.md posts/*.md
cat draft.md | slop-chop voice learn
```

`learn` sends the samples to the configured model (the same provider setup as
`fix --rewrite`), gets back a handful of tone notes, and merges them into your voice file
without duplicates. Run it again on new samples any time. Edit or prune the lines like any
other config. The rules pass ignores tone, so scores and deterministic output are unchanged,
and the rewrite's fail-closed meaning check still applies.

## In the web app

The settings panel has a "Your voice" section with the same three lists, one entry per line.
It merges above the presets, the same way the CLI does, and rides along in the share link and
the exported profile. Nothing leaves your browser.
