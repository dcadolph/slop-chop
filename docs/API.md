# Hosted API

!!! warning "Experimental"
    The hosted API is not reliable right now. It runs on a hosting plan that allows less
    compute per request than the engine needs to start, so many requests fail with an
    error. Everything else runs the engine on your own machine and is unaffected: the
    [web app](https://slop-chop.com), the command line tool, the editor integrations, the
    Obsidian plugin, the MCP server, and the `slop-chop-wasm` package for Node.

Chop text over HTTP. The API runs the same deterministic rules engine as everything else,
compiled to WebAssembly on Cloudflare Workers. No model, no account, no storage: the text is
processed in memory and the response is the only thing that leaves. Same input, same output,
every time.

Base URL: `https://api.slop-chop.com`

## POST /chop

```
curl -s https://api.slop-chop.com/chop \
  -H 'Content-Type: application/json' \
  -d '{"text": "In summary, we leverage a myriad of robust tools."}'
```

```json
{
  "output": "We use many solid tools.",
  "findings": [ { "rule": "phrase:in summary,", "match": "In summary, w", "offset": 0 } ],
  "score":      { "value": 80 },
  "scoreAfter": { "value": 0 }
}
```

The body takes the same options as the npm package:

| Field     | What it does                                                            |
|-----------|-------------------------------------------------------------------------|
| `text`    | The text to chop. Required, up to 1MB.                                   |
| `presets` | Built-in preset names to apply. Defaults to `["cleaver"]`.              |
| `voice`   | `{keep, prefer, avoid}` folded on top, your swaps winning.              |
| `profile` | A full profile that replaces the built-in default.                      |

With a voice:

```
curl -s https://api.slop-chop.com/chop \
  -H 'Content-Type: application/json' \
  -d '{"text": "we leverage robust tools",
       "voice": {"keep": ["robust"], "prefer": {"leverage": "wield"}}}'
```

```json
{ "output": "we wield robust tools" }
```

## GET /presets

Lists the built-in preset names.

## GET /badge

Scores a public repository's README and answers with an SVG badge, for embedding in that
README.

```markdown
[![slop score](https://api.slop-chop.com/badge?repo=dcadolph/slop-chop)](https://slop-chop.com)
```

| Field  | What it does                                                        |
|--------|---------------------------------------------------------------------|
| `repo` | The repository as `owner/name`. Required. Public repositories only. |

The README is read from GitHub's raw host, scored with the default profile, and discarded.
Only the first 16KB is scored, to keep the request inside the compute a hosted Worker is
allowed. Nothing is stored. The label is fixed, so the endpoint renders a score and nothing else.

A repository that cannot be read, has no README, or is private answers the gray `n/a` badge
with a 200, because a badge is loaded as an image and a non-200 renders as a broken image.
Scored badges are cacheable for six hours and the gray one for five minutes, which GitHub's
image proxy honors.

What the number measures is worth being plain about: READMEs score low. Across a sample of
widely used repositories the scores landed between 0 and 27 out of 100, because the score is
a density over prose and a README is mostly headings, lists, links, and fenced code, which
the engine skips. Treat the badge as a published commitment rather than a discriminating
test, and use `slop-chop check` in CI on the prose files where the measurement has room to
move.

## Notes

- CORS is open, so a browser page can call it directly.
- A body over 1MB answers 413. Malformed JSON answers 400.
- The optional model rewrite is not part of the API. It stays where your keys stay: the CLI
  and the web app. For private text, prefer those. They never send text anywhere at all.
