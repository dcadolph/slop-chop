---
description: Clean AI writing tells from a file or pasted text with slop-chop
argument-hint: "[file] | paste text after the command"
---

# slop-chop

Clean AI writing tells out of the target using the `slop-chop` CLI. The argument
is either a file path or the text to clean: `$ARGUMENTS`.

Steps:

1. Confirm the CLI is available with `slop-chop --version`. If it is missing,
   tell the user to install it with `go install github.com/dcadolph/slop-chop@latest`
   and stop.
2. If the argument is a path to an existing file, run `slop-chop fix <path>` and
   show the cleaned output. Do not pass `-w` unless the user asked to change the
   file in place.
3. If the argument is text rather than a path, pipe it in:
   `printf %s "<text>" | slop-chop fix`.
4. If there is no argument, ask the user for the file or text to clean.

Notes:

- The plain rules pass is deterministic and free. Do not add `--rewrite` unless
  the user asks for the model pass, since it needs `ANTHROPIC_API_KEY` and makes
  a paid API call.
- Use `slop-chop check` instead of `fix` when the user only wants to see the
  tells without changing anything. It exits non-zero when it finds any.
- Pass `--dialect american|british` or `--preset plain` through when the user
  names a spelling variant or wants corporate phrasing flattened.

See the slop-chop skill for the full flag reference.
