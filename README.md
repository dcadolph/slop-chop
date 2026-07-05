<p align="center">
  <img src="assets/banner.png" alt="slop-chop" width="420">
</p>

# slop-chop

Chop the slop. Paste in text and get back something that reads like a person wrote it.

AI writing leaves fingerprints. It runs on em-dashes, drops a semicolon into every other
sentence, reaches for words like `comprehensive` and `substrate`, and clears its throat
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
cost, and the same output on every run. It knows markdown, so fenced code blocks and
inline backtick spans come through untouched.

The second is an optional rewrite pass that hands the text to a model for the things
rules cannot manage, like reworking a sentence so it no longer needs a semicolon, or
nudging the writing toward a voice you picked.

[ENGINE.md](ENGINE.md) has the details if you want them.

## Install

```sh
go install github.com/dcadolph/slop-chop@latest
```

Or clone and build:

```sh
git clone git@github.com:dcadolph/slop-chop.git
cd slop-chop
go install .
```

## Usage

```sh
# Print the cleaned text to stdout. Your file is not changed.
slop-chop fix notes.md

# Clean the file in place, like gofmt -w.
slop-chop fix -w notes.md

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

# Deeper clean: rules first, then a model rewrite (needs ANTHROPIC_API_KEY)
slop-chop fix -rewrite notes.md
slop-chop fix -rewrite -model claude-sonnet-4-6 notes.md
```

`check -json` prints a `{"findings": [...]}` object to stdout, and `fix -json` adds the
cleaned text as `{"cleaned": "...", "findings": [...]}`. Each finding carries the rule,
the matched text, the suggested replacement, and a line and column.

## Modes

- `check` flags what it finds and exits non-zero. Drop it in CI.
- `fix` writes the cleaned text to stdout and leaves your file alone. Pass `-w` to change
  the file in place instead.

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

Or have it fix the files and push the cleanup back to the pull request branch:

```yaml
name: slop-chop
on: pull_request
jobs:
  slop:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
      - uses: dcadolph/slop-chop@v0.3.0
        with:
          files: docs/intro.md docs/guide.md
          mode: fix
          commit: "true"
          # message: Chop the slop   # optional commit message
```

## Rewrite pass (optional)

The rules pass is deterministic and free. For the work rules cannot do, like reworking a
sentence so it no longer needs a semicolon or bending the text toward your voice, add
`-rewrite`. It runs the rules first, then hands the result to a model.

```sh
export ANTHROPIC_API_KEY=sk-...
slop-chop fix -rewrite notes.md
slop-chop fix -rewrite -model claude-sonnet-4-6 notes.md
```

It defaults to Claude Opus 4.8. Set the voice it aims for with the `tone` list in your
profile. This pass costs money and the output varies from run to run, so the rules pass
stays the default.

## Style profiles

A profile is a small config file that lists what to cut and what to put in its place:
characters, words, phrases, and a couple of notes on tone. Keep your own and point the
tool at it.

## Status

Still early, but the core is in place. The rules pass is built and working. The rewrite
pass is built too and sits behind the `-rewrite` flag, because it needs an API key and
costs money, so the free, predictable rules pass stays the default. The one part not yet
exercised is a live rewrite run against the real API.

## License

MIT. See [LICENSE](LICENSE).
