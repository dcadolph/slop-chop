# Your voice

A voice makes the chop sound like you. It is a small JSON file of three lists laid over your
profile and presets, so the cleaned text keeps your words, swaps buzzwords for the ones you
would pick, and flags the words you never want to see.

## The three lists

| List     | What it does                                                           | Maps to                         |
|----------|------------------------------------------------------------------------|---------------------------------|
| `keep`   | Words and phrases to never flag or cut, so your signatures survive.    | `allow`                         |
| `prefer` | Your own swap, from a word or phrase to the one you want. An empty target drops the word. | `wordReplace` / `phraseReplace` |
| `avoid`  | Your own words to flag wherever they appear.                           | `blockWords`                    |

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

## Where it lives

- The personal default is `~/.slop-chop/voice.json`. Once it exists it applies to every run.
- `--voice path.json` points at a different file for a single run.
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
without duplicates. Run it again on new samples any time; edit or prune the lines like any
other config. The rules pass ignores tone, so scores and deterministic output are unchanged,
and the rewrite's fail-closed meaning check still applies.

## In the web app

The settings panel has a "Your voice" section with the same three lists, one entry per line.
It merges above the presets, the same way the CLI does, and rides along in the share link and
the exported profile. Nothing leaves your browser.
