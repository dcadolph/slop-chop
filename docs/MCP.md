# MCP server

Chop slop from inside any assistant that speaks the Model Context Protocol. `slop-chop mcp`
serves the engine over stdio, so Claude Desktop, Claude Code, Cursor, and the rest can clean a
draft on demand without you leaving the chat.

It runs the same deterministic rules engine as everything else, in a process on your machine.
Nothing is uploaded and no key is needed. The model rewrite stays off unless a call asks for
it.

## Run it

```
slop-chop mcp
```

It speaks the protocol on stdin and stdout, so you point a client at that command rather than
running it yourself. Install the binary first:

```sh
brew install dcadolph/tap/slop-chop
```

## The tools

| Tool      | What it does                                                                       |
|-----------|------------------------------------------------------------------------------------|
| `chop`    | Cleans the text and hands back the human version, with the score before and after. |
| `check`   | Reports the tells and where they are, changing nothing.                            |
| `presets` | Lists the built-in packs `chop` and `check` accept.                                |
| `drift`   | Says whether a draft sounds like your own writing, and names what does not.&nbsp;  |

`chop` and `check` both take `text`, plus an optional `presets` list and a `dialect` of
`american`, `british`, or `off`. `chop` also takes `model_rewrite`, off by default, covered
below. `drift` takes `text` and compares it against the fingerprint you measured with
`slop-chop voice fingerprint`, so an agent can check its draft against your own writing
before handing it over. Without a fingerprint it says so and measures nothing. See
[VOICE.md](VOICE.md).

Ask for a clean in whatever words you use. "Chop the slop out of this," "make this sound like a
person wrote it," and "strip the AI tells from my draft" all reach the same tool.

## Claude Code

```sh
claude mcp add slop-chop -- slop-chop mcp
```

Add `--preset cleaver` after `mcp` for the aggressive swaps. Check it took with
`claude mcp list`, which reports the server as connected.

## Claude Desktop

Open Settings, Developer, Edit Config, and add the server to
`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "slop-chop": {
      "command": "slop-chop",
      "args": ["mcp"]
    }
  }
}
```

Restart Claude Desktop. If it cannot find the binary, give the full path from
`which slop-chop` instead, since the app does not always inherit your shell's `PATH`.

## Cursor

Add the same block to `.cursor/mcp.json` in a project, or to `~/.cursor/mcp.json` for every
project:

```json
{
  "mcpServers": {
    "slop-chop": {
      "command": "slop-chop",
      "args": ["mcp", "--preset", "cleaver"]
    }
  }
}
```

## ChatGPT custom connectors

ChatGPT connectors reach a server over HTTP at a URL, and this one speaks stdio on your own
machine, so the two do not meet directly. Two ways around it:

- Put a stdio-to-HTTP bridge in front of it, such as `mcp-remote`, and give ChatGPT the URL
  the bridge serves. The text still gets chopped locally.
- Or skip MCP and call the [hosted API](API.md), which runs the same engine over
  `POST /chop`.

## Your own rules

The server reads the same configuration as every other command, so the tools already sound
like you:

- A `.slop-chop.json` in the directory the server starts in.
- Your `~/.slop-chop/voice.json`, written by `slop-chop voice init`. See [Your voice](VOICE.md).
- The flags you launch it with: `--profile`, `--preset`, `--dialect`, and `--voice`.

Flags set the defaults. A call naming its own `presets` or `dialect` overrides them for that
call alone.

## The model rewrite

`model_rewrite` is off by default, so the free deterministic clean is what you get unless you
ask for more. Set it to `true` and the rules run first, then a model reworks what rules cannot,
such as recasting a sentence so it no longer needs a semicolon. It needs a provider key and
makes a paid call.

```sh
export ANTHROPIC_API_KEY=sk-...
claude mcp add slop-chop -- slop-chop mcp
```

Point it at a local model instead and it costs nothing:

```sh
claude mcp add slop-chop -- slop-chop mcp \
  --provider openai --base-url http://localhost:11434/v1 --model llama3.1
```

Whatever the model returns is run through the rules again before you see it, so a rewrite
cannot put back the tells the rules just cut.

## Privacy

The rules pass is a function of the text and your profile. It runs in the server process on
your machine, reaches no network, and writes nothing to disk. The only text that leaves is what
you send to a model, and only when a call sets `model_rewrite`. See [Privacy](PRIVACY.md).
