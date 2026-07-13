/* The slop-chop API: the rules engine as a Cloudflare Worker. POST text, get it chopped,
   with the same options as the npm package. The wasm engine boots once per isolate on the
   first request and is reused after that. Deterministic, no model, no storage: the text is
   processed in memory and the response is the only thing that leaves. */
"use strict";

// wasm_exec.js runs for its side effect: it defines Go on the global object.
import "../engine/wasm_exec.js";
// The CompiledWasm rule turns this import into a WebAssembly.Module.
import engineModule from "../engine/slop-chop.wasm";

// maxTextBytes caps one request's text, so a giant paste cannot pin the isolate.
const maxTextBytes = 1024 * 1024;

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

// chop runs the engine over one request body and returns the response.
function chop(body, defaults) {
  const text = body.text;
  if (typeof text !== "string" || !text.trim()) {
    return json({ error: "text is required" }, 400);
  }
  const base = body.profile || defaults;
  const req = JSON.stringify({
    text,
    profile: voiceProfile(base, body.voice),
    presets: body.presets || ["cleaver"],
  });
  const res = JSON.parse(globalThis.slopChop(req));
  if (res.error) return json({ error: res.error }, 400);
  return json(res);
}

export default {
  // fetch routes the API: POST /chop does the work, GET /presets lists the packs, and
  // GET / describes the endpoints.
  async fetch(request) {
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
        },
        docs: "https://slop-chop.com/API.html",
      });
    }

    if (url.pathname === "/presets" && request.method === "GET") {
      await boot();
      return json({ presets: JSON.parse(globalThis.slopPresets()) });
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

    return json({ error: "not found" }, 404);
  },
};
