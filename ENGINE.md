# How slop-chop works

A walk through the engine. How a profile turns into rules, what order those rules run in,
and how check and fix differ.

## The pieces

Three things do the work: a profile, the rules, and a sanitizer. The profile is plain
config that lists what to swap and what to flag. Compiling it produces an ordered list of
rules, and the sanitizer holds that list and runs it over your text.

## Turning a profile into rules

A profile has five kinds of entry, and each one compiles into one or more rules. Every
rule is a compiled regular expression paired with a note about what to do when it matches.

Character swaps compile to literal matches. The text gets quoted first so nothing inside
it behaves like a regex, and this is where the em-dash, en-dash, smart quotes, and
ellipsis live.

Phrase swaps compile to case-insensitive matches, so they catch the phrase however it is
capitalized. The key keeps the trailing comma and space, which means deleting the phrase
leaves a clean sentence rather than a dangling comma.

Block words compile to word-boundary matches, so "robust" matches the standalone word and
not the middle of a longer one. Multi-word entries such as "blast radius" work the same
way, with the space kept as part of what gets matched.

The semicolon split and the space collapse are each a single fixed rule. One matches a
semicolon followed by a space and a letter. The other matches any run of two or more
spaces.

## The order rules run in

The order matters, so it is fixed:

1. Character swaps
2. Phrase removal
3. Block-word flags
4. Semicolon split
5. Space collapse

Space collapse comes last for a reason. Some of the earlier swaps leave a double space
behind. Take the input "word — word". The em-dash becomes a comma and a space, which
leaves two spaces sitting around the comma, and the final pass cleans that up. Phrase
removal can leave a stray space at the start of a line, and the same pass handles that too.

Within a single kind, the entries get sorted before they compile. Map order in Go is not
stable, and sorting keeps the rule list and the output identical from one run to the next.

## check and fix

Both run the same rules but do different things with them.

check reads the original text and reports every match without touching it, so the line
and column it gives you for each match are exact. A finding carries the rule name, the
text that matched, the suggested replacement when there is one, and a position. When
check finds anything it exits non-zero, which is what lets it gate a CI step.

fix returns the cleaned text. It runs check first to gather the findings against the
original, then applies the rewriting rules in order. Rules that only flag, like block
words, show up in the findings but leave the text alone.

## Why some rules rewrite and some only flag

Character swaps, phrase removal, the semicolon split, and space collapse all rewrite,
because the replacement is safe without knowing anything about the surrounding sentence.

Block words only flag. There is no safe automatic swap for "comprehensive" or "blast
radius", since the right replacement depends on the sentence, so the tool marks the word
and leaves the call to you.

## A closer look at the semicolon split

The rule matches a semicolon, the spaces after it, and the first letter of the next word.
It drops the semicolon, ends the clause with a period, adds one space, and puts that
captured letter back as a capital, so "it works; it ships" turns into "it works. It
ships".

This is about as far as plain rules can go. A semicolon separating items in a list should
not become a period, and telling the two cases apart takes more than a regex. That is the
kind of thing the rewrite pass is for.

## Line and column numbers

Each finding reports a line and a column worked out from the byte offset of the match.
The line is one plus the number of newlines before the offset. The column is one plus the
number of runes between the start of the line and the offset. Counting runes instead of
bytes keeps the column honest when the text holds characters wider than a single byte.

## Where the rules give out

The rules pass is deterministic, cheap, and good at the common tells, but it cannot reword
a sentence, judge tone, or match a voice. That takes a model. The plan is to keep the
rules as the default and add the model pass behind a flag, so the cheap and predictable
path stays the one you reach for most.
