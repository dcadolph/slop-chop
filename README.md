<p align="center">
  <img src="assets/banner.png" alt="slop-chop" width="420">
</p>

<p align="center">
  <a href="https://slop-chop.com"><img src="https://img.shields.io/badge/try_it-slop--chop.com-9bcf1a" alt="Try it at slop-chop.com"></a>
  <a href="https://github.com/dcadolph/slop-chop/releases/latest"><img src="https://img.shields.io/github/v/release/dcadolph/slop-chop?color=9bcf1a" alt="Latest release"></a>
  <a href="https://github.com/dcadolph/slop-chop/actions/workflows/ci.yml"><img src="https://github.com/dcadolph/slop-chop/actions/workflows/ci.yml/badge.svg" alt="ci status"></a>
</p>

# slop-chop

Chop the slop. Paste in text and get back something that reads like a person wrote it.

Try it without installing anything: [slop-chop.com](https://slop-chop.com/) runs the same
engine in your browser.

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

Homebrew:

```sh
brew install dcadolph/tap/slop-chop
```

With Go:

```sh
go install github.com/dcadolph/slop-chop@latest
```

Or clone and use the Makefile:

```sh
git clone git@github.com:dcadolph/slop-chop.git
cd slop-chop
make install     # build and install into $(go env GOPATH)/bin, version stamped
make uninstall   # remove it again
```

Run `make` with no target for the full list (`build`, `test`, `cover`, `lint`, `fmt`, `tidy`, `clean`).

## Everywhere else

The same engine runs on many surfaces. All local and free unless noted.

| Where | Get it |
| --- | --- |
| Web app | [slop-chop.com](https://slop-chop.com), nothing to install |
| VS Code, Cursor, VSCodium | search **slop-chop** on [Open VSX](https://open-vsx.org/extension/dcadolph/slop-chop) |
| JetBrains IDEs | the Marketplace plugin, or LSP4IJ with `slop-chop lsp`, see [docs/LSP.md](docs/LSP.md) |
| Neovim, Helix, any LSP editor | `slop-chop lsp`, see [docs/LSP.md](docs/LSP.md) |
| Chrome, Edge, Firefox | the browser extension, see [docs/EXTENSION.md](docs/EXTENSION.md) |
| Obsidian | the desktop plugin, see [obsidian/](obsidian/) |
| Node | `npm install slop-chop-wasm` |
| HTTP API | `POST https://api.slop-chop.com/chop`, see [docs/API.md](docs/API.md) |
| CI, Raycast, macOS, pre-commit | the GitHub Action and [integrations/](integrations/) |

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
- `score` rates the text from 0 to 100 on how much it reads like AI wrote it.

## Score

`score` gives a single number from 0 for clean to 100 for heavy slop. It weighs the density
of rule tells against how flat the sentence cadence is, since an even, machine-like rhythm
is a tell no word list catches.

```sh
slop-chop score notes.md            # prints a number like 42
slop-chop score --json notes.md     # {"value":42,"tells":7,"words":210,...}
slop-chop score --max 20 notes.md   # exit non-zero when the score is above 20
```

`--max` turns it into a gate, so a document over the bar fails a build the same way `check`
does.

## Structural tells

Word swaps catch the vocabulary of AI writing. The rules pass also flags a few structural
tells that a word list misses, like the `it's not just X, it's Y` and `not only X but also
Y` cadence, the `let's dive in` opener, and `here's the thing` throat-clearing. These are
flagged, not rewritten, since the fix depends on the whole sentence and is left to the
rewrite pass. Add your own with the `flagPatterns` field in a profile.

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
      - uses: dcadolph/slop-chop@v0.9.1
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
      - uses: dcadolph/slop-chop@v0.9.1
        with:
          files: docs/intro.md docs/guide.md
          mode: fix
          commit: "true"
          # message: Chop the slop   # optional commit message
```

The fix-and-commit workflow pushes back to the pull request branch, so it works for pull
requests from a branch in the same repository. A pull request from a fork gets a read-only
token, so the push fails with a 403 no matter the `permissions` block. For fork pull requests
use the check workflow above: it needs no write access and still fails the run on slop. To
auto-fix your own branches, run the same fix-and-commit job on a `push` trigger instead.

## Use it as a Claude Code plugin

slop-chop ships a Claude Code plugin, so the assistant can run the tool for you. The repo is
its own marketplace. Add the marketplace, then install the plugin from it:

```
/plugin marketplace add dcadolph/slop-chop
/plugin install slop-chop@slop-chop
```

The `slop-chop@slop-chop` name is `plugin@marketplace`: the plugin named slop-chop, from the
marketplace named slop-chop.

The plugin drives the `slop-chop` binary rather than replacing it, so install the binary and
put it on your `PATH` first:

```sh
go install github.com/dcadolph/slop-chop@latest   # lands in $(go env GOPATH)/bin
slop-chop --version                                # confirm it is on PATH
```

The plugin then gives Claude two ways to reach the tool:

| Way            | What it is                                        | You do                                  |
| -------------- | ------------------------------------------------- | --------------------------------------- |
| `slop-chop` skill  | Claude picks it up on its own for a draft.    | Hand it text and ask for a clean.       |
| `/slop-chop` command | A command you invoke on a file or text.     | Type `/slop-chop notes.md`.             |

```
# Let the skill decide
Clean the slop out of this before I send it: <paste your text>

# Or call the command
/slop-chop notes.md
```

The rules pass is free. The rewrite pass needs a key and stays off unless you ask for it, so
say when you want the deeper clean, and name a backend if you want a local, keyless one:

```
Rewrite this to sound human, and use my local Ollama so it costs nothing.
```

[docs/PLUGIN.md](docs/PLUGIN.md) is the full plugin guide, including troubleshooting.

## Profiles and presets

A profile is a JSON file that lists what to cut and what to put in its place: characters,
phrases, words, regular expressions, a blacklist, and a few switches. Point the tool at one
with `--profile`, or drop a `.slop-chop.json` in the directory you run from and it gets
picked up on its own. With neither, a built-in default runs.

Presets are curated packs you overlay with `--preset`. The built-in packs are `plain`,
`corporate`, `academic`, and `marketing`. `--preset plain` turns corporate phrasing into
plain English on top of whatever profile you already have, and the others target the stock
phrasing of their own worlds. Overlay more than one with a comma: `--preset corporate,plain`.

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

### Backends

The rewrite pass defaults to Anthropic, but `--provider openai` points it at any
OpenAI-compatible Chat Completions API using `OPENAI_API_KEY`. With `--base-url` you can
aim that at a local server, so the rewrite runs on your own machine with no key and no cost.

```sh
# OpenAI
OPENAI_API_KEY=sk-... slop-chop fix --rewrite --provider openai --model gpt-4o notes.md

# Local Ollama, no key, no bill
slop-chop fix --rewrite --provider openai --base-url http://localhost:11434/v1 \
  --model llama3.1 notes.md
```

Using a different vendor to rewrite than the one that wrote the draft is a good idea, since
a model is bad at spotting its own tics.

## Docs

- [docs/PLUGIN.md](docs/PLUGIN.md) is the Claude Code plugin guide: install, the skill, the
  command, backends, and troubleshooting.
- [docs/PROFILE.md](docs/PROFILE.md) is the profile and preset reference.
- [ENGINE.md](ENGINE.md) is how the engine works: the rule kinds, the order they run in,
  and the rewrite pass in detail.

## Status

Still early, but the core is in place. The rules pass is built and working. The rewrite
pass is built too and sits behind the `--rewrite` flag, because it needs an API key and
costs money, so the free, predictable rules pass stays the default. The live rewrite path
has a key-gated integration test, kept out of the default build so it never spends money by
accident. Run it against the real API with an API key:

```sh
ANTHROPIC_API_KEY=sk-... go test -tags=integration ./internal/rewrite/ -run Live -v
```

## License

MIT. See [LICENSE](LICENSE).
