<p align="center">
  <img src="assets/banner.png" alt="slop-chop" width="420">
</p>

# slop-chop

Chop the slop. Paste in text, get back text that sounds like a person wrote it.

You know the tells. Em-dashes splattered everywhere. A semicolon in every other
sentence. Words like "comprehensive" and "leverage." Openers like "In summary."
slop-chop strips all that out in one pass. You can also teach it your own style, so
your docs come out sounding like you and not like a chatbot.

## Why

Cleaning up AI text by hand gets old fast, and re-prompting the model to "stop using
em-dashes" never quite sticks. slop-chop just does it. Run your text through once and
the slop is gone.

## How it works

Two passes. Use one or both.

1. Rules. A fast pass that swaps characters, drops banned words, rewrites stock
   phrases, and tidies punctuation. No model, no cost, same result every time.
2. Rewrite (optional). A model handles the stuff rules can't, like reworking a
   sentence to lose a semicolon or bending text toward a voice you picked.

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
```

## Modes

- `check` flags what it finds and exits non-zero. Drop it in CI.
- `fix` cleans the text in place.

## Style profiles

A profile is a small config file. It says what to ban and what to swap in: characters,
words, phrases, and a few notes on tone. Everyone keeps their own.

## Status

Early days. Rules pass first. The rewrite pass comes later, behind a flag, once the
core earns it.

## License

MIT. See [LICENSE](LICENSE).
