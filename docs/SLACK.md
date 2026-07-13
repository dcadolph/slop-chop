# Slack

Chop slop without leaving Slack: a `/chop` command for any text, and a **Chop this message**
shortcut on messages already posted. Both run on the [hosted API](API.md), which runs the same
deterministic engine as everything else. Replies are ephemeral, so only you see the result,
and nothing is stored anywhere.

## Use

- `/chop In summary, we leverage a myriad of robust tools.` replies, visibly only to you,
  with the chopped text and the score movement (`slop 80 → 0`).
- Hover a message, **More actions**, **Chop this message**. The chopped version arrives as
  an ephemeral reply so you can compare, copy, or edit yours.

The command applies the same default as the web app: the cleaver preset on top of the
standard profile.

## Install into your workspace

The app is self-hosted on the same worker as the API, so setup is creating a Slack app that
points at it.

1. Go to [api.slack.com/apps](https://api.slack.com/apps), **Create New App**, **From a
   manifest**, pick your workspace, and paste the contents of
   [`worker/slack-app-manifest.json`](https://github.com/dcadolph/slop-chop/blob/main/worker/slack-app-manifest.json).
2. On the app's **Basic Information** page, copy the **Signing Secret**.
3. Give the worker the secret (your own terminal, from the repo's `worker/` folder):

   ```
   npx wrangler secret put SLACK_SIGNING_SECRET
   ```

   Paste the secret when prompted. Until this is set, the Slack endpoints answer 503 and do
   nothing.

4. Back in the Slack app config, **Install App** to your workspace.

## Privacy and verification

Every request is checked against Slack's signing secret (HMAC over the raw body, stale
timestamps rejected), so only your Slack app can reach the endpoints. The text of the
command or message is processed in memory by the engine and returned. It is not logged or
stored, and the reply goes only to Slack's response URL for that interaction.

## Endpoints

| Endpoint               | Purpose                                        |
| ---------------------- | ---------------------------------------------- |
| `POST /slack/command`  | The `/chop` slash command, replies inline.     |
| `POST /slack/interact` | The message shortcut, replies via response URL. |
