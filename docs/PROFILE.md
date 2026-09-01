# Profiles

A profile is a JSON file that tells slop-chop what to cut and what to put in its place.
Every rule the rules pass runs comes from a profile. This page is the full reference for
the format. For the order the rules run in and how each one works, see
[the engine reference](ENGINE.md).

For a friendlier personal layer of keep, prefer, and avoid lists that rides on top of a
profile, see [Your voice](VOICE.md).

## Contents

- [Pointing at a profile](#pointing-at-a-profile)
- [Precedence](#precedence)
- [Fields](#fields)
- [charReplace](#charreplace)
- [phraseReplace](#phrasereplace)
- [wordReplace](#wordreplace)
- [regexReplace](#regexreplace)
- [blockWords](#blockwords)
- [flagPatterns](#flagpatterns)
- [allow](#allow)
- [scoreWeights](#scoreweights)
- [dialect](#dialect)
- [collapseSpaces and splitSemicolons](#collapsespaces-and-splitsemicolons)
- [tone](#tone)
- [Presets](#presets)
- [Ignore directives](#ignore-directives)
- [A full example](#a-full-example)

## Pointing at a profile

Three ways, in order of who wins:

- `--profile path.json` points at a file directly.
- The nearest `.slop-chop.json` from the working directory upward is picked up on its
  own when `--profile` is not set, the way `.gitignore` works, so a repo's profile holds
  from any of its subdirectories and the nearest file wins.
- With neither, the built-in default profile is used. It targets the common tells:
  em-dashes, smart quotes, stock openers, and a starter list of buzzwords.

A profile does not have to set every field. Anything you leave out keeps its zero value, so
a file with only `blockWords` is valid and adds those words to nothing else.

## Precedence

From highest to lowest:

1. A flag, or its environment variable, for the settings a flag exposes (`--dialect`,
   `--preset`).
2. The profile file (`--profile` or a discovered `.slop-chop.json`).
3. The built-in default.

A `--preset` overlays its rules on top of the active profile. The profile always wins on a
key both set, so a preset adds without overwriting what you wrote. See
[Presets](#presets).

A project or `--profile` file extends the built-in default: your entries add to and
override the default rules, so a two-line policy means the default detection plus your
two decisions. `"standalone": true` makes the file replace the default outright. In the
extending form, boolean switches can be turned on but not off, since off is also the
unset value. Turning a default cleanup off takes a standalone profile.

## Fields

| Field             | Type                | Rewrites | What it does                                  |
| ----------------- | ------------------- | -------- | --------------------------------------------- |
| `charReplace`     | object              | yes      | Swap a literal character for another string.  |
| `phraseReplace`   | object              | yes      | Swap or delete a phrase, any casing.          |
| `wordReplace`     | object              | yes      | Swap a whole word, keeping its case.          |
| `regexReplace`    | object              | yes      | Swap on your own regular expression.          |
| `blockWords`      | array of strings    | no       | Flag a word or term but leave it in place.    |
| `blockAlways`     | array of strings    | no       | Flag a word with no proper-noun exemption.    |
| `flagPatterns`    | object              | no       | Flag a structural tell by regex, no rewrite.  |
| `allow`           | array of strings    | n/a      | Exempt a word from every rule.                |
| `scoreWeights`    | object              | n/a      | Tune how much each rule adds to the score.    |
| `dialect`         | string              | yes      | Enforce American or British spelling.         |
| `collapseSpaces`  | boolean             | yes      | Fold repeated spaces and tidy punctuation.    |
| `splitSemicolons` | boolean             | yes      | Turn a joining semicolon into two sentences.  |
| `tone`            | array of strings    | n/a      | Notes the rewrite pass feeds to the model.    |
| `standalone`      | boolean             | n/a      | Replace the default profile instead of extending it. |

## charReplace

Maps a literal substring to its replacement. Used for the character-level swaps: em-dashes,
smart quotes, ellipses.

```json
{
  "charReplace": {
    "—": ", ",
    "…": "..."
  }
}
```

The key is matched literally, so nothing inside it acts as a pattern.

## phraseReplace

Maps a phrase to its replacement, matched without regard to case. An empty replacement
deletes the phrase and restores the capital on the word that follows, so a deleted opener
leaves a clean sentence.

```json
{
  "phraseReplace": {
    "in order to": "to",
    "due to the fact that": "because",
    "in summary, ": ""
  }
}
```

A phrase whose last character is a word matches only as a whole word, so a key like `cat`
never fires inside `category`. A phrase that ends in punctuation, like the trailing comma
on a stock opener, is bounded by that punctuation. Deletion keys in the default profile
keep their trailing comma and space so the sentence reads cleanly once the opener is gone.

Use this for multi-word swaps. For a single word, [wordReplace](#wordreplace) carries the
case for you.

## wordReplace

Maps a whole word to its replacement. The match is case-insensitive and the replacement
takes on the case of what it replaced, so one entry covers every capitalization.

```json
{
  "wordReplace": {
    "utilize": "use",
    "regarding": "about"
  }
}
```

With the entry above, `utilize` becomes `use`, `Utilize` becomes `Use`, and `UTILIZE`
becomes `USE`. The match is whole-word, so `disutilize` and `utilization` are left alone.

This is the difference between a swap and a flag. `blockWords` only marks a word, because a
safe replacement for it depends on context. `wordReplace` is for the words where you have
already decided the replacement. The replacement follows the case of the source word, so it
is not the tool for forcing a fixed capitalization like an acronym.

## regexReplace

Maps a regular expression to its replacement. The pattern is used as written, so you
control the anchoring and the boundaries, and a reference like `$1` in the replacement
expands against the match.

```json
{
  "regexReplace": {
    "([0-9]+) ?%": "$1 percent",
    "\\bTODO\\b": ""
  }
}
```

The engine is [RE2](https://github.com/google/re2/wiki/Syntax), so the patterns are linear
time and there is no catch for a runaway backtrack. A pattern that can match nothing is
skipped rather than inserted between every character. A malformed pattern is an error the
moment the profile loads, naming the pattern that would not compile.

This is the escape hatch. Reach for it when a swap is not a plain word or phrase.

## blockWords

Words and terms flagged wherever they appear. They are reported but never rewritten, since
the right replacement depends on the sentence. This is your blacklist.

```json
{
  "blockWords": ["synergy", "circle back", "boil the ocean"]
}
```

A term matches on word boundaries, so a listed word is caught on its own and not inside a
longer one. A multi-word term works the same way.

## flagPatterns

Maps a rule name to a regular expression that flags its matches without rewriting them. It
catches structural tells a word list cannot, like a stock sentence shape, where the fix
depends on the whole sentence and belongs to the rewrite pass rather than a swap. The name
is what shows up in a finding, as `structural:<name>`.

```json
{
  "flagPatterns": {
    "rule-of-three": "(?i)\\b\\w+, \\w+,? and \\w+\\b",
    "as-an-ai": "(?i)\\bas an ai\\b"
  }
}
```

The engine is [RE2](https://github.com/google/re2/wiki/Syntax), so the pattern is used as
written and you control the anchoring. A malformed pattern is an error the moment the
profile loads. The default profile ships a starter set for the common cadence tells, such
as `it's not just X, it's Y` and `let's dive in`.

## allow

Words a rule must never flag or rewrite, matched against the exact text a rule matched,
without regard to case. It silences a false positive without turning off the rule that
raised it.

```json
{
  "blockWords": ["comprehensive"],
  "allow": ["comprehensive"]
}
```

The pair above is contrived on purpose to show the shape. In practice you would allow a
word the default profile flags but that you have a reason to keep in one project.

## scoreWeights

How much a finding adds to the score's tell density. A key is an exact rule name, like
`char:—` or `word:robust`, or a rule class, the part before the colon: `word`, `phrase`,
`structural`, `char`, `regex`, or `drop`. The cleanup rules with no colon in their name
share the class `tidy`. An exact name wins over its class, and both win over the built-in
defaults: structural tells count 2, typography swaps and cleanups count 0, and everything
else counts 1. Findings themselves never change, and neither does what `fix` rewrites.
Only the score moves.

```json
{
  "scoreWeights": {
    "char:—": 0,
    "word": 1.5,
    "word:delve": 0.5
  }
}
```

The profile above says em-dashes are no evidence of machine writing in this project,
buzzwords count half again as much as usual, and `delve` counts half. Set a class to `0`
to take a whole kind of finding out of the score without silencing its findings.

Repeats of one rule always decay: the second occurrence counts half the first, the third
half of that, and so on. Machine writing shows many different tells at once, so a dozen
em-dashes in a poem or one pet buzzword repeated is a habit, not a bot. The weight you
set is the starting weight of that decay.

## dialect

Enforces a spelling variant. `"american"` flags British spellings and rewrites them,
`"british"` does the reverse, and `"off"` or an empty value leaves spelling alone. The
`--dialect` flag overrides this field for a single run.

```json
{
  "dialect": "american"
}
```

The swap is a word-for-word lookup against a built-in list, not a suffix rule, so a word
that shares an ending but no dialect difference, like `size` or `advertise`, is never
touched. The match keeps its case. A word whose other-dialect spelling doubles as an
unrelated word, like `cheque` and `check` or `tyre` and `tire`, rewrites only toward
American, so British mode never turns a plain `check` into a `cheque`.

One call it makes on purpose: an `-ize` ending like `organize` is treated as American even
though British writing accepts it too, so `"british"` rewrites `organize` to `organise`. If
your house style keeps `-ize`, leave the dialect off.

## collapseSpaces and splitSemicolons

Two booleans that drive the cleanup rules.

```json
{
  "collapseSpaces": true,
  "splitSemicolons": true
}
```

`collapseSpaces` folds a run of two or more spaces into one and drops a space left in front
of punctuation, the debris a character swap leaves behind. It keeps indentation, a markdown
hard break, and the alignment padding on a table row. `splitSemicolons` turns a semicolon
that joins two clauses into a period and a capital. It leaves a semicolon that separates
list items alone.

## tone

Notes on the voice to aim for. The rules pass ignores it. The rewrite pass feeds it to the
model so the output sounds like you.

```json
{
  "tone": [
    "Plain and direct. Short sentences.",
    "Sound like a person talking, not a press release."
  ]
}
```

## Presets

A preset is a built-in profile you overlay with `--preset`. It adds a curated pack of rules
on top of your profile without your having to paste them in.

```sh
slop-chop fix --preset plain notes.md
slop-chop check --preset plain notes.md
```

Pass more than one, comma separated, and they overlay in order. Your own profile wins on
any entry it also sets, so a preset never overwrites a decision you made.

The packs that ship:

| Preset       | What it adds                                                                  |
| ------------ | ----------------------------------------------------------------------------- |
| `academic`   | Swaps for the padded verbs papers lean on, like `utilize` and `demonstrate`.  |
| `cleaver`    | Rewrites for the buzzwords the default profile only flags: `leverage` to `use`, `robust` to `solid`, `meticulously` to `carefully`, and dozens more. The strongest cut. |
| `corporate`  | A corporate-to-plain phrase and word map, plus a jargon blacklist.            |
| `marketing`  | Swaps for launch-copy words like `bespoke` and `curated`.                     |
| `no-dashes`  | Converts every dash standing in for an em-dash to a comma: ` - `, ` -- `, and the typed `word--word`. Compound words keep their hyphens. |
| `plain`      | A general jargon-to-plain map for everyday prose.                             |
| `typography` | Keeps typeset characters as they are: em- and en-dashes, curly quotes, and ellipses stay while the word, phrase, and structure rules still run. For published or professionally typeset text. |

The default profile flags words like `leverage` and `robust` but leaves them in place,
since the right swap depends on context. `cleaver` takes a position on each one and
rewrites it. With `cleaver`, `we leverage robust workflows` comes back as `we use solid
workflows`. The web app at [slop-chop.com](https://slop-chop.com/) ships with `cleaver` on.

The dash presets are the two ends of one dial. The default already turns an em-dash into
a comma and flags the spaced hyphen imitating one. `no-dashes` goes further and rewrites
every imitation too. `typography` goes the other way for text whose dashes and curly
quotes are deliberate typesetting, not model output. The same dial is available piecemeal:
put a character in your profile's `allow` list to keep it, or add your own `regexReplace`
to convert a form the presets do not cover.

## Ignore directives

Two inline comments silence a line, the way a linter pragma does. They work in `check` and
`fix` alike.

```text
a sentence with a flagged word <!-- slop-chop-ignore -->

<!-- slop-chop-ignore-next-line -->
the whole next line is left alone
this line is checked as usual
```

`slop-chop-ignore` silences the line it sits on. `slop-chop-ignore-next-line` silences the
line after it and not itself. They usually live in an HTML comment so they read as a
directive rather than prose, but any line containing the token counts.

## A full example

```json
{
  "charReplace": {
    "—": ", ",
    "…": "..."
  },
  "phraseReplace": {
    "in order to": "to",
    "in summary, ": ""
  },
  "wordReplace": {
    "utilize": "use"
  },
  "regexReplace": {
    "([0-9]+) ?%": "$1 percent"
  },
  "blockWords": ["synergy", "circle back"],
  "allow": ["bandwidth"],
  "dialect": "american",
  "collapseSpaces": true,
  "splitSemicolons": true,
  "tone": ["Plain and direct."]
}
```
