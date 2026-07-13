# slop-chop hosted API

The rules engine as a Cloudflare Worker: `POST /chop` in, chopped text and scores out. See
[the API page](https://slop-chop.com/API.html) for the endpoint reference.

## Build and run locally

From the repo root:

```
make worker
cd worker && npx wrangler dev
```

Then:

```
curl -s -X POST http://127.0.0.1:8787/chop \
  -H 'Content-Type: application/json' \
  -d '{"text": "In summary, we leverage synergy."}'
```

## Deploy

```
npx wrangler login    # once
cd worker && npx wrangler deploy
```

`wrangler.jsonc` routes the worker to `api.slop-chop.com` as a custom domain; the zone lives
on Cloudflare, so the deploy provisions the DNS record and certificate on its own.

## Slack

The worker also serves a Slack app: `POST /slack/command` (the `/chop` slash command) and
`POST /slack/interact` (the Chop this message shortcut), both verified against the app's
signing secret and answering ephemerally. `slack-app-manifest.json` is the app definition to
paste at api.slack.com/apps. The endpoints answer 503 until the secret is set:

```
cd worker && npx wrangler secret put SLACK_SIGNING_SECRET
```

Setup steps are on [the Slack page](https://slop-chop.com/SLACK.html). Local test:
`npx wrangler dev --var SLACK_SIGNING_SECRET:test-secret-123` and sign requests with that
value.
