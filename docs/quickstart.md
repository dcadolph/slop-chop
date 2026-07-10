# Quickstart

Zero to a cleaned file in a couple of minutes.

## Install

Homebrew:

```sh
brew install dcadolph/tap/slop-chop
```

With Go:

```sh
go install github.com/dcadolph/slop-chop@latest
```

Confirm it is on your `PATH`:

```sh
slop-chop --version
```

## Clean some text

Print the cleaned text to stdout. Your file is not changed:

```sh
slop-chop fix notes.md
```

Clean the file in place, like `gofmt -w`:

```sh
slop-chop fix -w notes.md
```

Pipe text through it:

```sh
echo "In summary, a robust—and seamless—result." | slop-chop fix
```

## Flag without changing

`check` flags what it finds and exits non-zero, so it drops straight into CI:

```sh
slop-chop check notes.md
slop-chop check --json notes.md
```

## Score it

`score` rates the text from 0 for clean to 100 for heavy slop:

```sh
slop-chop score notes.md          # prints a number like 42
slop-chop score --max 20 notes.md # exit non-zero when over the bar
```

## Enforce a dialect

Flag or fix the other spelling variant:

```sh
slop-chop check --dialect american notes.md
slop-chop fix --dialect british notes.md
```

## Deeper clean

The rules pass is free and default. Add `--rewrite` to hand the result to a model for the
work rules cannot do. It needs `ANTHROPIC_API_KEY` and costs money, so it stays off unless
you ask for it:

```sh
export ANTHROPIC_API_KEY=sk-...
slop-chop fix --rewrite notes.md
slop-chop fix --rewrite --verify notes.md
```

## Next

- [Profiles](PROFILE.md): say what to cut and what to put in its place.
- [Claude plugin](PLUGIN.md): run it from Claude Code with a skill and a command.
