<p align="center">
  <img src="assets/banner.png" alt="slop-chop" width="420">
</p>

# slop-chop

Chop the slop. A text sanitizer that strips AI tells and enforces your own style.

slop-chop takes any block of text and cleans it. It removes the patterns that make
writing read as machine-generated (em-dashes, hedging, padding, filler words) and lets
each person define a style profile so docs come out in their voice, not a model's.

## Why

LLM output has a tell: em-dashes stitching clauses, semicolons and colons everywhere,
words like "comprehensive" and "leverage", padding like "In summary". Fixing it by hand
or re-prompting every time is tedious. slop-chop does it in one pass.

## How it works

Two layers, used together or alone.

1. Deterministic pass. A rule engine driven by a config profile. Character
   replacements, a word blocklist, phrase swaps, and punctuation normalization. Fast,
   free, and predictable. No model required.
2. Rewrite pass (optional). A model pass for the things rules cannot do well, like
   restructuring a sentence to drop a semicolon or matching a target voice.

## Modes

- `check` reports violations and exits non-zero. Use it in CI.
- `fix` rewrites the text in place.

## Style profiles

A profile is a config file that declares what to ban and how to replace it: banned
characters, blocked words, phrase substitutions, and tone notes. Each person keeps
their own.

## Status

Early. Scaffold only. Layer 1 first, layer 2 behind a flag once the core is proven.

## License

MIT. See [LICENSE](LICENSE).
