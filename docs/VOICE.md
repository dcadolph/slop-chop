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

## In the web app

The settings panel has a "Your voice" section with the same three lists, one entry per line.
It merges above the presets, the same way the CLI does, and rides along in the share link and
the exported profile. Nothing leaves your browser.
