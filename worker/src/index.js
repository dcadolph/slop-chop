/* The slop-chop API: the rules engine as a Cloudflare Worker. POST text, get it chopped,
   with the same options as the npm package. The wasm engine boots once per isolate on the
   first request and is reused after that. Deterministic, no model, no storage: the text is
   processed in memory and the response is the only thing that leaves. */
"use strict";

// wasm_exec.js runs for its side effect: it defines Go on the global object.
import "../engine/wasm_exec.js";
// The CompiledWasm rule turns this import into a WebAssembly.Module.
import engineModule from "../engine/slop-chop.wasm";

import { verifySignature, handleCommand, handleInteract } from "./slack.js";

// maxTextBytes caps one request's text, so a giant paste cannot pin the isolate.
const maxTextBytes = 1024 * 1024;

// badgeCacheSeconds is how long a badge may be reused. GitHub serves README images
// through its own image proxy, which honors this, so the value decides how often the
// upstream file is fetched rather than how often a reader loads the page.
const badgeCacheSeconds = 6 * 60 * 60;

// badgeErrorCacheSeconds is the shorter reuse window for the gray badge, so a repo
// that was briefly unreachable is not stuck showing a placeholder for hours.
const badgeErrorCacheSeconds = 5 * 60;

// maxBadgeBytes caps the file the badge endpoint will score. A README past this is
// scored on the leading bytes rather than refused, since the density the score reports
// is a rate and a truncated read still answers the question.
const maxBadgeBytes = 256 * 1024;

// repoPattern is the owner/name shape the badge endpoint accepts. Anchored and limited
// to the characters GitHub allows, so the path cannot be steered anywhere else.
const repoPattern = /^[A-Za-z0-9][A-Za-z0-9-]{0,38}\/[A-Za-z0-9_.-]{1,100}$/;

// readmeNames are the README filenames tried in order, first hit winning. The raw host
// is case-sensitive and GitHub itself is not, so a repo whose file is readme.md is
// reachable on the web and a 404 on raw. The GitHub API resolves the real name in one
// call but allows sixty requests an hour from a shared egress address, which a hosted
// badge would exhaust, so the names are tried directly instead.
const readmeNames = ["README.md", "readme.md", "Readme.md", "README.markdown", "README.txt", "README"];

// corsHeaders lets browsers call the API from any page.
const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type",
};

let ready = null;

// boot instantiates the engine once per isolate. It runs inside a request context because
// the Go runtime needs timers, which Workers do not allow in global scope.
function boot() {
  if (ready) return ready;
  ready = (async () => {
    const go = new globalThis.Go();
    const instance = await WebAssembly.instantiate(engineModule, go.importObject);
    go.run(instance);
    // Give the Go runtime a tick to register its globals.
    await new Promise((r) => setTimeout(r, 0));
    return JSON.parse(globalThis.slopDefaults());
  })().catch((err) => {
    // A failed boot clears the cache, so the next request retries instead of the isolate
    // failing every request forever on a stale rejection.
    ready = null;
    throw err;
  });
  return ready;
}

// dedupe returns the array with duplicates dropped, order kept.
function dedupe(arr) {
  return [...new Set(arr)];
}

// voiceProfile folds a voice of keep, prefer, and avoid lists into the base profile, the
// same mapping every other surface uses: keep into allow, avoid into blockWords, prefer into
// word or phrase swaps with the voice winning.
function voiceProfile(base, voice) {
  if (!voice) return base;
  const wordReplace = { ...base.wordReplace };
  const phraseReplace = { ...base.phraseReplace };
  for (const [from, to] of Object.entries(voice.prefer || {})) {
    if (String(from).trim().split(/\s+/).length === 1) wordReplace[from] = to;
    else phraseReplace[from] = to;
  }
  return {
    ...base,
    wordReplace,
    phraseReplace,
    allow: dedupe([...(base.allow || []), ...(voice.keep || [])]),
    blockWords: dedupe([...(base.blockWords || []), ...(voice.avoid || [])]),
  };
}

// json wraps a body as a JSON response with CORS.
function json(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...corsHeaders },
  });
}

// svg wraps badge markup as an image response. A badge is embedded as an image, so a
// failure still answers 200 with the gray badge: a non-200 renders as a broken image
// in someone's README, which is a worse outcome than an honest "n/a".
function svg(markup, maxAge) {
  return new Response(markup, {
    status: 200,
    headers: {
      "Content-Type": "image/svg+xml; charset=utf-8",
      "Cache-Control": `public, max-age=${maxAge}`,
      ...corsHeaders,
    },
  });
}

// engineBadge renders a badge through the engine, with the same dead-engine handling
// engineChop uses.
function engineBadge(req) {
  try {
    return JSON.parse(globalThis.slopBadge(JSON.stringify(req)));
  } catch (err) {
    ready = null;
    return { error: "engine error: " + (err && err.message ? err.message : String(err)), died: true };
  }
}

// unknownBadge renders the gray placeholder, falling back to a bare literal if even
// that fails, so the endpoint always answers with an image.
function unknownBadge() {
  const res = engineBadge({ unknown: true });
  if (res && res.svg) return svg(res.svg, badgeErrorCacheSeconds);
  return svg(
    '<svg xmlns="http://www.w3.org/2000/svg" width="103" height="20" role="img"' +
      ' aria-label="slop score: unavailable"><rect width="103" height="20" rx="3"' +
      ' fill="#9e9e9e"/></svg>',
    badgeErrorCacheSeconds,
  );
}

