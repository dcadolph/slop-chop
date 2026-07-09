# slop-chop as a Claude Code plugin

slop-chop ships a Claude Code plugin, so the assistant can run the tool while you write. The
plugin does not bundle its own copy of slop-chop. It calls the `slop-chop` binary on your
`PATH`, so the same rules, profiles, and presets you use on the command line apply inside
Claude.

## What the plugin gives you

| Piece              | Kind            | When it runs                                          |
| ------------------ | --------------- | ----------------------------------------------------- |
| `slop-chop` skill  | Skill           | Claude reaches for it on its own when you hand it a draft to clean. |
| `/slop-chop`       | Slash command   | You invoke it directly on a file or on pasted text.   |

The skill is the hands-off path. The command is the on-demand path. Both end up running the
same binary.

## Prerequisites

The plugin needs the binary installed and reachable:

```sh
go install github.com/dcadolph/slop-chop@latest   # lands in $(go env GOPATH)/bin
slop-chop --version                                # confirm it is on PATH
```

If `slop-chop --version` prints a version, the plugin can find it. If the shell cannot find
the command, see [Troubleshooting](#troubleshooting).

The rewrite pass is the only part that needs a key. The rules pass runs offline and free.

## Install

The repo doubles as a plugin marketplace, so you add the marketplace and then install the
plugin from it:

```
/plugin marketplace add dcadolph/slop-chop
/plugin install slop-chop@slop-chop
```

The `slop-chop@slop-chop` name reads as `plugin@marketplace`: the plugin named slop-chop,
from the marketplace named slop-chop. They share a name because the repo holds one plugin.

## The skill

The skill teaches Claude when the tool fits and how to call it. You do not invoke a skill by
name. Claude reads the request and reaches for the skill when it matches, such as when you
ask it to clean a draft or strip the tells out of some text.

```
Here is my release note. Take the slop out before I post it:

<paste your text>
```

Claude runs `slop-chop fix` on the text and hands back the cleaned version. It leaves your
files alone unless you ask it to write the result back.

## The command

The `/slop-chop` command is the direct path. Point it at a file or paste the text after it:

```
/slop-chop notes.md
/slop-chop In summary, a result that ships.
```

The command runs the rules pass and shows the cleaned output. It does not change your file
unless you tell it to write in place.

## Backends

The plugin honors the same backends as the CLI. Ask for the rewrite pass only when you want
the paid, deeper clean.

| Backend            | How to ask                                          | Key                 |
| ------------------ | --------------------------------------------------- | ------------------- |
| Rules only         | The default. Just ask for a clean.                  | None.               |
| Anthropic rewrite  | Ask for a rewrite to sound human.                   | `ANTHROPIC_API_KEY` |
| OpenAI rewrite     | Ask for a rewrite with the OpenAI backend.          | `OPENAI_API_KEY`    |
| Local rewrite      | Ask for a rewrite through your local Ollama.        | None.               |

```
Rewrite this to read like a person, and use my local Ollama so it costs nothing.
```

## Troubleshooting

| Symptom                                   | Cause                                    | Fix                                                        |
| ----------------------------------------- | ---------------------------------------- | ---------------------------------------------------------- |
| Claude says the command is not found      | The binary is not on `PATH`.             | Add `$(go env GOPATH)/bin` to `PATH`, then reopen the shell. |
| The rewrite pass errors on a missing key  | No key set for the chosen backend.       | Export `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`, or use the local backend. |
| The plugin does not show up               | The marketplace was not added.           | Run `/plugin marketplace add dcadolph/slop-chop` first.    |
| Edits to a profile are ignored            | A stray `.slop-chop.json` is picked up.  | Point at the profile you mean with `--profile`, or remove the stray file. |

## Updating and removing

```
/plugin install slop-chop@slop-chop   # reinstall to pick up a new version
/plugin uninstall slop-chop           # remove the plugin
```

The binary updates on its own path, so `go install ...@latest` refreshes the tool the plugin
calls.
