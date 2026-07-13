/* Slack endpoints for the hosted API: a /chop slash command and a "Chop this message"
   shortcut. Both verify Slack's request signature before touching the engine, reply
   ephemerally so only the invoker sees the result, and send nothing anywhere but back to
   Slack's response URL. The signing secret lives in a worker secret, never in code. */
"use strict";

// encoder turns strings into bytes for HMAC work.
const encoder = new TextEncoder();

// maxSkewSeconds bounds how old a signed request may be, which blocks replays.
const maxSkewSeconds = 300;

// hex renders an ArrayBuffer as lowercase hex.
function hex(buf) {
  return [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// constantTimeEqual compares two strings without leaking where they differ.
function constantTimeEqual(a, b) {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

// verifySignature checks Slack's v0 HMAC signature over the raw request body. A missing
// header, a stale timestamp, or a mismatched digest all fail closed.
export async function verifySignature(request, rawBody, secret) {
  const ts = request.headers.get("X-Slack-Request-Timestamp");
  const sig = request.headers.get("X-Slack-Signature");
  if (!ts || !sig || !secret) return false;
  if (Math.abs(Date.now() / 1000 - Number(ts)) > maxSkewSeconds) return false;
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const mac = await crypto.subtle.sign("HMAC", key, encoder.encode(`v0:${ts}:${rawBody}`));
  return constantTimeEqual(`v0=${hex(mac)}`, sig);
}

// escapeMrkdwn escapes the three characters Slack's mrkdwn treats as control characters.
function escapeMrkdwn(text) {
  return text.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}

// ephemeral builds a Slack message payload only the invoking user sees: the chopped text as
// the body and the score movement as a context line.
function ephemeral(output, before, after) {
  const moved = before === after ? `slop ${before}` : `slop ${before} → ${after}`;
  return {
    response_type: "ephemeral",
    text: output,
    blocks: [
      { type: "section", text: { type: "mrkdwn", text: escapeMrkdwn(output) } },
      { type: "context", elements: [{ type: "mrkdwn", text: `${moved} · slop-chop` }] },
    ],
  };
}

// notice builds a short ephemeral text-only reply for usage hints and errors.
function notice(text) {
  return { response_type: "ephemeral", text };
}

// runChop feeds text through the engine with the default cleaver preset and returns the
// Slack payload for it. The chop callback is the worker's engine entry point.
function runChop(chop, text) {
  if (!text || !text.trim()) return notice("Nothing to chop. Usage: /chop <text>");
  const res = chop(text);
  if (res.error) return notice("slop-chop: " + res.error);
  return ephemeral(res.output, res.score.value, res.scoreAfter.value);
}

// handleCommand answers the /chop slash command. Slack posts it form-encoded and expects
// the reply inline within three seconds, which the engine clears with ease.
export function handleCommand(rawBody, chop) {
  const form = new URLSearchParams(rawBody);
  return Response.json(runChop(chop, form.get("text") || ""));
}

// handleInteract answers the interactivity endpoint, which carries the message shortcut.
// Slack wants a bare 200 quickly and the real reply posted to the response URL, so the
// result is delivered there and the acknowledgment body stays empty. Payload shapes other
// than the chop shortcut are acknowledged and dropped.
export async function handleInteract(rawBody, chop) {
  let payload;
  try {
    payload = JSON.parse(new URLSearchParams(rawBody).get("payload") || "");
  } catch {
    return new Response(null, { status: 200 });
  }
  if (!payload || payload.type !== "message_action" || payload.callback_id !== "chop_message") {
    return new Response(null, { status: 200 });
  }
  const text = payload.message && payload.message.text ? payload.message.text : "";
  const reply = runChop(chop, text);
  if (payload.response_url) {
    await fetch(payload.response_url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(reply),
    });
  }
  return new Response(null, { status: 200 });
}
