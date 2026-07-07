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
you have flagged, rewrites stock phrases and words, runs your own patterns, fixes spelling
to one dialect, and tidies the punctuation, with no model, no cost, and the same output on
every run. It knows markdown, so fenced code blocks and inline backtick spans come through
untouched.

The second is an optional rewrite pass that hands the text to a model for the things
rules cannot manage, like reworking a sentence so it no longer needs a semicolon, or
nudging the writing toward a voice you picked.

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

# Check or fix several files at once
slop-chop check docs/intro.md docs/guide.md README.md
slop-chop fix -w docs/intro.md docs/guide.md

# Enforce a spelling variant: flag or fix the other dialect
slop-chop check --dialect american notes.md
slop-chop fix --dialect british notes.md

# Overlay a built-in pack, like corporate phrasing to plain English
slop-chop fix --preset plain notes.md

# Use your own profile
slop-chop fix --profile myprofile.json notes.md

# Get findings as JSON for other tools to read
slop-chop check --json notes.md
slop-chop check --json --pretty notes.md

# Deeper clean: rules first, then a model rewrite (needs ANTHROPIC_API_KEY)
slop-chop fix --rewrite notes.md
slop-chop fix --rewrite --verify notes.md
```

`check --json` prints a `{"findings": [...]}` object to stdout, and `fix --json` adds the
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
      - uses: dcadolph/slop-chop@v0.9.0
        with:
          files: docs/intro.md docs/guide.md
          # profile: myprofile.json   # optional
          # dialect: american         # optional
          # preset: plain             # optional
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
      - uses: dcadolph/slop-chop@v0.9.0
        with:
          files: docs/intro.md docs/guide.md
          mode: fix
          commit: "true"
          # message: Chop the slop   # optional commit message
```

## Profiles and presets

A profile is a JSON file that lists what to cut and what to put in its place: characters,
phrases, words, regular expressions, a blacklist, and a few switches. Point the tool at one
with `--profile`, or drop a `.slop-chop.json` in the directory you run from and it gets
picked up on its own. With neither, a built-in default runs.

Presets are curated packs you overlay with `--preset`. `--preset plain` turns corporate
phrasing into plain English on top of whatever profile you already have.

[docs/PROFILE.md](docs/PROFILE.md) is the full reference: every field, the presets, the
spelling dialects, the allow list, and the inline ignore directives.

## Rewrite pass (optional)

The rules pass is deterministic and free. For the work rules cannot do, like reworking a
sentence so it no longer needs a semicolon or bending the text toward your voice, add
`--rewrite`. It runs the rules first, then hands the result to a model. It needs
`ANTHROPIC_API_KEY` and costs money, so it stays off by default.

```sh
export ANTHROPIC_API_KEY=sk-...
slop-chop fix --rewrite notes.md
slop-chop fix --rewrite --verify notes.md
```

The reply is checked before you get it. The rules run over it again, its code blocks and
load-bearing tokens are compared against your input, and `--verify` adds a model pass that
flags a change in meaning. [ENGINE.md](ENGINE.md) covers the rewrite and its checks in
full.

## Docs

- [docs/PROFILE.md](docs/PROFILE.md) is the profile and preset reference.
- [ENGINE.md](ENGINE.md) is how the engine works: the rule kinds, the order they run in,
  and the rewrite pass in detail.

## Status

Still early, but the core is in place. The rules pass is built and working. The rewrite
pass is built too and sits behind the `--rewrite` flag, because it needs an API key and
costs money, so the free, predictable rules pass stays the default. The one part not yet
exercised is a live rewrite run against the real API.

## License

MIT. See [LICENSE](LICENSE).
