/* Hosts the slop-chop wasm engine in a hidden extension page. The service worker relays
   messages tagged {target:"offscreen"}; this runs the rules pass and answers. The wasm boot
   mirrors the site's worker, since the same binary backs both. Settings (voice and presets)
   come from chrome.storage.local, so a chop reflects the latest options with no reload. */
"use strict";

let ready = null;
let defaults = null;

// boot instantiates the wasm module once and caches the built-in default profile. A failed
// boot clears the cached promise, so the next call retries instead of replaying a stale
// rejection forever.
function boot() {
  if (ready) return ready;
  ready = (async () => {
    const go = new Go();
    const url = chrome.runtime.getURL("engine/slop-chop.wasm");
    let result;
    try {
      result = await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
    } catch {
      const res = await fetch(url);
      result = await WebAssembly.instantiate(await res.arrayBuffer(), go.importObject);
    }
    go.run(result.instance);
    // Give the Go runtime a tick to register its globals.
    await new Promise((r) => setTimeout(r, 0));
    defaults = JSON.parse(self.slopDefaults());
  })().catch((err) => {
    ready = null;
    throw err;
  });
  return ready;
}

// dedupe returns the array with duplicates dropped, order kept.
function dedupe(arr) {
  return [...new Set(arr)];
}

// voiceProfile folds a voice into the default profile: keep into allow, avoid into
// blockWords, and each prefer entry into wordReplace when its key is one word or
// phraseReplace when it is several. Voice wins, and because the engine applies allow to every
// rule, a kept word survives even a preset that would swap it.
function voiceProfile(base, voice) {
  const wordReplace = { ...(base.wordReplace || {}) };
  const phraseReplace = { ...(base.phraseReplace || {}) };
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

// chop runs the engine with the voice folded in and the presets on top. Settings come from
// the service worker, since an offscreen document cannot read chrome.storage itself. Saved
// preset names are filtered against the engine's current packs, so a name removed by an
// update degrades gracefully instead of erroring every chop until the user re-saves.
function chop(text, settings) {
  const s = settings || {};
  const profile = voiceProfile(defaults, s.voice || {});
  const known = new Set(JSON.parse(self.slopPresets()));
  const presets = (s.presets || ["cleaver"]).filter((p) => known.has(p));
  const req = JSON.stringify({ text, profile, presets });
  const res = JSON.parse(self.slopChop(req));
  if (res.error) return { error: res.error };
  return {
    ok: true,
    output: res.output,
    before: res.score ? res.score.value : null,
    after: res.scoreAfter ? res.scoreAfter.value : null,
    findings: res.findings ? res.findings.length : 0,
  };
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.target !== "offscreen") return undefined;
  if (msg.type === "chop") {
    boot()
      .then(() => sendResponse(chop(msg.text, msg.settings)))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  if (msg.type === "presets") {
    boot()
      .then(() => sendResponse({ ok: true, presets: JSON.parse(self.slopPresets()) }))
      .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
    return true;
  }
  return undefined;
});

boot();
