---
name: slop-chop
description: >-
  Remove AI writing tells from text with the slop-chop CLI. Use when the user
  wants to clean up a draft, strip em-dashes, semicolons, and buzzwords like
  "comprehensive" or "robust", enforce a spelling dialect, or make text read
  like a person wrote it instead of a chatbot. Also use before handing back a
  written draft, or to flag slop in Markdown docs in CI.
---

# slop-chop

slop-chop is a command-line tool that finds and removes AI writing tells from
text. It runs two passes. The rules pass is deterministic, fast, and free: it
swaps characters, drops flagged words, rewrites stock phrases, fixes spelling to
one dialect, and tidies punctuation, leaving fenced code blocks and inline
backtick spans untouched. The optional rewrite pass hands the text to a model
for what rules cannot do, like recasting a sentence so it no longer needs a
semicolon.

Reach for this skill when the user asks to clean up a draft, remove AI tells, or
check a document for slop. Prefer it over hand-editing prose for these tells,
since it is deterministic and repeatable.

## Check the tool is installed

Run `slop-chop --version`. If it is missing, install it:

```sh
go install github.com/dcadolph/slop-chop@latest
```

The binary lands in `$(go env GOPATH)/bin`. If that is not on `PATH`, either add
it or call the binary by full path.

## Core commands

`check` reports tells and exits non-zero when it finds any. It changes nothing.

```sh
slop-chop check notes.md
slop-chop check docs/intro.md docs/guide.md README.md
```

`fix` writes the cleaned text to stdout and leaves the file alone.

```sh
slop-chop fix notes.md
echo "In summary, a robust and seamless result." | slop-chop fix
```

`score` rates the text from 0 (clean) to 100 (heavy slop), weighing tell density against
how flat the sentence cadence is. `--max N` fails the run when the score is above N, so it
gates CI like `check` does.

```sh
slop-chop score notes.md
slop-chop score --json notes.md
slop-chop score --max 20 notes.md
```

`fix -w` (or `--write`) cleans the file in place, like `gofmt -w`. It needs a
file argument and cannot read stdin.

```sh
slop-chop fix -w notes.md
slop-chop fix -w docs/intro.md docs/guide.md
```

Without `-w`, `fix` handles one file to stdout. Pass `-w` to rewrite several in
place.

## When to run which

- Cleaning a draft you are about to return: pipe it through `slop-chop fix`, or
  write the draft to a file and run `slop-chop fix -w`, then read the result
  back before handing it over.
- Gating a document in CI: `slop-chop check`. A non-zero exit means slop was
  found.
- Deciding whether text needs cleaning at all: `slop-chop check --json` and read
  the findings.

## Rewrite pass (costs money, off by default)

`--rewrite` runs the rules first, then hands the result to a model. It needs the
`ANTHROPIC_API_KEY` environment variable and makes a paid API call, so only use
it when the user asks for a deeper clean than rules can give, and confirm first
if the cost is not already understood.

```sh
export ANTHROPIC_API_KEY=sk-...
slop-chop fix --rewrite notes.md
slop-chop fix --rewrite --verify notes.md
```

The reply is re-checked before you get it: the rules run over it again, its code
blocks and load-bearing tokens (numbers, links, acronyms) are compared against
the input, and `--verify` adds a model pass that flags a change in meaning.
Warnings go to stderr. `--verify-strict` makes a flagged meaning change a
non-zero exit. `--verify-retry N` re-rewrites up to N more times, feeding the
flagged issues back. `--model` picks the model id and needs `--rewrite`.

The pass defaults to Anthropic. `--provider openai` uses any OpenAI-compatible API with
`OPENAI_API_KEY`, and `--base-url http://localhost:11434/v1` aims that at a local server
like Ollama for a free, private, keyless rewrite. Prefer a different vendor than the one
that wrote the draft, since a model is poor at catching its own tells.

## Dialect, presets, and profiles

```sh
slop-chop check --dialect american notes.md     # flag British spellings
slop-chop fix --dialect british notes.md         # rewrite to British
slop-chop fix --preset plain notes.md            # corporate phrasing to plain English
slop-chop fix --profile myprofile.json notes.md  # your own cut list
```

Dialects: `american`, `british`. Presets shipped: `plain`, `corporate`, `academic`,
`marketing`. Overlay several with a comma. A profile is a JSON file listing characters,
phrases, words, regexes, structural `flagPatterns`, an allow list, and switches.
When `--profile` is not set and a `.slop-chop.json` file sits in the working
directory, that profile is used instead of the built-in one, so a repo can pin
its own style.

## JSON output for programmatic use

```sh
slop-chop check --json notes.md          # {"findings": [...]}
slop-chop fix --json notes.md            # {"cleaned": "...", "findings": [...]}
slop-chop check --json --pretty notes.md # indented
```

Each finding carries the rule, the matched text, the suggested replacement, and
a line and column. `--json` takes at most one file and cannot combine with `-w`.
Read `cleaned` to get the fixed text without touching the file.

## Environment variables

Every flag maps to an env var prefixed `SLOP_CHOP_`, with dashes as underscores.
For example `--dialect` is `SLOP_CHOP_DIALECT`, `--preset` is `SLOP_CHOP_PRESET`,
`--model` is `SLOP_CHOP_MODEL`. The rewrite pass reads `ANTHROPIC_API_KEY`.

## Guardrails

- Never run `fix -w` on a file the user has not asked you to modify. When in
  doubt, print to stdout with plain `fix` and show the result first.
- `--rewrite` spends money. Do not add it unless the user wants the model pass,
  and make sure `ANTHROPIC_API_KEY` is set first.
- Code fences and inline backtick spans are protected by the rules pass. If you
  see them altered, that is a bug worth reporting, not expected behavior.