// readmeText fetches a public repository's README from GitHub's raw host and returns
// its text, or null when there is nothing to score. HEAD resolves the default branch,
// so the caller does not have to guess between main and master, and the candidate
// names are tried in order because the raw host is case-sensitive.
async function readmeText(repo) {
  for (const name of readmeNames) {
    const res = await fetch(`https://raw.githubusercontent.com/${repo}/HEAD/${name}`, {
      headers: { "User-Agent": "slop-chop-badge" },
      redirect: "follow",
    });
    if (!res.ok) continue;
    const buf = await res.arrayBuffer();
    const text = new TextDecoder().decode(buf.byteLength > maxBadgeBytes ? buf.slice(0, maxBadgeBytes) : buf);
    if (text.trim()) return text;
  }
  return null;
}

// badge answers GET /badge?repo=owner/name with an SVG of that README's slop score.
// Only the repository is taken from the query: the label is fixed in the engine, so
// the endpoint cannot be used to render arbitrary text on this domain.
async function badge(url) {
  const repo = url.searchParams.get("repo");
  if (!repo || !repoPattern.test(repo)) return unknownBadge();
  let text;
  try {
    text = await readmeText(repo);
  } catch {
    return unknownBadge();
  }
  if (text === null) return unknownBadge();
  const res = engineBadge({ text, presets: ["cleaver"] });
  if (!res || res.error || !res.svg) return unknownBadge();
  return svg(res.svg, badgeCacheSeconds);
}

// engineChop runs one text through the engine. A throw means the Go runtime died, which a
// panic surfaces as "Go program has already exited", so the cached boot is dropped and the
// result is marked died so callers can answer 500 rather than 400.
function engineChop(text, profile, presets) {
  try {
    return JSON.parse(globalThis.slopChop(JSON.stringify({ text, profile, presets })));
  } catch (err) {
    ready = null;
    return { error: "engine error: " + (err && err.message ? err.message : String(err)), died: true };
  }
}

// chop runs the engine over one request body and returns the response. A body that is not
// a JSON object, or is missing text, is a 400 rather than an uncaught error, so a bare
// "null" or an array does not throw. If the engine itself throws, which a Go panic surfaces
// as "Go program has already exited", the cached instance is discarded so the next request
// boots a fresh engine instead of the isolate serving a dead one forever.
function chop(body, defaults) {
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return json({ error: "body must be a JSON object like {\"text\": \"...\"}" }, 400);
  }
  const text = body.text;
  if (typeof text !== "string" || !text.trim()) {
    return json({ error: "text is required" }, 400);
  }
  const base = body.profile && typeof body.profile === "object" ? body.profile : defaults;
  const res = engineChop(text, voiceProfile(base, body.voice), body.presets || ["cleaver"]);
  if (res.died) return json({ error: res.error }, 500);
  if (res.error) return json({ error: res.error }, 400);
  return json(res);
}

export default {
  // fetch routes the API: POST /chop does the work, GET /badge scores a public
  // README as an image, GET /presets lists the packs, and GET / describes the
  // endpoints. The whole body runs under one guard so any unexpected
  // throw still answers with CORS headers, and a throw that means the engine died drops the
  // cached instance so the next request re-boots rather than serving a poisoned isolate.
  async fetch(request, env) {
    try {
      return await route(request, env);
    } catch (err) {
      ready = null;
      return json({ error: "internal error: " + (err && err.message ? err.message : String(err)) }, 500);
    }
  },
};

// route resolves one request to a response. It is separated from fetch so the fetch guard
// can turn any thrown error into a CORS-bearing JSON response.
async function route(request, env) {
  const url = new URL(request.url);
  if (request.method === "OPTIONS") {
    return new Response(null, { status: 204, headers: corsHeaders });
  }

  if (url.pathname === "/" && request.method === "GET") {
    await boot();
    return json({
      name: "slop-chop",
      version: globalThis.slopVersion(),
      endpoints: {
        "POST /chop": "{text, presets?, voice?, profile?} -> {output, findings, score, scoreAfter}",
        "GET /presets": "built-in preset names",
        "GET /badge": "?repo=owner/name -> an SVG badge of that README's slop score",
        "POST /slack/command": "the /chop slash command, signature-verified",
        "POST /slack/interact": "the Chop this message shortcut, signature-verified",
      },
      docs: "https://slop-chop.com/API.html",
    });
  }

  if (url.pathname === "/presets" && request.method === "GET") {
    await boot();
    return json({ presets: JSON.parse(globalThis.slopPresets()) });
  }

  if (url.pathname === "/badge") {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return json({ error: "use GET" }, 405);
    }
    await boot();
    return badge(url);
  }

  if (url.pathname === "/chop") {
    if (request.method !== "POST") {
      return json({ error: "use POST" }, 405);
    }
    const raw = await request.arrayBuffer();
    if (raw.byteLength > maxTextBytes) {
      return json({ error: "text too large: the cap is 1MB" }, 413);
    }
    let body;
    try {
      body = JSON.parse(new TextDecoder().decode(raw));
    } catch {
      return json({ error: "body must be JSON like {\"text\": \"...\"}" }, 400);
    }
    const defaults = await boot();
    return chop(body, defaults);
  }

  if (url.pathname === "/slack/command" || url.pathname === "/slack/interact") {
    if (request.method !== "POST") {
      return json({ error: "use POST" }, 405);
    }
    const secret = env && env.SLACK_SIGNING_SECRET;
    if (!secret) {
      return json({ error: "slack is not configured" }, 503);
    }
    const rawBody = await request.text();
    if (!(await verifySignature(request, rawBody, secret))) {
      return json({ error: "bad signature" }, 401);
    }
    const defaults = await boot();
    const run = (text) => engineChop(text, defaults, ["cleaver"]);
    if (url.pathname === "/slack/command") {
      return handleCommand(rawBody, run);
    }
    return handleInteract(rawBody, run);
  }

  return json({ error: "not found" }, 404);
}
