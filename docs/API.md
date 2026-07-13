# Hosted API

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

## Notes

- CORS is open, so a browser page can call it directly.
- A body over 1MB answers 413; malformed JSON answers 400.
- The optional model rewrite is not part of the API. It stays where your keys stay: the CLI
  and the web app. For private text, prefer those; they never send text anywhere at all.
