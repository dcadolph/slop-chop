/* Hosts the slop-chop wasm engine in a hidden extension page. The service worker relays
   {target:"offscreen", type:"chop", text}; this runs the rules pass and answers with the
   cleaned text and the before and after slop scores. The wasm boot mirrors the site's
   worker, since the same binary backs both. */
"use strict";

let ready = null;
let defaults = null;

// boot instantiates the wasm module once and caches the built-in default profile.
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
  })();
  return ready;
}

// chop runs the engine with the built-in defaults and the cleaver preset, the same setup
// the web widget ships by default.
function chop(text) {
  const req = JSON.stringify({ text, profile: defaults, presets: ["cleaver"] });
  const res = JSON.parse(self.slopChop(req));
  if (res.error) return { error: res.error };
  return {
    ok: true,
    output: res.output,
    before: res.score ? res.score.value : null,
    after: res.scoreAfter ? res.scoreAfter.value : null,
  };
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.target !== "offscreen" || msg.type !== "chop") return undefined;
  boot()
    .then(() => sendResponse(chop(msg.text)))
    .catch((err) => sendResponse({ error: String((err && err.message) || err) }));
  return true;
});

boot();
