<p align="center">
  <img src="assets/banner.png" alt="slop-chop" width="420">
</p>

# slop-chop

Chop the slop. Paste in text and get back something that reads like a person wrote it.

AI writing leaves fingerprints. It runs on em-dashes, drops a semicolon into every other
sentence, reaches for words like "comprehensive" and "substrate," and clears its throat
with openers like "In summary" or "Giving it to you honestly." slop-chop pulls all of
that out in a single pass. You can also hand it your own list of things to cut, so the
result reads like you instead of a chatbot.

## Why

Cleaning this up by hand is tedious, and asking the model to "stop using em-dashes" holds
for about three sentences before it forgets. slop-chop just takes the text and cleans it,
the same way every time.

## How it works

There are two passes, and you can run either one on its own.

The first is a rules pass. It is fast and deterministic. It swaps characters, drops words
you have flagged, rewrites stock phrases, and tidies the punctuation, with no model, no
cost, and the same output on every run.

The second is an optional rewrite pass that hands the text to a model for the things
rules cannot manage, like reworking a sentence so it no longer needs a semicolon, or
nudging the writing toward a voice you picked.

[ENGINE.md](ENGINE.md) has the details if you want them.

## Usage

```sh
# Clean a file and print the result
slop-chop fix notes.md

# Pipe text through it
echo "In summary, a robust—and seamless—result." | slop-chop fix

# Flag slop without changing anything (exits non-zero if it finds any)
slop-chop check notes.md

# Use your own profile
slop-chop fix -profile myprofile.json notes.md

# Get findings as JSON for other tools to read
slop-chop check -json notes.md
slop-chop check -json -pretty notes.md

# Get the cleaned text and the findings together
slop-chop fix -json notes.md
```

`check -json` prints a `{"findings": [...]}` object to stdout, and `fix -json` adds the
cleaned text as `{"cleaned": "...", "findings": [...]}`. Each finding carries the rule,
the matched text, the suggested replacement, and a line and column.

## Modes

- `check` flags what it finds and exits non-zero. Drop it in CI.
- `fix` cleans the text in place.

## Use it in CI

Add a workflow that fails a pull request when it finds slop:

```yaml
name: slop-chop
on: pull_request
jobs:
  slop:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dcadolph/slop-chop@v0.3.0
        with:
          files: docs/intro.md docs/guide.md
          # profile: myprofile.json   # optional
```

## Style profiles

A profile is a small config file that lists what to cut and what to put in its place:
characters, words, phrases, and a couple of notes on tone. Keep your own and point the
tool at it.

## Status

Still early. The rules pass is built and working. The rewrite pass is not done yet. It
will come later and sit behind a flag, because it needs an API key and costs money, and
the free, predictable path should stay the default.

## License

MIT. See [LICENSE](LICENSE).
