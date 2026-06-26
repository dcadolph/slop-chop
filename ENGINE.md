# How slop-chop chops the slop

This explains the engine. It covers how a profile becomes a set of rules, the order
those rules run in, and how check and fix differ.

## The shape of it

There are three pieces. A profile, a set of rules, and a sanitizer.

A profile is plain config. It lists what to swap and what to flag. A profile compiles
into an ordered list of rules. A sanitizer holds that list and runs it against text.

## From profile to rules

The profile has five kinds of entry. Each kind turns into one or more rules, and every
rule is a compiled regular expression plus a note on what to do when it matches.

Character swaps become literal-match rules. The text to match is quoted so nothing in it
acts as a regex. The em-dash, en-dash, smart quotes, and ellipsis live here.

Phrase swaps become case-insensitive rules. They match the same phrase in any casing.
The stored key includes the trailing comma and space, so removing the phrase leaves a
clean sentence behind.

Block words become word-boundary rules. The pattern wraps the word in boundary markers
so "robust" matches the word and not the inside of another word. Multi-word entries like
"blast radius" work the same way, with the space kept as part of the match.

The semicolon split is one fixed rule. It matches a semicolon followed by space and a
letter.

The space collapse is one fixed rule. It matches any run of two or more spaces.

## Rule order

Order matters, so the engine fixes it. Rules run in this sequence.

1. Character swaps.
2. Phrase removal.
3. Block-word flags.
4. Semicolon split.
5. Space collapse.

Space collapse runs last on purpose. Earlier swaps can leave a double space. Take "word
— word". The em-dash becomes a comma and a space, which leaves two spaces around the
comma. The final collapse pass tidies that up. Phrase removal can leave a stray space at
the start of a line too, and the same final pass cleans it.

Map entries inside a kind are sorted before they compile. Map order in the language is
not stable, so sorting keeps the rule list and the output the same on every run.

## Two ways to run

The sanitizer does two things with the same rules.

check scans the original text and reports every match. It does not change anything, so
the position it reports for each match is exact. Each finding carries the rule name, the
matched text, the suggested replacement when there is one, and a line and column. check
exits non-zero when it finds anything, which is what makes it useful in a CI step.

fix returns the cleaned text. It runs check first to collect the findings against the
original, then applies the rewrite rules in order. A rule that only flags, such as a
block word, is reported but never changes the text.

## Flag versus rewrite

Some rules rewrite and some only flag.

Character swaps, phrase removal, the semicolon split, and space collapse all rewrite.
The replacement is safe without knowing the surrounding text.

Block words only flag. There is no safe automatic swap for "comprehensive" or "blast
radius". The right replacement depends on the sentence, so the engine points at the word
and lets the writer decide.

## The semicolon split, in detail

The rule matches a semicolon, the spaces after it, and the first letter of the next
word. The replacement drops the semicolon, ends the clause with a period, adds a single
space, and puts the captured letter back in uppercase. So "it works; it ships" becomes
"it works. It ships".

This is the edge of what plain rules can do well. A semicolon that separates items in a
list should not become a period. Telling those two cases apart needs more than a regex,
which is the job of the optional rewrite pass.

## Positions

A finding reports a line and a column. The engine gets them from the byte offset of the
match. The line is one plus the number of newlines before the offset. The column is one
plus the number of runes between the start of the line and the offset. Counting runes,
not bytes, keeps the column right when the text holds characters wider than one byte.

## Where the rules stop

The engine is deterministic and cheap, and it handles the common tells well. It cannot
reword a sentence, judge tone, or match a chosen voice. Those need a model. The plan
keeps the rules as the default and adds the model pass behind a flag, so the cheap and
predictable path stays the one most people run.
